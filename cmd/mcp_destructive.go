package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/cleaner"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/docker"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// envAllowDelete is the explicit human opt-in for destructive MCP tools. It is
// read from the server process environment (set in the MCP client config), so
// a model cannot grant it to itself through tool arguments.
const envAllowDelete = "MAC_TOOLKIT_MCP_ALLOW_DELETE"

// mcpDockerRunner and mcpAnalyze are seams so tests never touch a real Docker
// daemon or the user's real files.
var (
	mcpDockerRunner docker.Runner = docker.ExecRunner{}
	mcpAnalyze                    = runAnalysisMCP
)

// DockerCleanupInput is the input of mac_docker_cleanup.
type DockerCleanupInput struct {
	Keep        string `json:"keep" jsonschema:"required,comma-separated container names to preserve"`
	DryRun      *bool  `json:"dry_run,omitempty" jsonschema:"optional,default true — false executes, but only with the server opt-in and confirm_plan"`
	ConfirmPlan string `json:"confirm_plan,omitempty" jsonschema:"optional,required when dry_run=false: the plan_digest returned by a preview the user reviewed"`
}

// DockerCleanupOutput is the preview (plan + plan_digest) or the real result.
type DockerCleanupOutput struct {
	DryRun     bool                `json:"dry_run"`
	Plan       docker.CleanupPlan  `json:"plan"`
	PlanDigest string              `json:"plan_digest"`
	Results    []docker.ItemResult `json:"results,omitempty"`
	Succeeded  int                 `json:"succeeded,omitempty"`
	Failed     int                 `json:"failed,omitempty"`
	AuditPath  string              `json:"audit_path,omitempty"`
	AuditError string              `json:"audit_error,omitempty"`
}

// CleanBatchInput is the input of mac_clean_batch.
type CleanBatchInput struct {
	Domains     []string `json:"domains,omitempty" jsonschema:"optional,list of domains to clean (empty = all). Valid: browser, dev_caches, logs, trash, xcode, ollama, downloads, appsupport"`
	DryRun      *bool    `json:"dry_run,omitempty" jsonschema:"optional,default true — false executes, but only with the server opt-in and confirm_plan"`
	ConfirmPlan string   `json:"confirm_plan,omitempty" jsonschema:"optional,required when dry_run=false: the plan_digest returned by a preview the user reviewed"`
}

// CleanBatchOutput is the preview (items + plan_digest) or the real result.
type CleanBatchOutput struct {
	Previewed    []domainItem        `json:"previewed_items"`
	Deleted      []domainItem        `json:"deleted_items,omitempty"`
	Results      []core.DeleteResult `json:"results,omitempty"`
	FreedBytes   int64               `json:"freed_bytes"`
	FreedHuman   string              `json:"freed"`
	SkippedCount int                 `json:"skipped_unsafe_count"`
	DryRun       bool                `json:"dry_run"`
	PlanDigest   string              `json:"plan_digest"`
	AuditPath    string              `json:"audit_path,omitempty"`
	AuditError   string              `json:"audit_error,omitempty"`
}

// requireDeleteOptIn rejects real deletion unless the human enabled it in the
// server environment. The message names the terminal alternative.
func requireDeleteOptIn(terminalCmd string) error {
	if os.Getenv(envAllowDelete) == "1" {
		return nil
	}
	return fmt.Errorf("deletion over MCP is disabled: the MCP server was not started with %s=1. "+
		"This opt-in is set by the human in the MCP client config (env), never by the agent. "+
		"To delete, either enable it and retry with confirm_plan from a reviewed preview, "+
		"or run it yourself in a terminal: %s", envAllowDelete, terminalCmd)
}

// requireConfirmedPlan compares the caller's confirm_plan with the digest of
// the plan recomputed right now. Any difference (or absence) means the human
// did not review exactly this state, so nothing is deleted.
func requireConfirmedPlan(confirm, current string) error {
	if confirm == "" {
		return fmt.Errorf("confirm_plan is required when dry_run=false: run a preview (dry_run=true), have the user review it, and pass its plan_digest")
	}
	if confirm != current {
		return fmt.Errorf("confirm_plan does not match the current plan: the state changed since the preview. Nothing was deleted; run a new preview and ask the user to review it")
	}
	return nil
}

