# Mac Toolkit — Requirements

Go CLI for macOS supervision, disk cleanup, and system optimization.
Replaces `mac-toolkit-pro` (Python) with a single static binary.

> **Status (v1.7.1).** Sections 1–4 keep the original v1.0 requirements as written
> (history). Where the shipped code differs, section 5 lists the divergences;
> sections 1.9–1.12 add the requirements for work delivered after v1.0 (TUI,
> doctor/AI, safety gates, MCP). Source of truth for flags: `mac-toolkit <cmd> --help`.

---

## 1. Functional Requirements

### 1.1 CLI Commands

The binary is called `toolkit`. It uses **cobra** for command routing.

| Command              | Description                                        |
| -------------------- | -------------------------------------------------- |
| `toolkit`            | No subcommand → interactive menu (huh/bubbletea)   |
| `mac-toolkit analyze`    | Run all 11 analyzers, print results, don't delete  |
| `mac-toolkit clean`      | Analyze → approval → delete (dry-run by default)   |
| `mac-toolkit full`       | Analyze + save report + clean (dry-run by default) |
| `mac-toolkit status`     | Print domain list with risk levels                 |
| `mac-toolkit report`     | Show saved reports                                 |
| `mac-toolkit battery`    | Battery health monitor                             |
| `mac-toolkit system`     | CPU, memory, swap, thermal monitor                 |
| `mac-toolkit memory`     | RAM used (Activity Monitor definition), pressure, swap, top apps |
| `mac-toolkit cpu`        | CPU split, load per core, P/E cores, thermal, top apps |
| `mac-toolkit processes`  | Top processes by CPU and memory                    |
| `mac-toolkit network`    | WiFi, connections, connectivity monitor            |

#### Global Flags

| Flag                | Type   | Default | Description                              |
| ------------------- | ------ | ------- | ---------------------------------------- |
| `--execute`         | bool   | false   | Actually delete files (dry-run without)  |
| `--domain <name>`   | string | all     | Restrict to a single analyzer domain     |
| `--mode <mode>`     | string | deal    | Approval mode: deal/category/item/checklist |
| `--save`            | bool   | false   | Save report to the canonical reports folder (`core.ReportsDir()`) |
| `--last`            | bool   | false   | Show the most recent saved report        |
| `--timeout`         | int    | 120     | Analysis timeout in seconds              |

### 1.2 Interactive Menu

When `toolkit` is invoked without a subcommand, display an interactive menu using
`charmbracelet/huh` (or bubbletea directly). The menu presents all available
commands grouped by category:

- **Disk Cleanup**: analyze, clean, full, status, report
- **System Monitors**: battery, system, memory, cpu, processes, network (all read-only)

### 1.3 Disk Analyzers

11 analyzers run in parallel via goroutines. Each analyzer scans a specific domain
and returns a list of cleanable items with size, age, and risk level.

| Domain        | What it scans                                          | Risk      |
| ------------- | ------------------------------------------------------ | --------- |
| `disk`        | APFS volume usage summary (df)                         | —         |
| `ollama`      | Downloaded LLM models (`~/.ollama/models`)             | 🔴 danger |
| `docker`      | Docker disk image (`~/Library/Containers/com.docker.*`)| 🔴 danger |
| `browser`     | Chrome, Safari, Firefox, Edge caches                   | 🟢 safe   |
| `logs`        | System and app logs older than 7 days                  | 🟢 safe   |
| `downloads`   | Large files, ZIPs, duplicates in `~/Downloads`         | 🟡 warn   |
| `appsupport`  | Application Support caches                             | 🟡 warn   |
| `repos`       | `node_modules`, `.venv`, `__pycache__` inside repos    | 🟢 safe   |
| `dev_caches`  | npm, pip, brew, Gradle, Maven, Cargo, Go caches        | 🟢 safe   |
| `xcode`       | DerivedData, Simulators, Archives                      | 🟢/🟡    |
| `trash`       | `~/.Trash`                                             | 🟡 warn   |

