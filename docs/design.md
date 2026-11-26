# Mac Toolkit — Design

Architecture and technical design for the Go CLI.

> **Status (v1.7.1).** Sections 1–7 are the original v1.0 design, kept as history.
> Sections 8–12 document what was added afterwards (TUI, doctor/AI, safety gates,
> MCP/Docker, thermal parsing) and section 13 lists where the shipped code differs
> from the original text. Update the status of a decision here; do not rewrite it.

---

## 1. Package Layout

```
mac-toolkit/
├── cmd/                          # Cobra command definitions
│   ├── root.go                   # Root command, global flags, no-subcommand → menu
│   ├── analyze.go                # analyze command
│   ├── clean.go                  # clean command (analyze → approve → delete)
│   ├── full.go                   # full command (analyze → save → clean)
│   ├── status.go                 # status command (domain list + risk levels)
│   ├── report.go                 # report command (last report formatted; --share, --list, --open)
│   ├── battery.go                # battery monitor command
│   ├── system.go                 # system monitor command
│   ├── processes.go              # processes monitor command
│   ├── network.go                # network monitor command
│   ├── (memory, cpu)             # memory/cpu commands live in monitors.go (v1.8.0)
│   └── menu.go                   # Interactive menu (huh)
│
├── internal/
│   ├── core/                     # Shared infrastructure
│   │   ├── config.go             # Paths, thresholds, blacklists, timeouts
│   │   ├── models.go             # CleanableItem, AnalysisResult, RiskLevel, etc.
│   │   ├── runner.go             # Parallel analyzer execution (errgroup + context)
│   │   ├── logger.go             # Styled console output (lipgloss wrappers)
│   │   └── approval.go           # 4-mode approval engine
│   │
│   ├── analyzer/                 # Disk analysis
│   │   ├── analyzer.go           # Analyzer interface
│   │   ├── registry.go           # Analyzer registry (name → instance map)
│   │   ├── disk.go               # APFS volume usage (df)
│   │   ├── ollama.go             # Ollama LLM models
│   │   ├── docker.go             # Docker disk image
│   │   ├── browser.go            # Chrome, Safari, Firefox, Edge caches
│   │   ├── logs.go               # System/app logs > 7 days
│   │   ├── downloads.go          # Large files, ZIPs, duplicates in ~/Downloads
│   │   ├── appsupport.go         # Application Support caches
│   │   ├── repos.go              # node_modules, .venv, __pycache__ in repos
│   │   ├── dev_caches.go         # npm, pip, brew, Gradle, Maven, Cargo, Go
│   │   ├── xcode.go              # DerivedData, Simulators, Archives
│   │   └── trash.go              # ~/.Trash
│   │
│   ├── monitor/                  # System monitors
│   │   ├── monitor.go            # Monitor interface
│   │   ├── battery.go            # ioreg + pmset parsing
│   │   ├── system.go             # gopsutil + system_profiler
│   │   ├── processes.go          # gopsutil top processes
│   │   └── network.go            # gopsutil + airport utility
│   │
│   ├── cleaner/                  # File deletion
│   │   ├── cleaner.go            # Cleaner interface + blacklist definitions
│   │   └── generic.go            # GenericCleaner implementation
│   │
│   └── reporter/                 # Output formatting
│       ├── reporter.go           # Reporter interface
│       ├── terminal.go           # lipgloss + tablewriter to stdout
│       ├── markdown.go           # .md file writer
│       ├── json.go               # .json file writer
│       └── audit.go              # Append-only audit.json with UUID sessions
│
├── main.go                       # Entry point: cmd.Execute()
├── go.mod
├── go.sum
├── Makefile                      # build, test, lint, clean targets
└── docs/
    ├── requirements.md
    ├── design.md
    └── tasks.md
```

Everything under `internal/` is unexported. The `cmd/` package is the only
consumer of internal packages.

---

## 2. Key Design Decisions

### 2.1 Interface-driven architecture

Each subsystem defines a Go interface. Implementations are concrete structs.
This enables testing with mocks and adding new domains without touching existing code.

### 2.2 Parallel analysis with errgroup

Analyzers run concurrently using `golang.org/x/sync/errgroup` with a shared
`context.Context`. If the timeout fires or any analyzer returns a fatal error,
the context is cancelled and remaining goroutines exit cleanly.

