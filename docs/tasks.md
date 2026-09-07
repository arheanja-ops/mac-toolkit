# Mac Toolkit — Tasks

Implementation plan broken into 8 phases. Each task is a self-contained unit
of work that can be implemented and tested independently.

---

## Phase 1: Core Infrastructure

Foundation packages that everything else depends on.

- [ ] **1.1** Initialize Go module: `go mod init`, `go.mod` with pinned deps, `main.go` stub
- [ ] **1.2** `internal/core/config.go` — paths (`~/.Trash`, `~/Downloads`, `~/Library/Caches`, etc.), thresholds (log age: 7 days, large file: 100 MB), blacklisted prefixes and identifiers
- [ ] **1.3** `internal/core/models.go` — `RiskLevel` enum, `CleanableItem` struct, `AnalysisResult` struct, `ApprovalMode` constants
- [ ] **1.4** `internal/core/logger.go` — lipgloss style definitions (safe/warn/danger/info), `PrintHeader()`, `PrintSuccess()`, `PrintWarning()`, `PrintError()`, `FormatSize(int64) string`
- [ ] **1.5** `internal/core/runner.go` — `RunAll(ctx, []Analyzer) []AnalysisResult` using errgroup with configurable timeout, collect results via mutex-protected slice
- [ ] **1.6** `internal/core/approval.go` — `Filter(results, mode, reader) []CleanableItem` implementing deal, category, item modes with stdin reader; accept s/si/sí/y/yes
- [ ] **1.7** Tests for Phase 1: config path expansion, model constructors, logger formatting, runner parallelism + timeout, approval input parsing

---

## Phase 2: Analyzers

Interface definition and all 11 domain implementations.

- [ ] **2.1** `internal/analyzer/analyzer.go` — `Analyzer` interface (Name, Description, RiskLevel, Analyze)
- [ ] **2.2** `internal/analyzer/registry.go` — `Register()`, `Get(name)`, `All()` functions; auto-register all analyzers at init
- [ ] **2.3** `internal/analyzer/disk.go` — parse `df -h /` output for APFS volume usage
- [ ] **2.4** `internal/analyzer/ollama.go` — scan `~/.ollama/models/`, list model dirs with sizes
- [ ] **2.5** `internal/analyzer/docker.go` — scan `~/Library/Containers/com.docker.docker/Data/vms/` for Docker disk image
- [ ] **2.6** `internal/analyzer/browser.go` — scan Chrome (`~/Library/Caches/Google/Chrome`), Safari, Firefox (`~/Library/Caches/Firefox`), Edge cache dirs
- [ ] **2.7** `internal/analyzer/logs.go` — scan `~/Library/Logs/`, `/var/log/` (readable entries), filter files older than 7 days
- [ ] **2.8** `internal/analyzer/downloads.go` — scan `~/Downloads/` for files > 100 MB, `.zip`/`.dmg`/`.pkg` files, duplicates by name pattern
- [ ] **2.9** `internal/analyzer/appsupport.go` — scan `~/Library/Application Support/` for known cache subdirs
- [ ] **2.10** `internal/analyzer/repos.go` — walk common project dirs (`~/repos`, `~/projects`, `~/Developer`, `~/code`) for `node_modules/`, `.venv/`, `__pycache__/`
- [ ] **2.11** `internal/analyzer/dev_caches.go` — npm (`~/.npm`), pip (`~/Library/Caches/pip`), brew (`~/Library/Caches/Homebrew`), Gradle (`~/.gradle/caches`), Maven (`~/.m2/repository`), Cargo (`~/.cargo/registry`), Go (`~/go/pkg/mod/cache`)
- [ ] **2.12** `internal/analyzer/xcode.go` — DerivedData (`~/Library/Developer/Xcode/DerivedData`), iOS Simulators, Archives
- [ ] **2.13** `internal/analyzer/trash.go` — scan `~/.Trash/` contents with sizes
- [ ] **2.14** Tests for Phase 2: each analyzer tested with temp directories containing mock file structures; registry lookup tests

---

## Phase 3: Monitors

Interface definition and 4 monitor implementations.

- [ ] **3.1** `internal/monitor/monitor.go` — `Monitor` interface (Name, Collect, Display), `Snapshot` and `Field` structs
- [ ] **3.2** `internal/monitor/battery.go` — parse `ioreg -rn AppleSmartBattery` for health, cycles, temperature, voltage; parse `pmset -g batt` for charge % and time remaining
- [ ] **3.3** `internal/monitor/system.go` — `gopsutil` for CPU%, memory, swap; `system_profiler SPHardwareDataType` for model/chip; `memory_pressure` for pressure level
- [ ] **3.4** `internal/monitor/processes.go` — `gopsutil` process list, sort by CPU% (top 10) and memory% (top 10), format as tables
- [ ] **3.5** `internal/monitor/network.go` — `gopsutil` net stats and connections; `airport -I` for WiFi SSID and signal; basic connectivity check (DNS resolve)
- [ ] **3.6** Tests for Phase 3: mock exec.Command output for each macOS tool; verify parsing logic; display formatting tests

---

## Phase 4: Cleaner

Blacklist enforcement and file deletion.

- [ ] **4.1** `internal/cleaner/cleaner.go` — `Cleaner` interface, `IsBlacklisted(path) bool` with prefix matching + symlink resolution + identifier matching
- [ ] **4.2** `internal/cleaner/generic.go` — `GenericCleaner` struct implementing `Clean(items, dryRun)`: blacklist check → `os.RemoveAll` (dirs) / `os.Remove` (files) → return `[]CleanResult`
- [ ] **4.3** Tests for Phase 4: blacklist rejects all protected paths including via symlinks; cleaner deletes temp files in execute mode; cleaner skips in dry-run mode; cleaner handles permission errors gracefully