Each analyzer implements a common `Analyzer` interface and is registered in a
central registry. The runner executes all (or a filtered set) in parallel using
`errgroup` with context cancellation.

### 1.4 System Monitors

4 monitors that query macOS hardware and OS state.

| Monitor     | Data sources                           | Key metrics                                       |
| ----------- | -------------------------------------- | ------------------------------------------------- |
| `battery`   | `ioreg`, `pmset`                       | Health %, cycles, temperature, voltage, time left  |
| `system`    | `gopsutil`, `system_profiler`          | Model, CPU%, memory, swap, thermal state           |
| `memory`    | `vm_stat`, `sysctl`, `memory_pressure`, `ps` | Used (App+Wired+Compressed), pressure, swap, swap in/out, uptime, top 5 apps by RAM |
| `cpu`       | `top -l 2`, `sysctl`, `pmset`, `ps` | user/sys/idle, load 1/5/15, load per core, P/E cores, thermal, top 5 apps by CPU |
| `processes` | `gopsutil`                             | Top 10 by CPU, top 10 by memory                   |
| `network`   | `gopsutil`, `airport` (`/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport`) | WiFi SSID, signal, stats, active connections, connectivity |

Monitors use `exec.Command` for macOS-specific CLI tools and `gopsutil` for
cross-platform metrics. Output is formatted with lipgloss styles.

### 1.5 Cleaner

A single `GenericCleaner` handles all file deletion. Before every delete:

1. Verify the path is not in the blacklist.
2. Verify the path is not a symlink pointing into a blacklisted prefix.
3. Log the action to the audit trail.

**Blacklisted paths** (prefix match, never deletable):

- `/System`
- `/usr`
- `/bin`
- `/sbin`
- `/private/var/db`

**Blacklisted identifiers** (bundle or directory names, never deletable):

- `com.apple.dock`
- `com.apple.finder`
- `com.apple.Spotlight`

### 1.6 Reporters

4 output formats, all implementing a common `Reporter` interface.

| Reporter   | Output                                                  |
| ---------- | ------------------------------------------------------- |
| `terminal` | Styled tables via lipgloss + tablewriter to stdout      |
| `markdown` | `.md` file saved to the reports folder                  |
| `json`     | `.json` file saved to the reports folder                |
| `audit`    | `audit.json` — append-only log with UUID session ID     |

Report filenames include a timestamp: `report-2026-08-28T083000.md`.
Audit entries include session UUID, action, path, size, timestamp, and result.

### 1.7 Approval Modes

The `--mode` flag controls how the user approves items for deletion.

| Mode         | Behavior                                               |
| ------------ | ------------------------------------------------------ |
| `deal`       | Show total summary → approve/reject all (default)      |
| `category`   | Prompt s/N per domain                                  |
| `item`       | Prompt s/N per individual file/directory                |
| `checklist`  | Interactive checkbox selection (arrow keys + space)     |

All prompts accept Spanish and English affirmatives: `s`, `si`, `sí`, `y`, `yes`.

### 1.8 Safety

- **Dry-run by default**: without `--execute`, no file is deleted. The CLI prints
  what *would* be deleted with sizes and risk levels.
- Preview table before deletion shows: path, size (human-readable), risk, age.
- Blacklist is enforced at the cleaner layer, independent of analyzers.
- Every session generates an `audit.json` entry for traceability.

### 1.9 Interactive TUI (`internal/tui`) — delivered v1.3.0 → v1.7.0

- `mac-toolkit` with no subcommand (and `mac-toolkit tui`) opens a two-pane TUI
  (Bubble Tea): action list (Disk, Clean, Monitors, Reports, Session) and a
  scrollable detail pane, with a live header (disk, memory, CPU, thermal) refreshed
  every 2 seconds. `mac-toolkit menu` keeps the linear promptui menu as a fallback.
- Keys (normal mode): `↑↓` move, `Tab` switch pane, `⏎` run, `/` filter, `a` analyze,
  `p` preview, `c` clean selection, `d` doctor, `r` refresh doctor, `q`/`Ctrl+C`/`Esc`
  quit.