### 2.3 macOS-specific tools via exec.Command

Battery, system, and network monitors shell out to macOS utilities:

| Tool               | Used by       | Purpose                          |
| ------------------ | ------------- | -------------------------------- |
| `ioreg`            | battery       | Battery serial data              |
| `pmset -g batt`    | battery       | Charge %, time remaining         |
| `system_profiler`  | system        | Hardware model, chip             |
| `df`               | disk analyzer | Volume usage                     |
| `airport -I`       | network       | WiFi SSID, signal strength       |
| `memory_pressure`  | system, memory | Memory pressure level / free %  |
| `vm_stat`          | system, memory | Page counters (used = App + Wired + Compressed) |
| `sysctl`           | memory, cpu   | hw.memsize, vm.swapusage, kern.boottime, vm.loadavg, hw.ncpu, hw.perflevel* |
| `top -l 2`         | cpu           | user/sys/idle (2nd sample)       |
| `ps`               | memory, cpu   | RSS/%CPU aggregated per app (no PIDs, no paths) |

All `exec.Command` calls use argument arrays (no shell interpolation). Timeouts
are enforced via `context.WithTimeout`. Output is parsed with string splitting
or regex — no eval.

### 2.4 Struct-based configuration

No config files. All paths, thresholds, and blacklists live in `core/config.go`
as Go constants and struct literals. This keeps the binary self-contained and
eliminates a class of "missing config" errors.

### 2.5 Sizes in bytes

All sizes are stored as `int64` (bytes) internally. Human-readable formatting
(KB, MB, GB) happens only at the display layer in reporters and logger.

### 2.6 Approval engine

The approval module reads from `os.Stdin` via a `bufio.Scanner`. It normalizes
input to lowercase and accepts Spanish + English affirmatives: `s`, `si`, `sí`,
`y`, `yes`. The `checklist` mode delegates to `charmbracelet/huh` for
interactive selection.

### 2.7 Blacklist enforcement

The blacklist is defined in `core/config.go` and checked in `cleaner/cleaner.go`.
Every path passed to `os.Remove` or `os.RemoveAll` must first pass
`IsBlacklisted(path) == false`. The check:

1. Canonicalizes the path (`filepath.EvalSymlinks` + `filepath.Clean`).
2. Rejects if any blacklisted prefix matches.
3. Rejects if the final path component matches a blacklisted identifier.

This runs at the cleaner layer, not the analyzer layer, so no analyzer bug
can bypass it.

### 2.8 Audit logging

Each `toolkit clean --execute` or `toolkit full --execute` invocation creates
a session UUID via `google/uuid`. Every delete action appends a JSON line to
`audit.json` with: session ID, timestamp, path, size, risk, result (ok/error).

---

## 3. Interface Definitions

### 3.1 Analyzer

```go
package analyzer

import "github.com/user/mac-toolkit/internal/core"

// Analyzer scans a specific domain and returns cleanable items.
type Analyzer interface {
    // Name returns the domain identifier (e.g., "browser", "docker").
    Name() string

    // Description returns a human-readable summary of what this analyzer scans.
    Description() string

    // RiskLevel returns the default risk for items found by this analyzer.
    RiskLevel() core.RiskLevel

    // Analyze scans the domain and returns found items.
    // The context carries the timeout deadline.
    Analyze(ctx context.Context) (*core.AnalysisResult, error)
}
```

### 3.2 Monitor

```go
package monitor

// Snapshot holds the collected metrics for a monitor domain.
type Snapshot struct {
    Title  string
    Fields []Field
}

type Field struct {
    Label string
    Value string
    Style string // lipgloss style key: "ok", "warn", "danger", "info"
}

// Monitor collects and displays system metrics.
type Monitor interface {
    // Name returns the monitor identifier (e.g., "battery").
    Name() string

    // Collect gathers current metrics from the system.
    Collect(ctx context.Context) (*Snapshot, error)

    // Display renders the snapshot to the terminal.
    Display(snap *Snapshot) error
}
```

### 3.3 Cleaner

```go
package cleaner

import "github.com/user/mac-toolkit/internal/core"

// CleanResult reports what happened to a single item.
type CleanResult struct {
    Item  core.CleanableItem
    Error error
}

// Cleaner deletes approved items from disk.
type Cleaner interface {
    // Clean removes the given items. If dryRun is true, no files are deleted.
    // Returns a result per item (success or error).
    Clean(items []core.CleanableItem, dryRun bool) []CleanResult
}
```

