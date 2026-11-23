package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/docker"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"
)

// cmdAuditDir overrides where audit logs are written; empty means the default
// reports/cleanup_<timestamp>. Tests point it at t.TempDir().
var cmdAuditDir string

// runDockerCleanup is the whole `docker cleanup` flow with an injectable
// Runner. Without execute it only prints the plan. With execute it still
// refuses to touch Docker until the human types an affirmative.
func runDockerCleanup(keep []string, execute bool, r docker.Runner) []docker.ItemResult {
	plan, err := docker.PlanWith(r, keep)
	if err != nil {
		core.Error("Cleanup failed: %v", err)
		return nil
	}
	printDockerPlan(plan)

	if !execute {
		fmt.Println()
		core.Info("Dry-run — nothing was deleted. Re-run with --execute to apply.")
		return nil
	}
	if !confirmDockerCleanup(plan) {
		core.Info("Aborted — nothing was deleted.")
		return nil
	}

	results := docker.Apply(plan, r)
	if _, err := writeDeleteAudit(docker.ToDeleteResults(results), "docker-cli"); err != nil {
		core.Error("Audit log: %v", err)
	}
	printDockerSummary(results)
	return results
}

// printDockerPlan lists everything that will be removed and kept. Volumes get
// their own highlighted line because they can hold database data.
func printDockerPlan(plan docker.CleanupPlan) {
	fmt.Println()
	fmt.Printf("  Kept containers:    %s\n", strings.Join(plan.Kept.Containers, ", "))
	fmt.Printf("  Kept images:        %s\n", strings.Join(plan.Kept.Images, ", "))
	fmt.Printf("  Kept volumes:       %s\n", strings.Join(plan.Kept.Volumes, ", "))
	fmt.Println()
	printPlanKind("Containers to remove", plan.Names(docker.KindContainer))
	printPlanKind("Images to remove", plan.Names(docker.KindImage))
	vols := plan.Names(docker.KindVolume)
	printPlanKind("Volumes to remove", vols)
	if len(vols) > 0 {
		core.Warn("Volúmenes: pueden contener datos (bases de datos)")
	}
	fmt.Printf("  Build cache:        %s (builder prune -a)\n", plan.BuildCacheReclaimable)
}

func printPlanKind(label string, names []string) {
	fmt.Printf("  %-20s %d\n", label+":", len(names))
	for _, n := range names {
		fmt.Printf("    - %s\n", n)
	}
}

// confirmDockerCleanup is the human gate: it states the exact counts and only
// returns true for a typed affirmative. Empty input, EOF or anything else
// means "do not delete". It reads finalConfirmReader (shared with clean) and
// core.IsAffirmative so every destructive path agrees on what "yes" is.
func confirmDockerCleanup(plan docker.CleanupPlan) bool {
	core.Warn("Esto eliminará %d contenedores, %d imágenes, %d volúmenes y el build cache. Irreversible.",
		len(plan.Names(docker.KindContainer)), len(plan.Names(docker.KindImage)),
		len(plan.Names(docker.KindVolume)))
	fmt.Fprint(os.Stderr, `Escribe "si" para confirmar: `)

	line, err := bufio.NewReader(finalConfirmReader).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	return core.IsAffirmative(line)
}

// writeDeleteAudit writes the audit log for a real (non dry-run) deletion and
// returns the directory it was written to.
func writeDeleteAudit(results []core.DeleteResult, mode string) (string, error) {
	audit := &reporter.AuditReporter{
		OutputDir:    cmdAuditDir,
		DryRun:       false,
		ApprovalMode: core.ApprovalMode(mode),
	}
	err := audit.WriteAudit(results)
	return audit.OutputDir, err
}

// printDockerSummary shows successes and, explicitly, every failure.
func printDockerSummary(results []docker.ItemResult) {
	var ok, failed, skipped int
	for _, r := range results {
		switch r.Result {
		case docker.ResultSuccess:
			ok++
		case docker.ResultFailure:
			failed++
			core.Error("%s %s: %s", r.Kind, r.Name, r.Error)
		default:
			skipped++
			core.Warn("skipped %s %s: %s", r.Kind, r.Name, r.Error)
		}
	}
	fmt.Println()
	if failed == 0 && skipped == 0 {
		core.Success("Cleanup complete: %d items removed", ok)
		return
	}
	core.Warn("Cleanup finished: %d succeeded, %d failed, %d skipped", ok, failed, skipped)
}