- Interactive clean: checkbox list where only `safe` items are markable
  (`space` toggle, `a` all safe, `n` none, `⏎` continue, `Esc` cancel), then a
  modal that requires typing `si` before anything is deleted.
- Must stay usable at 80x24 (tested: no line wider than 80 columns, frame ≤ 24 rows).
- Doctor panel: PERFORMANCE / QUICK WINS / REQUIERE REVISIÓN groups, read-only.

### 1.10 Doctor and AI (`internal/ai`) — delivered v1.4.0 (CLI/MCP), v1.7.0 (TUI)

- `mac-toolkit doctor [--no-ai] [--json]` runs all analyzers and monitors and
  returns a prioritized `Diagnosis` (performance, quick wins, review). Strictly
  read-only.
- Privacy by construction: the snapshot sent to any model contains aggregates only
  (sizes per domain, severity, counts, system metrics, hottest process name and %).
  No paths, file names, repo names or PIDs.
- Engines: Groq (OpenAI-compatible, `net/http`, 20 s timeout) when `GROQ_API_KEY` is
  set and `--no-ai` is not; otherwise — or on any AI failure — a deterministic
  heuristic engine. AI errors are never surfaced to the user.
- Env: `GROQ_API_KEY`, `GROQ_MODEL` (default `openai/gpt-oss-20b`),
  `GROQ_BASE_URL` (default `https://api.groq.com/openai/v1`).
- The `disk` domain is context only and never counts as reclaimable space.

### 1.11 Safety gates (see `docs/SAFETY.md`)

- `clean --execute` requires, after the approval mode, a final confirmation that
  shows the item count and total size and accepts only an unambiguous affirmative
  (`core.IsAffirmative`: `si`/`sí`/`yes`/`y`). Anything else, including EOF, aborts.
- The TUI deletes only items with `Risk == safe && SafeToDelete`, after explicit
  selection and the same affirmative gate; it aborts entirely if a non-safe item is
  marked, uses the cleaner (blacklist) and writes the audit log.
- `doctor`, analyzers, monitors, `Preview` and the Doctor panel never modify the
  system.
- Thermal states (`Normal`, `Throttled`, `Warning`, `Unknown`) are read-only and only
  `Throttled`/`Warning` count as an alert; unknown output must never raise an alarm.

### 1.12 MCP server

- `mac-toolkit mcp` (stdio) exposes 12 tools: 9 read-only (`mac_analyze`,
  `mac_battery`, `mac_system`, `mac_processes`, `mac_network`, `mac_status`,
  `mac_clean_preview`, `mac_docker_compact`, `mac_doctor`), 2 destructive
  (`mac_clean_batch`, `mac_docker_cleanup`) and `mac_docker_backup`, which writes a
  dump to `~/docker-backups/` without deleting.
- Destructive tools default to `dry_run=true`: an omitted flag is a dry-run and only
  an explicit `dry_run=false` deletes (`isDryRun`).

### 1.13 Docker operations

- `mac-toolkit docker backup <container>`, `docker cleanup --keep a,b [--execute]`
  (dry-run by default) and `docker compact`.

---

## 2. Non-Functional Requirements

| Requirement              | Detail                                              |
| ------------------------ | --------------------------------------------------- |
| **Platform**             | macOS only — `darwin/arm64` and `darwin/amd64`       |
| **Distribution**         | Single static binary, zero runtime dependencies      |
| **Go version**           | Go 1.22+                                            |
| **Parallelism**          | All 11 analyzers run concurrently via errgroup       |
| **Timeout**              | Configurable per-run, default 120 seconds            |
| **Terminal output**      | Colorized via lipgloss; graceful fallback on dumb terms |
| **Localization**         | Prompts accept Spanish + English input (s/si/y/yes)  |
| **Binary size**          | Target < 15 MB (stripped, no CGO where possible)     |
| **Startup time**         | < 200 ms to first output                             |
| **Test coverage**        | ≥ 80% on core, cleaner, and approval packages        |