### 3.4 Reporter

```go
package reporter

import "github.com/user/mac-toolkit/internal/core"

// Reporter formats and outputs analysis results.
type Reporter interface {
    // Report renders the analysis results.
    // For terminal: prints to stdout.
    // For file-based reporters: writes to the canonical reports folder (`core.ReportsDir()`).
    Report(results []core.AnalysisResult) error
}
```

---

## 4. Core Models

```go
package core

import "time"

// RiskLevel classifies how dangerous a cleanup action is.
type RiskLevel int

const (
    RiskSafe   RiskLevel = iota // 🟢 safe to delete
    RiskWarn                     // 🟡 review recommended
    RiskDanger                   // 🔴 data loss possible
)

// CleanableItem represents a single file or directory that can be deleted.
type CleanableItem struct {
    Path      string
    Size      int64     // bytes
    Risk      RiskLevel
    Domain    string    // analyzer name that found it
    ModTime   time.Time // last modification time
    IsDir     bool
}

// AnalysisResult groups items found by a single analyzer.
type AnalysisResult struct {
    Domain      string
    Description string
    Risk        RiskLevel
    Items       []CleanableItem
    TotalSize   int64         // sum of all item sizes
    Duration    time.Duration // how long the analysis took
    Error       error         // non-nil if the analyzer failed
}

// ApprovalMode controls how the user approves deletions.
type ApprovalMode string

const (
    ModeDeal      ApprovalMode = "deal"
    ModeCategory  ApprovalMode = "category"
    ModeItem      ApprovalMode = "item"
    ModeChecklist ApprovalMode = "checklist"
)
```

---

## 5. Data Flow

### 5.1 Analyze

```
toolkit analyze [--domain X] [--save] [--timeout N]
        │
        ▼
  registry.Get(domain)          // all analyzers or filtered
        │
        ▼
  runner.RunAll(ctx, analyzers) // errgroup, parallel, timeout
        │
        ▼
  []AnalysisResult
        │
        ├──▶ terminal.Report(results)   // always: print to stdout
        │
        └──▶ markdown.Report(results)   // if --save
             json.Report(results)        // if --save
```

### 5.2 Clean

```
toolkit clean [--execute] [--mode M] [--domain X]
        │
        ▼
  (same analyze flow)
        │
        ▼
  []AnalysisResult
        │
        ▼
  approval.Filter(results, mode)  // interactive prompts
        │
        ▼
  []CleanableItem (approved)
        │
        ▼
  cleaner.Clean(items, dryRun)    // blacklist check → delete or simulate
        │
        ▼
  []CleanResult
        │
        ▼
  audit.Write(sessionID, results) // append to audit.json
  terminal.Report(summary)        // print what was deleted/simulated
```

### 5.3 Monitor

```
toolkit battery | system | memory | cpu | processes | network
        │
        ▼
  monitor.Collect(ctx)    // exec.Command + gopsutil
        │
        ▼
  *Snapshot
        │
        ▼
  monitor.Display(snap)   // lipgloss-styled output
```

### 5.4 Interactive Menu

```
toolkit (no args)
        │
        ▼
  menu.Show()              // huh form with grouped options
        │
        ▼
  selected command string
        │
        ▼
  cmd.Execute(selected)   // dispatch to the chosen cobra command
```

---

## 6. Error Handling

- Analyzer errors are captured per-domain in `AnalysisResult.Error`. A failed
  analyzer does not abort the entire run — other analyzers continue.
- `exec.Command` failures (tool not installed, permission denied) return a
  descriptive error. The monitor prints "unavailable" for that metric.
- Cleaner errors are captured per-item in `CleanResult.Error`. A failed delete
  does not abort remaining deletes.
- The CLI exits with code 0 on success, 1 on partial failure (some analyzers/deletes
  failed), 2 on total failure (no results produced).

---

## 7. Testing Strategy

- **Unit tests**: Each analyzer, monitor, cleaner, and reporter has unit tests.
  File-system analyzers use `os.MkdirTemp` to create disposable directory trees.
- **Blacklist tests**: Verify that every blacklisted path and identifier is
  rejected, including symlink-through-blacklist scenarios.
