package docker

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// fakeRunner scripts read commands by their joined args and records every
// call, so tests can assert exactly which commands were (not) issued.
type fakeRunner struct {
	out   map[string]string
	fail  map[string]error
	calls []string
}

func (f *fakeRunner) Run(args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err := f.fail[key]; err != nil {
		return nil, err
	}
	return []byte(f.out[key]), nil
}

// destructive lists recorded calls that would modify Docker state.
func (f *fakeRunner) destructive() []string {
	var d []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "rm ") || strings.HasPrefix(c, "rmi ") ||
			strings.HasPrefix(c, "volume rm ") || strings.HasPrefix(c, "builder ") {
			d = append(d, c)
		}
	}
	return d
}

const inspectDB = `[{"Name":"/db","State":{"Status":"running"},
"Config":{"Image":"postgres:15","Env":[]},
"Mounts":[{"Type":"volume","Name":"dbdata"},{"Type":"bind","Name":""}]}]`

func newFake() *fakeRunner {
	return &fakeRunner{
		out: map[string]string{
			"ps -a --format {{.Names}}": "app\ndb\njunk\n",
			"inspect db":                inspectDB,
			"images --format {{.Repository}}:{{.Tag}}\t{{.ID}}": "postgres:15\tid1\nnginx:latest\tid2\n<none>:<none>\tid3\n",
			"volume ls -q": "dbdata\njunkvol\n",
			"system df --format {{.Type}}\t{{.Reclaimable}}": "Images\t1GB (50%)\nBuild Cache\t1.5GB (80%)\n",
		},
		fail: map[string]error{},
	}
}

func names(p CleanupPlan) []string {
	var n []string
	for _, it := range p.Remove {
		n = append(n, it.Kind+":"+it.Name)
	}
	return n
}

func TestPlanKeepsContainerImageAndVolumes(t *testing.T) {
	f := newFake()
	plan, err := PlanWith(f, []string{"db"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"container:app", "container:junk", "image:nginx:latest", "image:id3", "volume:junkvol", "build-cache:build-cache"}
	if !reflect.DeepEqual(names(plan), want) {
		t.Fatalf("plan = %v, want %v", names(plan), want)
	}
	if plan.Kept.Images[0] != "postgres:15" || plan.Kept.Volumes[0] != "dbdata" {
		t.Fatalf("kept = %+v", plan.Kept)
	}
	if plan.BuildCacheReclaimable != "1.5GB (80%)" {
		t.Fatalf("reclaimable = %q", plan.BuildCacheReclaimable)
	}
}

func TestPlanIsReadOnly(t *testing.T) {
	f := newFake()
	if _, err := PlanWith(f, []string{"db"}); err != nil {
		t.Fatal(err)
	}
	if d := f.destructive(); len(d) != 0 {
		t.Fatalf("Plan issued destructive commands: %v", d)
	}
}

func TestPlanRejectsEmptyKeep(t *testing.T) {
	for _, keep := range [][]string{nil, {}, {"", "  "}} {
		f := newFake()
		if _, err := PlanWith(f, keep); err == nil {
			t.Fatalf("keep %q should be rejected", keep)
		}
		if len(f.calls) != 0 {
			t.Fatalf("no docker call expected on empty keep, got %v", f.calls)
		}
	}
}

func TestPlanRejectsUnknownKeepContainer(t *testing.T) {
	if _, err := PlanWith(newFake(), []string{"typo"}); err == nil {
		t.Fatal("unknown keep container must abort the plan")
	}
}

func TestPlanPropagatesReadErrors(t *testing.T) {
	f := newFake()
	f.fail["volume ls -q"] = errors.New("daemon down")
	if _, err := PlanWith(f, []string{"db"}); err == nil {
		t.Fatal("read error must abort the plan, not yield a partial plan")
	}
}

func TestApplyRunsOnlyPlanItemsAndPropagatesErrors(t *testing.T) {
	f := newFake()
	plan := CleanupPlan{
		Kept: Kept{Containers: []string{"db"}},
		Remove: []PlanItem{
			{Kind: KindContainer, Name: "app"},
			{Kind: KindVolume, Name: "junkvol"},
			{Kind: KindImage, Name: "nginx:latest"},
		},
	}
	f.fail["volume rm junkvol"] = errors.New("volume in use")
	res := Apply(plan, f)

	want := []string{"rm -f app", "volume rm junkvol", "rmi nginx:latest"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %v, want exactly %v", f.calls, want)
	}
	if res[0].Result != ResultSuccess || res[2].Result != ResultSuccess {
		t.Fatalf("results = %+v", res)
	}
	if res[1].Result != ResultFailure || !strings.Contains(res[1].Error, "volume in use") {
		t.Fatalf("failure not propagated: %+v", res[1])
	}
}

func TestApplySkipsUnsafeItems(t *testing.T) {
	f := newFake()
	plan := CleanupPlan{
		Kept: Kept{Containers: []string{"db"}},
		Remove: []PlanItem{
			{Kind: KindContainer, Name: "db"}, // protected by the plan itself
			{Kind: KindVolume, Name: "-f"},    // would parse as a flag
			{Kind: "network", Name: "bridge"}, // unknown kind
			{Kind: KindImage, Name: ""},       // empty
		},
	}
	res := Apply(plan, f)
	if len(f.calls) != 0 {
		t.Fatalf("no command may run, got %v", f.calls)
	}
	for _, r := range res {
		if r.Result != ResultSkipped || r.Error == "" {
			t.Fatalf("expected skipped with reason, got %+v", r)
		}
	}
}

func TestDigestDeterministicAndOrderIndependent(t *testing.T) {
	a := CleanupPlan{Remove: []PlanItem{{Kind: KindContainer, Name: "a"}, {Kind: KindVolume, Name: "v"}}}
	b := CleanupPlan{Remove: []PlanItem{{Kind: KindVolume, Name: "v"}, {Kind: KindContainer, Name: "a"}}}
	if Digest(a) != Digest(a) || Digest(a) != Digest(b) {
		t.Fatal("digest must be stable and order independent")
	}
	c := CleanupPlan{Remove: []PlanItem{{Kind: KindContainer, Name: "a"}}}
	if Digest(a) == Digest(c) {
		t.Fatal("different plans must have different digests")
	}
	if len(Digest(a)) != 64 {
		t.Fatalf("expected sha256 hex, got %q", Digest(a))
	}
}

func TestToDeleteResultsPaths(t *testing.T) {
	got := ToDeleteResults([]ItemResult{
		{Kind: KindContainer, Name: "c", Result: ResultSuccess},
		{Kind: KindImage, Name: "i", Result: ResultFailure, Error: "x"},
		{Kind: KindVolume, Name: "v", Result: ResultSkipped},
		{Kind: KindBuildCache, Name: KindBuildCache, Result: ResultSuccess},
	})
	want := []string{"docker:container:c", "docker:image:i", "docker:volume:v", "docker:build-cache"}
	for i, w := range want {
		if got[i].Path != w {
			t.Fatalf("path[%d] = %q, want %q", i, got[i].Path, w)
		}
	}
	if got[1].Error != "x" {
		t.Fatal("error must carry over to the audit result")
	}
}
