package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/docker"
)

// fakeDocker scripts read commands and records all calls; it never touches a
// real Docker daemon.
type fakeDocker struct {
	out   map[string]string
	fail  map[string]error
	calls []string
}

func (f *fakeDocker) Run(args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err := f.fail[key]; err != nil {
		return nil, err
	}
	return []byte(f.out[key]), nil
}

func (f *fakeDocker) destructive() []string {
	var d []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "rm ") || strings.HasPrefix(c, "rmi ") ||
			strings.HasPrefix(c, "volume rm ") || strings.HasPrefix(c, "builder ") {
			d = append(d, c)
		}
	}
	return d
}

func newFakeDocker() *fakeDocker {
	return &fakeDocker{
		out: map[string]string{
			"ps -a --format {{.Names}}": "app\ndb\n",
			"inspect db": `[{"Name":"/db","State":{"Status":"running"},"Config":{"Image":"postgres:15"},
"Mounts":[{"Type":"volume","Name":"dbdata"}]}]`,
			"images --format {{.Repository}}:{{.Tag}}\t{{.ID}}": "postgres:15\tid1\nnginx:latest\tid2\n",
			"volume ls -q": "dbdata\njunkvol\n",
			"system df --format {{.Type}}\t{{.Reclaimable}}": "Build Cache\t1GB (50%)\n",
		},
		fail: map[string]error{},
	}
}

// useTempAudit redirects audit logs to a temp dir for the test.
func useTempAudit(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "audit")
	orig := cmdAuditDir
	cmdAuditDir = dir
	t.Cleanup(func() { cmdAuditDir = orig })
	return dir
}

type auditFile struct {
	DryRun       bool                `json:"dry_run"`
	ApprovalMode string              `json:"approval_mode"`
	Deletions    []core.DeleteResult `json:"deletions"`
}

func readAudit(t *testing.T, dir string) auditFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatalf("audit log not written: %v", err)
	}
	var a auditFile
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatal(err)
	}
	return a
}

// ---- CLI gate ----

func TestDockerCleanupDryRunNeverDeletes(t *testing.T) {
	f := newFakeDocker()
	withStdin("si\n", func() { runDockerCleanup([]string{"db"}, false, f) })
	if d := f.destructive(); len(d) != 0 {
		t.Fatalf("dry-run issued %v", d)
	}
}

func TestDockerCleanupGateRejectsNonAffirmative(t *testing.T) {
	for _, in := range []string{"", "\n", "no\n", "maybe\n", "s i\n"} {
		f := newFakeDocker()
		dir := useTempAudit(t)
		withStdin(in, func() { runDockerCleanup([]string{"db"}, true, f) })
		if d := f.destructive(); len(d) != 0 {
			t.Fatalf("input %q must not delete, got %v", in, d)
		}
		if _, err := os.Stat(dir); err == nil {
			t.Fatalf("input %q: no audit expected when nothing ran", in)
		}
	}
}

func TestDockerCleanupAffirmativeExecutesAndAudits(t *testing.T) {
	f := newFakeDocker()
	f.fail["volume rm junkvol"] = errors.New("in use")
	dir := useTempAudit(t)
	var res []docker.ItemResult
	withStdin("si\n", func() { res = runDockerCleanup([]string{"db"}, true, f) })

	want := []string{"rm -f app", "rmi nginx:latest", "volume rm junkvol", "builder prune -a -f"}
	if strings.Join(f.destructive(), "|") != strings.Join(want, "|") {
		t.Fatalf("destructive = %v, want %v", f.destructive(), want)
	}
	if len(res) != 4 {
		t.Fatalf("results = %+v", res)
	}
	a := readAudit(t, dir)
	if a.ApprovalMode != "docker-cli" || a.DryRun {
		t.Fatalf("audit meta = %+v", a)
	}
	var sawFailure bool
	for _, d := range a.Deletions {
		if d.Path == "docker:volume:junkvol" && d.Result == "failure" && d.Error != "" {
			sawFailure = true
		}
	}
	if !sawFailure || len(a.Deletions) != 4 {
		t.Fatalf("audit must record the failure: %+v", a.Deletions)
	}
}

func TestDockerCleanupEmptyKeepNeverReachesRunner(t *testing.T) {
	f := newFakeDocker()
	withStdin("si\n", func() { runDockerCleanup(nil, true, f) })
	if len(f.calls) != 0 {
		t.Fatalf("empty keep must not call docker, got %v", f.calls)
	}
}

// ---- MCP docker ----

func withMCPDocker(t *testing.T, f *fakeDocker) {
	t.Helper()
	orig := mcpDockerRunner
	mcpDockerRunner = f
	t.Cleanup(func() { mcpDockerRunner = orig })
}

func ptr(b bool) *bool { return &b }

func TestMCPDockerCleanupPreviewReturnsDigestAndNeverDeletes(t *testing.T) {
	f := newFakeDocker()
	withMCPDocker(t, f)
	_, out, err := handleDockerCleanup(context.Background(), nil, DockerCleanupInput{Keep: "db"})
	if err != nil || !out.DryRun || len(out.PlanDigest) != 64 {
		t.Fatalf("preview: %+v err=%v", out, err)
	}
	if d := f.destructive(); len(d) != 0 {
		t.Fatalf("preview deleted: %v", d)
	}
}