- **Approval tests**: Inject `strings.Reader` as stdin to simulate user input
  for all 4 modes.
- **Integration tests**: End-to-end test of `analyze` and `clean --execute` on
  a temp directory with known contents.
- **No mocks for gopsutil**: Use real system calls in CI (macOS runners).
  Skip gracefully on non-darwin.

---

## 8. TUI (`internal/tui`) — v1.3.0 → v1.7.0

Bubble Tea program (`tui.Run`) started by `mac-toolkit` (no args) or `mac-toolkit tui`.
It needs a TTY; without one it returns an error that suggests `mac-toolkit menu`.

| File | Responsibility |
| ---- | -------------- |
| `tui.go` | `Run`: TTY check, `tea.NewProgram(..., tea.WithAltScreen())` |
| `model.go` | Model, `Update`/`View`, key routing per mode, footer hints |
| `actions.go` | `buildActions()`: Disk (Analyze, Status, Doctor), Clean (Preview, Select & clean), Monitors, Reports, Session (Quit); formatters for analysis/status/monitors |
| `header.go` | Live header (disk, memory, CPU, thermal), refreshes on its own, degrades field by field |
| `welcome.go` | Welcome dashboard shown on open |
| `layout.go`, `styles.go`, `delegate.go` | Width math, truncation, palette, list delegate with selection caret |
| `clean.go` | Clean flow: `buildCleanRows`, `toggleMark`, `markAllSafe`, `runCleanCmd`, `writeCleanAudit` |
| `doctor.go` | Doctor panel: `doctorFn` (injectable), `runDoctor`, `formatDiagnosis` |

**Modes.** The model has four modes — `normalMode`, `selectMode`, `confirmMode`,
`cleaningMode`. The clean modes own the keyboard while active. Side effects run in
`tea.Cmd`s (spinner while they run), never in `Update`.

**Clean flow.** `c` → `enterSelect` (fresh analysis, safe rows pre-marked; entering
never deletes) → `selectMode` (`space`, `a`, `n`, `⏎`, `Esc`) → `confirmMode` (type
`si`, `core.IsAffirmative`) → `cleaningMode` (`runCleanCmd` → `cleaner.NewGenericCleaner(false, true).Clean`
→ `writeCleanAudit` with approval mode `tui-select`). Only rows with `Risk == safe &&
SafeToDelete` are markable; `warn`/`danger` rows are listed dimmed with their reason.

**Layout.** `bodyH = height - 5`; the frame (header + spacing + panels + footer) must
fit 80x24. Below 90 columns the footer drops the navigation hints and keeps
`⏎ a c p d / q`. Tests assert no line exceeds 80 columns and at most 24 rows.

**Testability.** `analysisFn`, `doctorFn` and `cleanerFn` are package variables so
tests inject fixed data and never touch the filesystem, the monitors, Groq or the
real cleaner.

Specs: `docs/specs/2026-09-21-tui-k9s.md`, `2026-11-01-tui-clean.md`,
`2026-11-02-tui-doctor.md`.

---

## 9. Doctor and AI (`internal/ai`) — v1.4.0 → v1.7.0

```
analyzers + monitors ─▶ BuildDoctorSnapshot ─▶ AISnapshot (PII-free)
                                   │
                          Diagnose(snap, Options{NoAI})
                          ├─ NoAI or no GROQ_API_KEY ─▶ Heuristic(snap)
                          └─ GroqDiagnose (20 s) ─ok─▶ Diagnosis
                                              └fail─▶ Heuristic(snap)   (silent)
```

| File | Responsibility |
| ---- | -------------- |
| `snapshot.go` | `AISnapshot`, `DomainSummary`, `BuildSnapshot`: the privacy gate. Aggregates only; `disk` is context and excluded from reclaimable totals |
| `collect.go` | `BuildDoctorSnapshot(analyze)`: the single assembly point for CLI, MCP and TUI (analysis source is injected); `systemMetrics`, `topCPUProcess` (name + %, no PID); `EngineLabel` |
| `heuristic.go` | Deterministic engine and source of truth for the `Diagnosis` schema (memory pressure, hot process, disk level, quick wins, review) |
| `groq.go` | OpenAI-compatible client over `net/http`; env `GROQ_API_KEY`, `GROQ_MODEL`, `GROQ_BASE_URL`; the response is validated against the schema and the engine label is stamped by the program |
| `diagnose.go` / `diagnosis.go` | Orchestrator, `Options`, `Diagnosis`/`Insight` types, severities (`high`/`medium`/`low`), engines (`groq`/`heuristic`) |

