package docker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// Kinds of resources a cleanup can touch.
const (
	KindContainer  = "container"
	KindImage      = "image"
	KindVolume     = "volume"
	KindBuildCache = "build-cache"
)

// Result values of an applied item.
const (
	ResultSuccess = "success"
	ResultFailure = "failure"
	ResultSkipped = "skipped"
)

// PlanItem is one resource scheduled for removal.
type PlanItem struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// Kept lists what the plan preserves: the --keep containers plus the images
// and volumes they depend on.
type Kept struct {
	Containers []string `json:"containers"`
	Images     []string `json:"images"`
	Volumes    []string `json:"volumes"`
}

// CleanupPlan is the read-only description of a cleanup. Apply executes
// exactly what is in Remove and nothing else.
type CleanupPlan struct {
	Remove                []PlanItem `json:"remove"`
	Kept                  Kept       `json:"kept"`
	BuildCacheReclaimable string     `json:"build_cache_reclaimable"`
}

// ItemResult is the outcome of applying one PlanItem.
type ItemResult struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// Names returns the plan's item names of one kind, in plan order.
func (p CleanupPlan) Names(kind string) []string {
	var out []string
	for _, it := range p.Remove {
		if it.Kind == kind {
			out = append(out, it.Name)
		}
	}
	return out
}

// Digest fingerprints a plan: SHA-256 (hex) over its sorted `kind:name`
// identifiers. Order-independent and stable, so a preview and a later
// execution can be proven to target the same resources.
func Digest(plan CleanupPlan) string {
	ids := make([]string, len(plan.Remove))
	for i, it := range plan.Remove {
		ids[i] = it.Kind + ":" + it.Name
	}
	return core.DigestStrings(ids)
}

// Plan builds the cleanup plan using the real docker CLI. Read-only.
func Plan(keep []string) (CleanupPlan, error) {
	return PlanWith(ExecRunner{}, keep)
}

// PlanWith is Plan with an injectable Runner. It only issues read commands
// (ps, inspect, images, volume ls, system df); nothing is ever removed here.
func PlanWith(r Runner, keep []string) (CleanupPlan, error) {
	keep = nonEmpty(keep)
	if len(keep) == 0 {
		return CleanupPlan{}, errors.New("keep list is empty: refusing to plan a cleanup that would remove every container; pass at least one container name to preserve")
	}

	out, err := r.Run("ps", "-a", "--format", "{{.Names}}")
	if err != nil {
		return CleanupPlan{}, fmt.Errorf("docker ps failed: %w", err)
	}
	all := nonEmpty(strings.Split(string(out), "\n"))

	kept, err := resolveKept(r, keep, all)
	if err != nil {
		return CleanupPlan{}, err
	}

	plan := CleanupPlan{Kept: kept}
	keepSet := toSet(keep)
	for _, c := range all {
		if !keepSet[c] {
			plan.Remove = append(plan.Remove, PlanItem{Kind: KindContainer, Name: c})
		}
	}
	if err := planImages(r, &plan); err != nil {
		return CleanupPlan{}, err
	}
	if err := planVolumes(r, &plan); err != nil {
		return CleanupPlan{}, err
	}
	planBuildCache(r, &plan)
	return plan, nil
}

// resolveKept inspects every --keep container to learn which images and
// volumes must survive. A keep name that does not exist or cannot be inspected
// aborts the plan: silently ignoring it (e.g. a typo) would leave its volumes
// unprotected and schedule them for deletion.
func resolveKept(r Runner, keep, existing []string) (Kept, error) {
	exists := toSet(existing)
	kept := Kept{Containers: keep}
	seenImg, seenVol := map[string]bool{}, map[string]bool{}
	for _, name := range keep {
		if !exists[name] {
			return Kept{}, fmt.Errorf("keep container %q not found; refusing to plan (its volumes would not be protected)", name)
		}
		info, err := inspectWith(r, name)
		if err != nil {
			return Kept{}, fmt.Errorf("cannot inspect keep container %q: %w", name, err)
		}
		if !seenImg[info.Image] {
			seenImg[info.Image] = true
			kept.Images = append(kept.Images, info.Image)
		}
		for _, v := range info.Volumes {
			if !seenVol[v] {
				seenVol[v] = true
				kept.Volumes = append(kept.Volumes, v)
			}
		}
	}
	return kept, nil
}