func TestMCPDockerCleanupRejectsWithoutOptIn(t *testing.T) {
	t.Setenv(envAllowDelete, "")
	f := newFakeDocker()
	withMCPDocker(t, f)
	_, _, err := handleDockerCleanup(context.Background(), nil,
		DockerCleanupInput{Keep: "db", DryRun: ptr(false), ConfirmPlan: "anything"})
	if err == nil || !strings.Contains(err.Error(), envAllowDelete) ||
		!strings.Contains(err.Error(), "mac-toolkit docker cleanup --keep db --execute") {
		t.Fatalf("expected opt-in error naming env and terminal command, got %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("rejected call must do no docker work: %v", f.calls)
	}
}

func TestMCPDockerCleanupRejectsMissingOrWrongDigest(t *testing.T) {
	t.Setenv(envAllowDelete, "1")
	for _, confirm := range []string{"", "deadbeef"} {
		f := newFakeDocker()
		withMCPDocker(t, f)
		_, _, err := handleDockerCleanup(context.Background(), nil,
			DockerCleanupInput{Keep: "db", DryRun: ptr(false), ConfirmPlan: confirm})
		if err == nil {
			t.Fatalf("confirm_plan %q must be rejected", confirm)
		}
		if d := f.destructive(); len(d) != 0 {
			t.Fatalf("rejected call deleted %v", d)
		}
	}
}

func TestMCPDockerCleanupExecutesWithOptInAndDigestAndAudits(t *testing.T) {
	t.Setenv(envAllowDelete, "1")
	dir := useTempAudit(t)
	f := newFakeDocker()
	withMCPDocker(t, f)
	_, prev, err := handleDockerCleanup(context.Background(), nil, DockerCleanupInput{Keep: "db"})
	if err != nil {
		t.Fatal(err)
	}
	f.fail["rmi nginx:latest"] = errors.New("conflict")
	_, out, err := handleDockerCleanup(context.Background(), nil,
		DockerCleanupInput{Keep: "db", DryRun: ptr(false), ConfirmPlan: prev.PlanDigest})
	if err != nil {
		t.Fatal(err)
	}
	if out.Succeeded != 3 || out.Failed != 1 || len(out.Results) != 4 {
		t.Fatalf("out = %+v", out)
	}
	if a := readAudit(t, dir); a.ApprovalMode != "mcp" || len(a.Deletions) != 4 {
		t.Fatalf("audit = %+v", a)
	}
}

func TestMCPDockerCleanupRejectsStaleDigest(t *testing.T) {
	t.Setenv(envAllowDelete, "1")
	f := newFakeDocker()
	withMCPDocker(t, f)
	_, prev, _ := handleDockerCleanup(context.Background(), nil, DockerCleanupInput{Keep: "db"})
	f.out["volume ls -q"] = "dbdata\njunkvol\nnewvol\n" // state changed after the preview
	_, _, err := handleDockerCleanup(context.Background(), nil,
		DockerCleanupInput{Keep: "db", DryRun: ptr(false), ConfirmPlan: prev.PlanDigest})
	if err == nil {
		t.Fatal("changed state must invalidate the digest")
	}
	if d := f.destructive(); len(d) != 0 {
		t.Fatalf("stale plan deleted %v", d)
	}
}

// ---- MCP batch ----

func withBatchItems(t *testing.T, paths ...string) {
	t.Helper()
	orig := mcpAnalyze
	mcpAnalyze = func(string) []core.AnalysisResult {
		var items []core.CleanableItem
		for _, p := range paths {
			items = append(items, core.CleanableItem{Path: p, SizeBytes: 3, SafeToDelete: true})
		}
		items = append(items, core.CleanableItem{Path: "/unsafe/x", SafeToDelete: false})
		return []core.AnalysisResult{{Domain: "fake", Items: items}}
	}
	t.Cleanup(func() { mcpAnalyze = orig })
}

func makeFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMCPBatchGate(t *testing.T) {
	tmp := t.TempDir()
	f1, f2 := makeFile(t, tmp, "a"), makeFile(t, tmp, "b")
	withBatchItems(t, f1, f2)
	ctx := context.Background()

	t.Setenv(envAllowDelete, "")
	if _, _, err := handleCleanBatch(ctx, nil, CleanBatchInput{DryRun: ptr(false), ConfirmPlan: "x"}); err == nil ||
		!strings.Contains(err.Error(), "mac-toolkit clean --execute") {
		t.Fatalf("no opt-in must reject with terminal command, got %v", err)
	}

	t.Setenv(envAllowDelete, "1")
	_, prev, err := handleCleanBatch(ctx, nil, CleanBatchInput{})
	if err != nil || !prev.DryRun || prev.PlanDigest != core.DigestStrings([]string{f1, f2}) || prev.SkippedCount != 1 {
		t.Fatalf("preview = %+v err=%v", prev, err)
	}
	for _, c := range []string{"", "wrong"} {
		if _, _, err := handleCleanBatch(ctx, nil, CleanBatchInput{DryRun: ptr(false), ConfirmPlan: c}); err == nil {
			t.Fatalf("confirm_plan %q must be rejected", c)
		}
	}
	for _, p := range []string{f1, f2} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("rejected calls must not delete %s", p)
		}
	}
}

func TestMCPBatchExecutesAndWritesAudit(t *testing.T) {
	t.Setenv(envAllowDelete, "1")
	dir := useTempAudit(t)
	tmp := t.TempDir()
	f1 := makeFile(t, tmp, "a")
	withBatchItems(t, f1)
	_, prev, _ := handleCleanBatch(context.Background(), nil, CleanBatchInput{})
	_, out, err := handleCleanBatch(context.Background(), nil,
		CleanBatchInput{DryRun: ptr(false), ConfirmPlan: prev.PlanDigest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Fatal("safe item should be deleted")
	}
	if len(out.Deleted) != 1 || out.FreedBytes != 3 || out.AuditPath == "" {
		t.Fatalf("out = %+v", out)
	}
	if a := readAudit(t, dir); a.ApprovalMode != "mcp" || len(a.Deletions) != 1 || a.Deletions[0].Result != "success" {
		t.Fatalf("audit = %+v", a)
	}
}
