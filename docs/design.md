# Mac Toolkit — Design

Architecture and technical design for the Go CLI.

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
│   ├── report.go                 # report command (--last, list saved reports)
│   ├── battery.go                # battery monitor command
│   ├── system.go                 # system monitor command
│   ├── processes.go              # processes monitor command
│   ├── network.go                # network monitor command
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
| `memory_pressure`  | system        | Memory pressure level            |

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
    // For file-based reporters: writes to reports/ directory.
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
toolkit battery | system | processes | network
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
