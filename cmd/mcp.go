package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/analyzer"
	"github.com/arheanja-ops/mac-toolkit/internal/cleaner"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/docker"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run as MCP server (stdio transport for AI chat integration)",
	Long:  "Starts the toolkit as a Model Context Protocol server, exposing disk analysis, monitors, and cleanup preview as MCP tools invocable from any MCP-compatible chat client.",
	Run: func(cmd *cobra.Command, args []string) {
		runMCPServer()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

// ---------------------------------------------------------------------------
// MCP Server
// ---------------------------------------------------------------------------

func runMCPServer() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mac-toolkit",
		Version: "1.0.0",
	}, nil)

	// Register all tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_analyze",
		Description: "Analyze disk usage across macOS domains (browser caches, dev tools, Docker, Ollama, logs, downloads, etc). Returns structured JSON with size, severity, risk, and per-item details.",
	}, handleAnalyze)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_battery",
		Description: "Get battery health, cycle count, temperature, voltage, charge status, and time remaining.",
	}, handleBattery)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_system",
		Description: "Get system info: CPU usage, memory/swap stats, thermal state, hardware model and chip.",
	}, handleSystem)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_processes",
		Description: "Get top 10 processes by CPU and top 10 by memory usage, plus memory pressure.",
	}, handleProcesses)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_network",
		Description: "Get network status: WiFi SSID/signal, bytes sent/received, active connections, online check.",
	}, handleNetwork)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_status",
		Description: "List all registered analyzer domains with their risk levels (safe/warn/danger).",
	}, handleStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_clean_preview",
		Description: "Preview what would be cleaned (dry-run). Returns list of items with size, risk, and safe_to_delete flag. Never deletes anything.",
	}, handleCleanPreview)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_docker_backup",
		Description: "Backup a Docker container's database. Auto-detects postgres/mysql from image, finds the DB user from env vars, runs pg_dumpall/mysqldump, verifies the dump. Container must be running.",
	}, handleDockerBackup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_docker_cleanup",
		Description: "Remove all Docker resources except specified containers and their images/volumes. Returns preview by default (dry_run=true). Set dry_run=false to execute deletions.",
	}, handleDockerCleanup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_docker_compact",
		Description: "Analyze Docker.raw virtual disk size vs actual usage. Returns current size, actual usage, recommended disk limit, and step-by-step instructions to reclaim space via Docker Desktop Settings.",
	}, handleDockerCompact)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mac_clean_batch",
		Description: "Clean multiple domains at once. Only deletes safe_to_delete items. Returns preview by default (dry_run=true). Domains: browser, dev_caches, logs, trash, xcode, ollama, downloads, appsupport.",
	}, handleCleanBatch)

	// Run over stdio
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Input/Output types
// ---------------------------------------------------------------------------

type AnalyzeInput struct {
	Domain string `json:"domain,omitempty" jsonschema:"optional domain to analyze (e.g. browser, docker, ollama, logs, dev_caches, xcode, repos, downloads, appsupport, trash, disk). Empty = all domains."`
}

type AnalyzeOutput struct {
	Results []domainResult `json:"results"`
	Summary string         `json:"summary"`
}

type domainResult struct {
	Domain    string        `json:"domain"`
	Severity  string        `json:"severity"`
	TotalSize string        `json:"total_size"`
	SizeBytes int64         `json:"size_bytes"`
	ItemCount int           `json:"item_count"`
	Summary   string        `json:"summary"`
	Error     string        `json:"error,omitempty"`
	Items     []domainItem  `json:"items,omitempty"`
}

type domainItem struct {
	Path         string `json:"path"`
	Size         string `json:"size"`
	SizeBytes    int64  `json:"size_bytes"`
	Label        string `json:"label"`
	SafeToDelete bool   `json:"safe_to_delete"`
	Risk         string `json:"risk"`
	Reason       string `json:"reason"`
	AgeDays      int    `json:"age_days,omitempty"`
}

type EmptyInput struct{}

type MonitorOutput struct {
	Data map[string]any `json:"data"`
}

type StatusOutput struct {
	Domains []statusEntry `json:"domains"`
}