Privacy is enforced when the snapshot is built, not by trusting a prompt. The package
is read-only: insights carry a suggested `action` in text, but nothing is executed.
Consumers: `cmd/doctor.go` (`--no-ai`, `--json`), MCP `mac_doctor` (`no_ai`), TUI
Doctor panel. Roadmap and specs: `docs/specs/2026-09-09-ai-roadmap.md`,
`2026-09-09-ai-doctor.md`.

---

## 10. Safety gates

| Gate | Where |
| ---- | ----- |
| Dry-run by default | `cmd/clean.go`, `cmd/docker.go` (`--execute`), MCP `isDryRun` |
| Per-mode approval; EOF/empty ⇒ no | `internal/core/approval.go` |
| Final `si` confirmation with count + total | `confirmFinalDeletion` (`cmd/clean.go`) |
| One shared affirmative gate | `core.IsAffirmative` (`internal/core/affirmative.go`) used by CLI and TUI |
| Only `safe` deletable from the TUI + abort-all defence | `internal/tui/clean.go` |
| Blacklist before every delete | `cleaner.IsBlacklisted` via `cleaner.Delete` (`internal/cleaner`, prefixes and bundle IDs in `core/config.go`) |
| Audit log per real cleanup | `reporter.AuditReporter` (`cmd/clean.go`, `internal/tui/clean.go`) |

Known gaps (documented, not hidden): `docker cleanup --execute` (CLI) and
`mac_docker_cleanup` (MCP) use `internal/docker` directly — no `si` gate, blacklist or
audit log; `mac_clean_batch` deletes through the cleaner (blacklist applies) but
writes no audit log. See `docs/SAFETY.md`.

---

## 11. MCP server and Docker

`cmd/mcp.go` registers 12 tools with `mcp.AddTool` (go-sdk): 9 read-only, 2
destructive with `dry_run *bool` (nil ⇒ dry-run) and `mac_docker_backup` (writes a
dump to `~/docker-backups/`, no `dry_run`). `internal/docker` implements `Backup`
(postgres/mysql auto-detected from the image), `Cleanup(keep, dryRun)` and
`Compact`, used by both `cmd/docker.go` and the MCP handlers.

---

## 12. Thermal state (`internal/monitor/thermal.go`) — v1.7.1

`parseThermal(pmset -g therm output)` is a pure function with table tests. Priority:
reduced `CPU_Speed_Limit` (`Throttled`) > a really recorded warning level
(`Warning`) > explicit all-clear or `CPU_Speed_Limit = 100` (`Normal`); anything else
is `Unknown` ("ignorance must not raise an alarm"). `IsThermalAlert` is an allowlist
(`Throttled`, `Warning`) shared by the TUI header and the heuristic engine.

---

## 13. Divergences between the original design and v1.7.1

| Original text | Shipped state |
| ------------- | ------------- |
| `cmd/battery.go`, `system.go`, `processes.go`, `network.go`, `menu.go` (huh) | Monitors live in `cmd/monitors.go`; `cmd/menu.go` is a promptui menu; `cmd/tui.go` holds `tui`/`menu` commands; plus `cmd/doctor.go`, `cmd/docker.go`, `cmd/mcp.go` |
| Parallelism with errgroup + context | goroutines + `sync.Mutex` in `core.RunAnalyzers`, per-analyzer timeout 180 s |
| Monitors use gopsutil | Monitors use `top`, `vm_stat`, `sysctl`, `ps`, `netstat`, airport, `pmset`, `ioreg` via `exec.Command` (with `cmdC` forcing `LC_ALL=C`) |
| Reporter `terminal` with lipgloss + tablewriter | Own terminal reporter; lipgloss only in the TUI |
| "Interactive Menu" is the no-args behaviour (5.4) | No-args opens the TUI (section 8); the menu is `mac-toolkit menu` |
| Exit codes 0/1/2 (section 6) | `Execute()` exits 1 when Cobra returns an error; partial-failure codes are not implemented |
| Integration tests, symlink blacklist tests (section 7) | Not present; the suite is 108 top-level tests (167 with subtests) across `cmd`, `ai`, `analyzer`, `cleaner`, `core`, `monitor`, `tui` |
| MCP server version | `mcp.Implementation.Version` is a fixed `"1.0.0"` in `cmd/mcp.go` |