---

## Phase 5: Reporters

4 output format implementations.

- [ ] **5.1** `internal/reporter/reporter.go` — `Reporter` interface
- [ ] **5.2** `internal/reporter/terminal.go` — lipgloss-styled tables via tablewriter; color-coded risk levels; summary row with totals
- [ ] **5.3** `internal/reporter/markdown.go` — generate `.md` report with timestamp filename, domain sections, tables, totals; write to `reports/` dir
- [ ] **5.4** `internal/reporter/json.go` — serialize `[]AnalysisResult` to indented JSON with timestamp filename; write to `reports/` dir
- [ ] **5.5** `internal/reporter/audit.go` — `AuditReporter` with session UUID; `Write(sessionID, []CleanResult)` appends JSON lines to `audit.json`
- [ ] **5.6** Tests for Phase 5: terminal reporter produces expected table output; markdown/json reporters create correct files in temp dir; audit appends without corrupting existing entries

---

## Phase 6: CLI Commands

Cobra command definitions and interactive menu.

- [ ] **6.1** `cmd/root.go` — root command with global flags (--execute, --domain, --mode, --save, --timeout, --last); run menu when no subcommand
- [ ] **6.2** `cmd/analyze.go` — wire registry → runner → terminal reporter; respect --domain and --save flags
- [ ] **6.3** `cmd/clean.go` — wire analyze → approval → cleaner → audit; respect --execute, --mode, --domain flags
- [ ] **6.4** `cmd/full.go` — combine analyze --save + clean; respect all flags
- [ ] **6.5** `cmd/status.go` — print domain table with risk levels using terminal reporter
- [ ] **6.6** `cmd/report.go` — list saved reports in `reports/`; --last shows most recent; print file contents to stdout
- [ ] **6.7** `cmd/battery.go` — instantiate battery monitor, collect, display
- [ ] **6.8** `cmd/system.go` — instantiate system monitor, collect, display
- [ ] **6.9** `cmd/processes.go` — instantiate processes monitor, collect, display
- [ ] **6.10** `cmd/network.go` — instantiate network monitor, collect, display
- [ ] **6.11** `cmd/menu.go` — huh form with grouped command options; dispatch to selected command
- [ ] **6.12** `main.go` — `func main() { cmd.Execute() }`
- [ ] **6.13** Tests for Phase 6: cobra command registration (all commands exist); flag parsing; help text generation

---

## Phase 7: Tests

Comprehensive test pass and coverage verification.

- [ ] **7.1** Verify all unit tests pass: `go test ./...`
- [ ] **7.2** Add integration test: create temp tree with known sizes → `analyze` → verify results match expected
- [ ] **7.3** Add integration test: `clean --execute` on temp tree → verify files deleted, blacklisted paths untouched
- [ ] **7.4** Add integration test: `clean` (dry-run) → verify no files deleted
- [ ] **7.5** Add approval mode tests: inject stdin with all 4 modes, verify correct items pass through
- [ ] **7.6** Add reporter round-trip test: analyze → json report → parse JSON → verify structure
- [ ] **7.7** Verify test coverage ≥ 80% on core, cleaner, approval: `go test -cover ./internal/...`
- [ ] **7.8** Run `go vet ./...` and fix all warnings

---

## Phase 8: Build & Polish

Makefile, build verification, final cleanup.

- [ ] **8.1** `Makefile` with targets: `build` (darwin/arm64 + amd64), `test`, `lint` (go vet + staticcheck), `clean`, `install`
- [ ] **8.2** Build for both architectures, verify binary runs: `./toolkit --help`
- [ ] **8.3** Verify interactive menu works without subcommand
- [ ] **8.4** Verify `toolkit analyze` runs all 11 analyzers and prints results
- [ ] **8.5** Verify `toolkit clean` shows dry-run preview without deleting
- [ ] **8.6** Verify `toolkit clean --execute --mode checklist` shows interactive checkboxes
- [ ] **8.7** Verify all 4 monitor commands produce output: battery, system, processes, network
- [ ] **8.8** Verify `--save` writes reports to `reports/` with timestamp filenames
- [ ] **8.9** Verify `audit.json` is created on `--execute` runs
- [ ] **8.10** Strip binary (`go build -ldflags="-s -w"`), verify size < 15 MB
- [ ] **8.11** Update `README.md` with usage, build instructions, and architecture overview

---

## Task Dependencies

```
Phase 1 (core) ─────────┬──▶ Phase 2 (analyzers) ──┐
                         │                           │
                         ├──▶ Phase 3 (monitors)     ├──▶ Phase 6 (CLI) ──▶ Phase 7 (tests) ──▶ Phase 8 (build)
                         │                           │
                         ├──▶ Phase 4 (cleaner) ─────┤
                         │                           │
                         └──▶ Phase 5 (reporters) ───┘
```

Phases 2, 3, 4, and 5 can be developed in parallel once Phase 1 is complete.
Phase 6 requires all four. Phase 7 and 8 are sequential at the end.

---

## Estimated Effort

| Phase | Tasks | Estimate  |
| ----- | ----- | --------- |
| 1     | 7     | 1 session |
| 2     | 14    | 2 sessions |
| 3     | 6     | 1 session |
| 4     | 3     | 0.5 session |
| 5     | 6     | 1 session |
| 6     | 13    | 1.5 sessions |
| 7     | 8     | 1 session |
| 8     | 11    | 1 session |
| **Total** | **68** | **~9 sessions** |