---

## 3. Dependencies

All dependencies are pinned to exact versions in `go.mod`.

| Module                                  | Version  | Purpose                            |
| --------------------------------------- | -------- | ---------------------------------- |
| `github.com/spf13/cobra`               | v1.8.1   | CLI framework and command routing  |
| `github.com/shirou/gopsutil/v3`        | v3.24.5  | System metrics (CPU, mem, disk, net, process) |
| `github.com/charmbracelet/lipgloss`    | v0.13.0  | Terminal styling and colors        |
| `github.com/charmbracelet/huh`         | v0.6.0   | Interactive forms and menus        |
| `github.com/olekukonenko/tablewriter`  | v0.0.5   | ASCII table rendering              |
| `github.com/google/uuid`               | v1.6.0   | Audit session UUIDs                |
| `github.com/stretchr/testify`          | v1.9.0   | Test assertions and mocks (test only) |

No additional dependencies should be added without justification. Each dependency
is attack surface and maintenance burden.

---

## 4. Out of Scope

- Linux or Windows support
- Configuration files (YAML, TOML, env) — struct-based config only
- Daemon mode or scheduled execution
- GUI or web interface
- Auto-update mechanism
- Remote/networked cleanup

---

## 5. Divergences between the original requirements and v1.7.1

| Original text | Shipped state (v1.7.1) |
| ------------- | ---------------------- |
| Binary called `toolkit` | Command is `mac-toolkit` (`go install`, release); `make build` still writes `bin/toolkit` |
| No subcommand → huh/bubbletea menu | No subcommand → two-pane TUI; the menu is `mac-toolkit menu` (promptui) |
| "Global flags" `--execute`, `--domain`, `--mode`, `--save`, `--last`, `--timeout` | Per-command flags: `analyze` (`--domain`, `--min-size`, `--save`), `clean` (`--domain`, `--execute`, `--mode`), `full` (`--execute`, `--mode`), `report` (`--last`), `doctor` (`--no-ai`, `--json`); only `-v/--verbose` is global |
| Go 1.22+ | `go 1.25.0` in `go.mod` |
| Parallelism via errgroup; default timeout 120 s | goroutines + `sync.Mutex` (`core.RunAnalyzers`); timeout 180 s (`core.AnalyzerTimeoutSeconds`) |
| Dependencies: gopsutil, huh, tablewriter, uuid, testify | Not used. Direct deps: cobra, bubbletea, lipgloss, bubbles, promptui, go-isatty, MCP go-sdk; stdlib tests |
| Reporter `terminal` via lipgloss + tablewriter | Own ANSI/terminal reporter (`internal/reporter/terminal.go`) |
| `audit.json` append-only | One `audit.json` per cleanup session in `<reports folder>/cleanup_<timestamp>/` |
| Report filename `report-<ts>.md` | `<reports folder>/analysis_<ts>/report.md` and `report.json` |
| `reports/` relative to the working directory | (v1.9.0) Fixed folder `os.UserConfigDir()/mac-toolkit/reports` (macOS: `~/Library/Application Support/mac-toolkit/reports`), independent of the cwd |
| Flat TUI menu | (v1.9.0) Category menu: Health, Disk, Clean, Memory, CPU, Processes, System, Reports, Session |
| Test coverage ≥ 80 % | Not met: 31.3 % `internal/core`, 63.9 % `internal/cleaner` (measured with `go test -cover`) |
| Out of scope: "GUI or web interface" | Still no GUI/web; the terminal TUI (section 1.9) is in scope |
| Spanish/English affirmatives `s`, `si`, `sí`, `y`, `yes` in all prompts | Per-mode approval prompts accept `s`/`si`/`sí`/`y`/`yes` (`internal/core/approval.go`); the final deletion gate and the TUI modal accept only `si`/`sí`/`yes`/`y` (not bare `s`) |