// planImages adds every image not used by a kept container. Untagged images
// are addressed by ID because `<none>:<none>` is not a valid rmi target.
func planImages(r Runner, plan *CleanupPlan) error {
	out, err := r.Run("images", "--format", "{{.Repository}}:{{.Tag}}\t{{.ID}}")
	if err != nil {
		return fmt.Errorf("docker images failed: %w", err)
	}
	keepImg := toSet(plan.Kept.Images)
	seen := map[string]bool{}
	for _, line := range nonEmpty(strings.Split(string(out), "\n")) {
		ref, id, _ := strings.Cut(line, "\t")
		ref, id = strings.TrimSpace(ref), strings.TrimSpace(id)
		if keepImg[ref] || keepImg[strings.Split(ref, ":")[0]] {
			continue
		}
		name := ref
		if strings.Contains(ref, "<none>") {
			name = id
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		plan.Remove = append(plan.Remove, PlanItem{Kind: KindImage, Name: name})
	}
	return nil
}

// planVolumes adds every volume not mounted by a kept container.
func planVolumes(r Runner, plan *CleanupPlan) error {
	out, err := r.Run("volume", "ls", "-q")
	if err != nil {
		return fmt.Errorf("docker volume ls failed: %w", err)
	}
	keepVol := toSet(plan.Kept.Volumes)
	for _, v := range nonEmpty(strings.Split(string(out), "\n")) {
		if !keepVol[v] {
			plan.Remove = append(plan.Remove, PlanItem{Kind: KindVolume, Name: v})
		}
	}
	return nil
}

// planBuildCache always schedules the build-cache prune (it is part of what
// Apply runs) and records the reclaimable estimate for the preview. A failed
// estimate is informational only, so it never blocks planning.
func planBuildCache(r Runner, plan *CleanupPlan) {
	plan.BuildCacheReclaimable = "unknown"
	out, err := r.Run("system", "df", "--format", "{{.Type}}\t{{.Reclaimable}}")
	var size int64
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if rest, ok := strings.CutPrefix(line, "Build Cache\t"); ok {
				plan.BuildCacheReclaimable = strings.TrimSpace(rest)
				size = parseDockerSize(rest)
			}
		}
	}
	plan.Remove = append(plan.Remove, PlanItem{Kind: KindBuildCache, Name: KindBuildCache, SizeBytes: size})
}

// Apply executes exactly the items in plan through r and reports one result
// per item. Failures are returned, never swallowed, and one failure does not
// stop the remaining items.
func Apply(plan CleanupPlan, r Runner) []ItemResult {
	protected := toSet(plan.Kept.Containers)
	results := make([]ItemResult, 0, len(plan.Remove))
	for _, it := range plan.Remove {
		res := ItemResult{Kind: it.Kind, Name: it.Name, SizeBytes: it.SizeBytes}
		args, why := commandFor(it, protected)
		switch {
		case args == nil:
			res.Result, res.Error = ResultSkipped, why
		default:
			if _, err := r.Run(args...); err != nil {
				res.Result, res.Error = ResultFailure, err.Error()
			} else {
				res.Result = ResultSuccess
			}
		}
		results = append(results, res)
	}
	return results
}

// commandFor maps a plan item to its docker command. It returns nil args plus
// a reason when the item must not be executed: an unknown kind, a name that
// could be parsed as a flag, or a container the plan itself says to keep.
// These are defense-in-depth checks for a hand-built or tampered plan.
func commandFor(it PlanItem, protected map[string]bool) ([]string, string) {
	if it.Name == "" || strings.HasPrefix(it.Name, "-") {
		return nil, "invalid name"
	}
	switch it.Kind {
	case KindContainer:
		if protected[it.Name] {
			return nil, "container is in the keep list"
		}
		return []string{"rm", "-f", it.Name}, ""
	case KindImage:
		return []string{"rmi", it.Name}, ""
	case KindVolume:
		return []string{"volume", "rm", it.Name}, ""
	case KindBuildCache:
		return []string{"builder", "prune", "-a", "-f"}, ""
	default:
		return nil, "unknown kind " + it.Kind
	}
}

// AuditPath is the audit identifier of an item: docker:<kind>:<name>, or
// docker:build-cache for the cache.
func AuditPath(kind, name string) string {
	if kind == KindBuildCache {
		return "docker:build-cache"
	}
	return "docker:" + kind + ":" + name
}

// ToDeleteResults adapts item results to the shared audit format.
func ToDeleteResults(items []ItemResult) []core.DeleteResult {
	out := make([]core.DeleteResult, len(items))
	for i, it := range items {
		out[i] = core.DeleteResult{
			Path:      AuditPath(it.Kind, it.Name),
			Result:    it.Result,
			SizeBytes: it.SizeBytes,
			Error:     it.Error,
		}
	}
	return out
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
