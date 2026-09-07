# Mac Toolkit — Requirements

Go CLI for macOS supervision, disk cleanup, and system optimization.
Replaces `mac-toolkit-pro` (Python) with a single static binary.

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
| `mac-toolkit processes`  | Top processes by CPU and memory                    |
| `mac-toolkit network`    | WiFi, connections, connectivity monitor            |

#### Global Flags

| Flag                | Type   | Default | Description                              |
| ------------------- | ------ | ------- | ---------------------------------------- |
| `--execute`         | bool   | false   | Actually delete files (dry-run without)  |
| `--domain <name>`   | string | all     | Restrict to a single analyzer domain     |
| `--mode <mode>`     | string | deal    | Approval mode: deal/category/item/checklist |
| `--save`            | bool   | false   | Save report to `reports/` directory      |
| `--last`            | bool   | false   | Show the most recent saved report        |
| `--timeout`         | int    | 120     | Analysis timeout in seconds              |

### 1.2 Interactive Menu

When `toolkit` is invoked without a subcommand, display an interactive menu using
`charmbracelet/huh` (or bubbletea directly). The menu presents all available
commands grouped by category:

- **Disk Cleanup**: analyze, clean, full, status, report
- **System Monitors**: battery, system, processes, network

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
| `markdown` | `.md` file saved to `reports/`                          |
| `json`     | `.json` file saved to `reports/`                        |
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