func handleDockerCleanup(_ context.Context, _ *mcp.CallToolRequest, input DockerCleanupInput) (*mcp.CallToolResult, DockerCleanupOutput, error) {
	keep := nonEmptyList(input.Keep)
	if len(keep) == 0 {
		return nil, DockerCleanupOutput{}, fmt.Errorf("keep is required (comma-separated container names)")
	}
	dryRun := isDryRun(input.DryRun)
	if !dryRun {
		// Checked before planning so a rejected call does no work at all.
		if err := requireDeleteOptIn("mac-toolkit docker cleanup --keep " + strings.Join(keep, ",") + " --execute"); err != nil {
			return nil, DockerCleanupOutput{}, err
		}
	}

	plan, err := docker.PlanWith(mcpDockerRunner, keep)
	if err != nil {
		return nil, DockerCleanupOutput{}, err
	}
	out := DockerCleanupOutput{DryRun: dryRun, Plan: plan, PlanDigest: docker.Digest(plan)}
	if dryRun {
		return nil, out, nil
	}
	if err := requireConfirmedPlan(input.ConfirmPlan, out.PlanDigest); err != nil {
		return nil, DockerCleanupOutput{}, err
	}

	out.Results = docker.Apply(plan, mcpDockerRunner)
	for _, r := range out.Results {
		if r.Result == docker.ResultSuccess {
			out.Succeeded++
		} else {
			out.Failed++
		}
	}
	out.AuditPath, out.AuditError = auditOutcome(docker.ToDeleteResults(out.Results))
	return nil, out, nil
}

// auditOutcome writes the "mcp" audit log and reports path/error as strings so
// a failed audit is visible to the caller instead of hiding a completed delete.
func auditOutcome(results []core.DeleteResult) (path, errMsg string) {
	dir, err := writeDeleteAudit(results, "mcp")
	if err != nil {
		return "", err.Error()
	}
	return dir, ""
}

// selectBatchItems analyzes the requested domains and splits safe_to_delete
// items (the only deletable ones) from the count of skipped unsafe items.
func selectBatchItems(domains []string) (safe []core.CleanableItem, skipped int) {
	var results []core.AnalysisResult
	if len(domains) == 0 {
		results = mcpAnalyze("")
	}
	for _, d := range domains {
		results = append(results, mcpAnalyze(strings.TrimSpace(d))...)
	}
	for _, r := range results {
		for _, item := range r.Items {
			if item.SafeToDelete {
				safe = append(safe, item)
			} else {
				skipped++
			}
		}
	}
	return safe, skipped
}

// batchDigest fingerprints the selected paths (order-independent).
func batchDigest(items []core.CleanableItem) string {
	paths := make([]string, len(items))
	for i, it := range items {
		paths[i] = it.Path
	}
	return core.DigestStrings(paths)
}

func handleCleanBatch(_ context.Context, _ *mcp.CallToolRequest, input CleanBatchInput) (*mcp.CallToolResult, CleanBatchOutput, error) {
	dryRun := isDryRun(input.DryRun)
	if !dryRun {
		if err := requireDeleteOptIn("mac-toolkit clean --execute"); err != nil {
			return nil, CleanBatchOutput{}, err
		}
	}

	safe, skipped := selectBatchItems(input.Domains)
	out := CleanBatchOutput{
		Previewed:    toDomainItems(safe),
		SkippedCount: skipped,
		DryRun:       dryRun,
		PlanDigest:   batchDigest(safe),
	}
	if dryRun {
		return nil, out, nil
	}
	if err := requireConfirmedPlan(input.ConfirmPlan, out.PlanDigest); err != nil {
		return nil, CleanBatchOutput{}, err
	}

	// cleaner.Delete applies the blacklist to every path.
	out.Results = cleaner.NewGenericCleaner(false, true).Clean(safe)
	for _, dr := range out.Results {
		if dr.Result == "success" {
			out.FreedBytes += dr.SizeBytes
			out.Deleted = append(out.Deleted, domainItem{
				Path: dr.Path, Size: core.FormatBytes(dr.SizeBytes), SizeBytes: dr.SizeBytes,
			})
		}
	}
	out.FreedHuman = core.FormatBytes(out.FreedBytes)
	out.AuditPath, out.AuditError = auditOutcome(out.Results)
	return nil, out, nil
}

func toDomainItems(items []core.CleanableItem) []domainItem {
	out := make([]domainItem, len(items))
	for i, item := range items {
		out[i] = domainItem{
			Path:         item.Path,
			Size:         core.FormatBytes(item.SizeBytes),
			SizeBytes:    item.SizeBytes,
			Label:        item.Label,
			SafeToDelete: true,
			Risk:         string(item.Risk),
			Reason:       item.Reason,
			AgeDays:      item.AgeDays,
		}
	}
	return out
}