## v1.11.0 — Listening mode

- `internal/watch`: `Sampler` (light readings via `monitor.ReadLight`, rates from the
  swap counters, `statfs` for disk) → `Evaluator` (sustained-threshold `Rules`, windows,
  gap reset, escalation, resolution) → `Notifier` (osascript, cooldown, hourly cap) and
  `Store` (daily JSONL, 7-day `Prune` limited to `YYYY-MM-DD.jsonl`). `Engine` ties them
  on a ticker and exposes a concurrent-safe `Status`. `Summarize`/`Hourly` feed `history`
  and the TUI.
- `internal/core/reports.go`: `HistoryDir()` (sibling of `ReportsDir`).
- `internal/tui`: `tui.Run` starts the engine; the header reuses its last sample;
  `watch_view.go` adds the ALERTS views and the `n`/`w` keys.
- Spec: `docs/specs/2026-11-25-watch-mode.md`.

## v1.10.0 — Report v2

- `internal/reporter`: one `Report` model (`model.go`, `schema_version` 1) built once by
  `Build(Inputs)` (`build.go`, `build_system.go`) from the analysis, a single monitor
  reading (`ai.CollectMetrics`) and the diagnosis; `Collect` gathers the inputs. Markdown
  (`markdown.go`), JSON (`json.go`, `LoadJSON`) and HTML (`html.go` + `html_template.go`,
  `html/template`, inline CSS) only render it. `Share()` (`share.go`) returns the redacted
  variant. `SaveReport` (`save.go`) writes both variants with 0600 permissions and replaces
  `SaveAnalysis`. `OpenReport` (`open.go`) runs `open` through the injectable `OpenFn`,
  only for a `report.html` inside the reports folder.
- Reclaimable total = safe + review, without the `disk` domain (volume = context).
- `internal/core/reports.go`: `ReportEntry` gains `Dir`, `Formats`, `HasShare`.
- `internal/tui/report_render.go`: `RenderReport(r, w)`, shared by the TUI (Last report)
  and `mac-toolkit report`. `reports_view.go` adds Open report (`o`).
- Spec: `docs/specs/2026-11-25-report-v2.md`.

## v1.9.0 — Categories, process views and reports

- `internal/core/reports.go`: `ReportsDir()` = `os.UserConfigDir()/mac-toolkit/reports`
  (absolute; `ReportsDirOverride` only for tests), `NewReportDir`, `ListReports`,
  `LatestReport`. `internal/reporter/save.go`: `SaveAnalysis`, the single save path
  (`report`, `analyze --save`, `full`, TUI). The audit log lives in the same folder.
- `internal/monitor`: exported `ProcInfo`/`ProcSummary`; `ClassifyLoad(ratio, idle)`
  adds the "high load, idle CPU" verdict; `cpusample.go` reads the CPU split from
  `iostat -c 2 -w 1`; `SystemMonitor{Light}` skips `system_profiler` for the header.
- `internal/tui`: `menu.go` draws the category menu itself (the `bubbles/list` keeps only
  state), `processes_view.go`, `reports_view.go`; every view is a `viewID` with a runner.
- Spec: `docs/specs/2026-11-25-tui-categories.md`.

## v1.8.0 — Memory and CPU views

- `internal/monitor`: `MemoryMonitor`, `CPUMonitor`, `classify.go` (pure threshold
  functions), `apps.go` (per-app aggregation, no PID/path), `memstats.go` (parsers and
  the single `readMemUsage` source of "used memory"). External commands go through the
  injectable `runCmd`, so tests feed synthetic output.
- `internal/tui`: `viewID`/`openView` generalizes the Doctor flag; `memory_view.go`,
  `cpu_view.go` and `views.go` (own lipgloss bars). Manual refresh only (`r`).
- `internal/ai`: `AISnapshot` gains numeric fields; `collect.go` copies an allowlist of
  monitor keys; `heuristic.go` adds three PERFORMANCE rules. Spec:
  `docs/specs/2026-11-23-tui-memory-cpu.md`.