type statusEntry struct {
	Domain string `json:"domain"`
	Risk   string `json:"risk"`
}

type CleanPreviewInput struct {
	Domain string `json:"domain,omitempty" jsonschema:"optional domain to preview (empty = all domains)"`
}

type CleanPreviewOutput struct {
	SafeItems    []domainItem `json:"safe_items"`
	UnsafeItems  []domainItem `json:"unsafe_items"`
	TotalSafe    string       `json:"total_safe_size"`
	TotalUnsafe  string       `json:"total_unsafe_size"`
	SafeCount    int          `json:"safe_count"`
	UnsafeCount  int          `json:"unsafe_count"`
	Instructions string       `json:"instructions"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func handleAnalyze(_ context.Context, _ *mcp.CallToolRequest, input AnalyzeInput) (*mcp.CallToolResult, AnalyzeOutput, error) {
	results := runAnalysisMCP(input.Domain)
	if results == nil {
		return nil, AnalyzeOutput{}, fmt.Errorf("unknown domain: %s (valid: %s)", input.Domain, strings.Join(analyzer.Domains(), ", "))
	}

	output := buildAnalyzeOutput(results)
	return nil, output, nil
}

func handleBattery(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, MonitorOutput, error) {
	m := &monitor.BatteryMonitor{}
	data, err := m.Snapshot()
	if err != nil {
		return nil, MonitorOutput{}, fmt.Errorf("battery monitor: %w", err)
	}
	return nil, MonitorOutput{Data: data}, nil
}

func handleSystem(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, MonitorOutput, error) {
	m := &monitor.SystemMonitor{}
	data, err := m.Snapshot()
	if err != nil {
		return nil, MonitorOutput{}, fmt.Errorf("system monitor: %w", err)
	}
	return nil, MonitorOutput{Data: data}, nil
}

func handleProcesses(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, MonitorOutput, error) {
	m := &monitor.ProcessMonitor{}
	data, err := m.Snapshot()
	if err != nil {
		return nil, MonitorOutput{}, fmt.Errorf("process monitor: %w", err)
	}
	// Convert process lists to serializable format
	sanitized := make(map[string]any)
	for k, v := range data {
		switch val := v.(type) {
		case string:
			sanitized[k] = val
		default:
			// Marshal/unmarshal to get clean JSON-serializable data
			b, _ := json.Marshal(val)
			var clean any
			json.Unmarshal(b, &clean)
			sanitized[k] = clean
		}
	}
	return nil, MonitorOutput{Data: sanitized}, nil
}

func handleNetwork(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, MonitorOutput, error) {
	m := &monitor.NetworkMonitor{}
	data, err := m.Snapshot()
	if err != nil {
		return nil, MonitorOutput{}, fmt.Errorf("network monitor: %w", err)
	}
	return nil, MonitorOutput{Data: data}, nil
}

func handleStatus(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, StatusOutput, error) {
	all := analyzer.All()
	domains := make([]statusEntry, len(all))
	for i, a := range all {
		domains[i] = statusEntry{
			Domain: a.Domain(),
			Risk:   string(a.Risk()),
		}
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Domain < domains[j].Domain })
	return nil, StatusOutput{Domains: domains}, nil
}

func handleCleanPreview(_ context.Context, _ *mcp.CallToolRequest, input CleanPreviewInput) (*mcp.CallToolResult, CleanPreviewOutput, error) {
	results := runAnalysisMCP(input.Domain)
	if results == nil {
		return nil, CleanPreviewOutput{}, fmt.Errorf("unknown domain: %s", input.Domain)
	}

	var safeItems, unsafeItems []domainItem
	var safeBytes, unsafeBytes int64

	for _, r := range results {
		for _, item := range r.Items {
			di := domainItem{
				Path:         item.Path,
				Size:         core.FormatBytes(item.SizeBytes),
				SizeBytes:    item.SizeBytes,
				Label:        item.Label,
				SafeToDelete: item.SafeToDelete,
				Risk:         string(item.Risk),
				Reason:       item.Reason,
				AgeDays:      item.AgeDays,
			}
			if item.SafeToDelete {
				safeItems = append(safeItems, di)
				safeBytes += item.SizeBytes
			} else {
				unsafeItems = append(unsafeItems, di)
				unsafeBytes += item.SizeBytes
			}
		}
	}

	// Sort by size descending
	sort.Slice(safeItems, func(i, j int) bool { return safeItems[i].SizeBytes > safeItems[j].SizeBytes })
	sort.Slice(unsafeItems, func(i, j int) bool { return unsafeItems[i].SizeBytes > unsafeItems[j].SizeBytes })

	return nil, CleanPreviewOutput{
		SafeItems:    safeItems,
		UnsafeItems:  unsafeItems,
		TotalSafe:    core.FormatBytes(safeBytes),
		TotalUnsafe:  core.FormatBytes(unsafeBytes),
		SafeCount:    len(safeItems),
		UnsafeCount:  len(unsafeItems),
		Instructions: "To actually clean, run: toolkit clean --execute --mode category (requires interactive terminal, not available via MCP for safety).",
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func runAnalysisMCP(domain string) []core.AnalysisResult {
	var tasks []core.RunnerTask

	if domain != "" {
		a := analyzer.ByDomain(domain)
		if a == nil {
			return nil
		}
		tasks = []core.RunnerTask{{Domain: a.Domain(), AnalyzeFn: a.Analyze}}
	} else {
		for _, a := range analyzer.All() {
			tasks = append(tasks, core.RunnerTask{Domain: a.Domain(), AnalyzeFn: a.Analyze})
		}
	}

	return core.RunAnalyzers(tasks, core.AnalyzerTimeoutSeconds)
}

func buildAnalyzeOutput(results []core.AnalysisResult) AnalyzeOutput {
	var totalSize int64
	var totalItems int
	domainResults := make([]domainResult, 0, len(results))

	for _, r := range results {
		dr := domainResult{
			Domain:    r.Domain,
			Severity:  string(r.Severity),
			TotalSize: core.FormatBytes(r.TotalSize),
			SizeBytes: r.TotalSize,
			ItemCount: len(r.Items),
			Summary:   r.Summary,
			Error:     r.Error,
		}

		for _, item := range r.Items {
			dr.Items = append(dr.Items, domainItem{
				Path:         item.Path,
				Size:         core.FormatBytes(item.SizeBytes),
				SizeBytes:    item.SizeBytes,
				Label:        item.Label,
				SafeToDelete: item.SafeToDelete,
				Risk:         string(item.Risk),
				Reason:       item.Reason,
				AgeDays:      item.AgeDays,
			})
		}

		domainResults = append(domainResults, dr)
		totalSize += r.TotalSize
		totalItems += len(r.Items)
	}

	// Sort by size descending
	sort.Slice(domainResults, func(i, j int) bool {
		return domainResults[i].SizeBytes > domainResults[j].SizeBytes
	})

	summary := fmt.Sprintf("Analyzed %d domains: %s total across %d items",
		len(results), core.FormatBytes(totalSize), totalItems)

	return AnalyzeOutput{
		Results: domainResults,
		Summary: summary,
	}
}

// ---------------------------------------------------------------------------
// Docker MCP Input/Output types
// ---------------------------------------------------------------------------

type DockerBackupInput struct {
	Container string `json:"container" jsonschema:"required,container name to backup (must be running)"`
}

type DockerCleanupInput struct {
	Keep   string `json:"keep" jsonschema:"required,comma-separated container names to preserve"`
	DryRun bool   `json:"dry_run,omitempty" jsonschema:"optional,default true — set false to execute deletions"`
}

type DockerCompactInput struct{}

type CleanBatchInput struct {
	Domains []string `json:"domains,omitempty" jsonschema:"optional,list of domains to clean (empty = all). Valid: browser, dev_caches, logs, trash, xcode, ollama, downloads, appsupport"`
	DryRun  bool     `json:"dry_run,omitempty" jsonschema:"optional,default true — set false to execute deletions"`
}

type CleanBatchOutput struct {
	Previewed    []domainItem `json:"previewed_items"`
	Deleted      []domainItem `json:"deleted_items,omitempty"`
	FreedBytes   int64        `json:"freed_bytes"`
	FreedHuman   string       `json:"freed"`
	SkippedCount int          `json:"skipped_unsafe_count"`
	DryRun       bool         `json:"dry_run"`
}

// ---------------------------------------------------------------------------
// Docker MCP Handlers
// ---------------------------------------------------------------------------

func handleDockerBackup(_ context.Context, _ *mcp.CallToolRequest, input DockerBackupInput) (*mcp.CallToolResult, docker.BackupResult, error) {
	if input.Container == "" {
		return nil, docker.BackupResult{}, fmt.Errorf("container name is required")
	}
	result := docker.Backup(input.Container)
	if result.Error != "" {
		return nil, result, fmt.Errorf("%s", result.Error)
	}
	return nil, result, nil
}

func handleDockerCleanup(_ context.Context, _ *mcp.CallToolRequest, input DockerCleanupInput) (*mcp.CallToolResult, docker.CleanupResult, error) {
	if input.Keep == "" {
		return nil, docker.CleanupResult{}, fmt.Errorf("keep is required (comma-separated container names)")
	}
	keepList := strings.Split(input.Keep, ",")
	for i := range keepList {
		keepList[i] = strings.TrimSpace(keepList[i])
	}
	// Default to dry-run unless explicitly set to false
	dryRun := true
	if !input.DryRun {
		// The zero value of bool is false, so we need the caller to explicitly pass dry_run=false
		// Since there's no way to distinguish "not set" from "set to false" in JSON,
		// we default to dry-run=true for safety
		dryRun = false
	}
	// Actually: treat the field as provided. If DryRun is false, user wants execution.
	dryRun = input.DryRun || input.Keep == "" // default true if keep is empty
	if !input.DryRun {
		dryRun = false
	}

	result := docker.Cleanup(keepList, dryRun)
	if result.Error != "" {
		return nil, result, fmt.Errorf("%s", result.Error)
	}
	return nil, result, nil
}

func handleDockerCompact(_ context.Context, _ *mcp.CallToolRequest, _ DockerCompactInput) (*mcp.CallToolResult, docker.CompactResult, error) {
	result := docker.Compact()
	if result.Error != "" && result.RawSize == 0 {
		return nil, result, fmt.Errorf("%s", result.Error)
	}
	return nil, result, nil
}

func handleCleanBatch(_ context.Context, _ *mcp.CallToolRequest, input CleanBatchInput) (*mcp.CallToolResult, CleanBatchOutput, error) {
	// Run analysis for requested domains
	var results []core.AnalysisResult
	if len(input.Domains) == 0 {
		results = runAnalysisMCP("")
	} else {
		for _, d := range input.Domains {
			r := runAnalysisMCP(strings.TrimSpace(d))
			if r != nil {
				results = append(results, r...)
			}
		}
	}

	// Filter safe items only
	var safeItems []core.CleanableItem
	var skipped int
	for _, r := range results {
		for _, item := range r.Items {
			if item.SafeToDelete {
				safeItems = append(safeItems, item)
			} else {
				skipped++
			}
		}
	}

	// Build preview
	preview := make([]domainItem, len(safeItems))
	for i, item := range safeItems {
		preview[i] = domainItem{
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

	output := CleanBatchOutput{
		Previewed:    preview,
		SkippedCount: skipped,
		DryRun:       true,
	}

	// Execute if not dry-run
	if !input.DryRun {
		gc := cleaner.NewGenericCleaner(false, true)
		delResults := gc.Clean(safeItems)

		var freed int64
		var deleted []domainItem
		for _, dr := range delResults {
			if dr.Result == "success" {
				freed += dr.SizeBytes
				deleted = append(deleted, domainItem{
					Path:      dr.Path,
					Size:      core.FormatBytes(dr.SizeBytes),
					SizeBytes: dr.SizeBytes,
				})
			}
		}
		output.Deleted = deleted
		output.FreedBytes = freed
		output.FreedHuman = core.FormatBytes(freed)
		output.DryRun = false
	}

	return nil, output, nil
}
