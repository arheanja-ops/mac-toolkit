# Mac Toolkit — Tasks

Implementation plan broken into 8 phases (+ phases 9-12 added for later work).
Each task is a self-contained unit of work that can be implemented and tested
independently.

**Estado a v1.7.1.** Leyenda: `[x]` entregado · `[~]` parcial o entregado de forma
distinta a lo planeado (ver la nota del ítem) · `[ ]` pendiente. El plan original
no se reescribe: solo se actualiza su estado. Las tareas 7.2–7.7 y 8.10 siguen
pendientes: cobertura medida hoy = 31.3 % en `internal/core` y 63.9 % en
`internal/cleaner` (no se alcanza el 80 % de 7.7), y el binario local sin strip
pesa ~18 MB (8.10 apunta a < 15 MB con `-ldflags="-s -w"`, que GoReleaser aplica; no
medido en el release).

---

## Phase 1: Core Infrastructure

Foundation packages that everything else depends on.

- [x] **1.1** Initialize Go module: `go mod init`, `go.mod` with pinned deps, `main.go` stub
- [x] **1.2** `internal/core/config.go` — paths (`~/.Trash`, `~/Downloads`, `~/Library/Caches`, etc.), thresholds (log age: 7 days, large file: 100 MB), blacklisted prefixes and identifiers
- [x] **1.3** `internal/core/models.go` — `RiskLevel` enum, `CleanableItem` struct, `AnalysisResult` struct, `ApprovalMode` constants
- [x] **1.4** `internal/core/logger.go` — lipgloss style definitions (safe/warn/danger/info), `PrintHeader()`, `PrintSuccess()`, `PrintWarning()`, `PrintError()`, `FormatSize(int64) string` — _entregado como_ salida ANSI propia (no lipgloss) en `logger.go`
- [x] **1.5** `internal/core/runner.go` — `RunAll(ctx, []Analyzer) []AnalysisResult` using errgroup with configurable timeout, collect results via mutex-protected slice — _entregado con_ goroutines + `sync.Mutex` (`RunAnalyzers`), no errgroup
- [x] **1.6** `internal/core/approval.go` — `Filter(results, mode, reader) []CleanableItem` implementing deal, category, item modes with stdin reader; accept s/si/sí/y/yes
- [~] **1.7** Tests for Phase 1: config path expansion, model constructors, logger formatting, runner parallelism + timeout, approval input parsing — _parcial_: hay tests de models y runner (+ `IsAffirmative`); faltan config, logger y approval

---

## Phase 2: Analyzers

Interface definition and all 11 domain implementations.

- [x] **2.1** `internal/analyzer/analyzer.go` — `Analyzer` interface (Name, Description, RiskLevel, Analyze)
- [x] **2.2** `internal/analyzer/registry.go` — `Register()`, `Get(name)`, `All()` functions; auto-register all analyzers at init — `Register()`, `ByDomain()`, `All()`
- [x] **2.3** `internal/analyzer/disk.go` — parse `df -h /` output for APFS volume usage
- [x] **2.4** `internal/analyzer/ollama.go` — scan `~/.ollama/models/`, list model dirs with sizes
- [x] **2.5** `internal/analyzer/docker.go` — scan `~/Library/Containers/com.docker.docker/Data/vms/` for Docker disk image
- [x] **2.6** `internal/analyzer/browser.go` — scan Chrome (`~/Library/Caches/Google/Chrome`), Safari, Firefox (`~/Library/Caches/Firefox`), Edge cache dirs
- [x] **2.7** `internal/analyzer/logs.go` — scan `~/Library/Logs/`, `/var/log/` (readable entries), filter files older than 7 days
- [x] **2.8** `internal/analyzer/downloads.go` — scan `~/Downloads/` for files > 100 MB, `.zip`/`.dmg`/`.pkg` files, duplicates by name pattern
- [x] **2.9** `internal/analyzer/appsupport.go` — scan `~/Library/Application Support/` for known cache subdirs
- [x] **2.10** `internal/analyzer/repos.go` — walk common project dirs (`~/repos`, `~/projects`, `~/Developer`, `~/code`) for `node_modules/`, `.venv/`, `__pycache__/`
- [x] **2.11** `internal/analyzer/dev_caches.go` — npm (`~/.npm`), pip (`~/Library/Caches/pip`), brew (`~/Library/Caches/Homebrew`), Gradle (`~/.gradle/caches`), Maven (`~/.m2/repository`), Cargo (`~/.cargo/registry`), Go (`~/go/pkg/mod/cache`)
- [x] **2.12** `internal/analyzer/xcode.go` — DerivedData (`~/Library/Developer/Xcode/DerivedData`), iOS Simulators, Archives
- [x] **2.13** `internal/analyzer/trash.go` — scan `~/.Trash/` contents with sizes
- [~] **2.14** Tests for Phase 2: each analyzer tested with temp directories containing mock file structures; registry lookup tests — _parcial_: `analyzer_test.go` y `repos_test.go`; no hay un test por analyzer con árboles temporales

---

## Phase 3: Monitors

Interface definition and 4 monitor implementations.

- [x] **3.1** `internal/monitor/monitor.go` — `Monitor` interface (Name, Collect, Display), `Snapshot` and `Field` structs — _entregado como_ `Name`, `Snapshot`, `Display` (sin `Collect`)
- [x] **3.2** `internal/monitor/battery.go` — parse `ioreg -rn AppleSmartBattery` for health, cycles, temperature, voltage; parse `pmset -g batt` for charge % and time remaining
- [x] **3.3** `internal/monitor/system.go` — `gopsutil` for CPU%, memory, swap; `system_profiler SPHardwareDataType` for model/chip; `memory_pressure` for pressure level — _entregado con_ `top`, `vm_stat`, `sysctl`, `system_profiler`, `memory_pressure`, `pmset -g therm` (sin gopsutil)
- [x] **3.4** `internal/monitor/processes.go` — `gopsutil` process list, sort by CPU% (top 10) and memory% (top 10), format as tables — _entregado con_ `ps` (sin gopsutil)
- [x] **3.5** `internal/monitor/network.go` — `gopsutil` net stats and connections; `airport -I` for WiFi SSID and signal; basic connectivity check (DNS resolve) — _entregado con_ `netstat`, airport y check HTTP (sin gopsutil)
- [~] **3.6** Tests for Phase 3: mock exec.Command output for each macOS tool; verify parsing logic; display formatting tests — _parcial_: solo `thermal_test.go` (parser térmico)

---

## Phase 4: Cleaner

Blacklist enforcement and file deletion.

- [x] **4.1** `internal/cleaner/cleaner.go` — `Cleaner` interface, `IsBlacklisted(path) bool` with prefix matching + symlink resolution + identifier matching
- [x] **4.2** `internal/cleaner/generic.go` — `GenericCleaner` struct implementing `Clean(items, dryRun)`: blacklist check → `os.RemoveAll` (dirs) / `os.Remove` (files) → return `[]CleanResult`
- [~] **4.3** Tests for Phase 4: blacklist rejects all protected paths including via symlinks; cleaner deletes temp files in execute mode; cleaner skips in dry-run mode; cleaner handles permission errors gracefully — _parcial_: tests de blacklist, dry-run, not-found, success y blacklisted; sin test de symlinks

---

## Phase 5: Reporters

4 output format implementations.

- [x] **5.1** `internal/reporter/reporter.go` — `Reporter` interface
- [x] **5.2** `internal/reporter/terminal.go` — lipgloss-styled tables via tablewriter; color-coded risk levels; summary row with totals — _entregado sin_ tablewriter
- [x] **5.3** `internal/reporter/markdown.go` — generate `.md` report with timestamp filename, domain sections, tables, totals; write to `reports/` dir
- [x] **5.4** `internal/reporter/json.go` — serialize `[]AnalysisResult` to indented JSON with timestamp filename; write to `reports/` dir
- [x] **5.5** `internal/reporter/audit.go` — `AuditReporter` with session UUID; `Write(sessionID, []CleanResult)` appends JSON lines to `audit.json` — _entregado como_ `audit.json` por sesión en `reports/cleanup_<fecha_hora>/` (no JSON lines acumulado)
- [ ] **5.6** Tests for Phase 5: terminal reporter produces expected table output; markdown/json reporters create correct files in temp dir; audit appends without corrupting existing entries

---

## Phase 6: CLI Commands

Cobra command definitions and interactive menu.

- [x] **6.1** `cmd/root.go` — root command with global flags (--execute, --domain, --mode, --save, --timeout, --last); run menu when no subcommand — `mac-toolkit` sin subcomando abre la **TUI**; los flags son por comando, no globales (solo `-v/--verbose`)
- [x] **6.2** `cmd/analyze.go` — wire registry → runner → terminal reporter; respect --domain and --save flags
- [x] **6.3** `cmd/clean.go` — wire analyze → approval → cleaner → audit; respect --execute, --mode, --domain flags
- [x] **6.4** `cmd/full.go` — combine analyze --save + clean; respect all flags
- [x] **6.5** `cmd/status.go` — print domain table with risk levels using terminal reporter
- [x] **6.6** `cmd/report.go` — list saved reports in `reports/`; --last shows most recent; print file contents to stdout
- [x] **6.7** `cmd/battery.go` — instantiate battery monitor, collect, display — los 4 monitores viven en `cmd/monitors.go`
- [x] **6.8** `cmd/system.go` — instantiate system monitor, collect, display
- [x] **6.9** `cmd/processes.go` — instantiate processes monitor, collect, display
- [x] **6.10** `cmd/network.go` — instantiate network monitor, collect, display
- [x] **6.11** `cmd/menu.go` — huh form with grouped command options; dispatch to selected command — _entregado con_ promptui (no huh); desde v1.3.0 el default es la TUI y el menú queda en `mac-toolkit menu`
- [x] **6.12** `main.go` — `func main() { cmd.Execute() }`
- [~] **6.13** Tests for Phase 6: cobra command registration (all commands exist); flag parsing; help text generation — _parcial_: `root_test.go` (versión) y `clean_test.go` (gate); sin test de registro de comandos

---

## Phase 7: Tests

Comprehensive test pass and coverage verification.

- [x] **7.1** Verify all unit tests pass: `go test ./...` — 108 tests de nivel superior pasan (`go test ./... -count=1`)
- [ ] **7.2** Add integration test: create temp tree with known sizes → `analyze` → verify results match expected
- [ ] **7.3** Add integration test: `clean --execute` on temp tree → verify files deleted, blacklisted paths untouched
- [ ] **7.4** Add integration test: `clean` (dry-run) → verify no files deleted
- [ ] **7.5** Add approval mode tests: inject stdin with all 4 modes, verify correct items pass through
- [ ] **7.6** Add reporter round-trip test: analyze → json report → parse JSON → verify structure
- [ ] **7.7** Verify test coverage ≥ 80% on core, cleaner, approval: `go test -cover ./internal/...`
- [x] **7.8** Run `go vet ./...` and fix all warnings — `go vet ./...` limpio

---

## Phase 8: Build & Polish

Makefile, build verification, final cleanup.

- [x] **8.1** `Makefile` with targets: `build` (darwin/arm64 + amd64), `test`, `lint` (go vet + staticcheck), `clean`, `install` — _parcial_: targets `all/build/test/lint/clean/install`; sin staticcheck; binarios por arquitectura vía GoReleaser
- [x] **8.2** Build for both architectures, verify binary runs: `./toolkit --help` — los binarios darwin arm64/amd64 los produce GoReleaser en cada tag
- [~] **8.3** Verify interactive menu works without subcommand — _distinto_: sin subcomando abre la TUI; el menú es `mac-toolkit menu`
- [x] **8.4** Verify `toolkit analyze` runs all 11 analyzers and prints results
- [x] **8.5** Verify `toolkit clean` shows dry-run preview without deleting
- [~] **8.6** Verify `toolkit clean --execute --mode checklist` shows interactive checkboxes — implementado (`approval.go`); no re-verificado de forma interactiva en esta revisión
- [x] **8.7** Verify all 4 monitor commands produce output: battery, system, processes, network
- [x] **8.8** Verify `--save` writes reports to `reports/` with timestamp filenames
- [x] **8.9** Verify `audit.json` is created on `--execute` runs — `audit.json` se escribe tras `clean --execute` y tras borrar desde la TUI
- [ ] **8.10** Strip binary (`go build -ldflags="-s -w"`), verify size < 15 MB
- [x] **8.11** Update `README.md` with usage, build instructions, and architecture overview

---

## Phase 9: TUI (`internal/tui`, v1.3.0 → v1.7.0)

- [x] **9.1** TUI-1 (v1.3.0): dos paneles (acciones + viewport), header en vivo cada 2 s (disco, memoria, CPU, térmico); `mac-toolkit` sin args abre la TUI, `menu` queda como fallback
- [x] **9.2** (v1.3.1) Pantalla de bienvenida, acción Quit / Salir, total reclaimable sin `disk` ni dominios con error, ancho de paneles
- [x] **9.3** (v1.5.0) Rediseño: paleta, bordes con título, caret de selección, tabla coloreada por severidad, footer con key-hints
- [x] **9.4** TUI-2 (v1.6.0) clean interactivo: checkboxes (solo `safe` marcables), teclas espacio/`a`/`n`/⏎/Esc, modal "si", cleaner con blacklist, audit log (`tui-select`)
- [x] **9.5** (v1.6.1) Responsive hasta 80x24 con test de ancho/alto y línea "✓ Listo (<cmd>)"
- [x] **9.6** TUI-3 (v1.7.0) panel Doctor: acción en Disk, atajo `d`, `r` refresca, solo lectura
- [ ] **9.7** TUI-2.1: limpiar items `warn`/`danger` desde la TUI con confirmación por item (no implementado)
- [ ] **9.8** Acciones Docker desde la TUI (no implementado)

---

## Phase 10: Doctor / IA (`internal/ai`, v1.4.0 → v1.7.0)

- [x] **10.1** `snapshot.go`: agregado sin PII (sin rutas, nombres de archivo ni PIDs); `disk` solo como contexto, fuera de lo reclaimable
- [x] **10.2** `heuristic.go`: motor determinista (fallback)
- [x] **10.3** `groq.go`: cliente OpenAI-compatible sobre `net/http`, timeout 20 s, `GROQ_API_KEY`/`GROQ_MODEL`/`GROQ_BASE_URL`
- [x] **10.4** `diagnose.go`: orquestador con fallback silencioso al heurístico
- [x] **10.5** `cmd/doctor.go` con `--no-ai` y `--json`; tool MCP `mac_doctor` (v1.4.0)
- [x] **10.6** `collect.go` (v1.7.0): `BuildDoctorSnapshot` compartido por CLI, MCP y TUI
- [x] **10.7** Tests: `snapshot_test`, `heuristic_test`, `groq_test`, `collect_test`, `doctor_test` (TUI)
- [ ] **10.8** Fase 2 IA: `doctor` agente (function-calling) — no implementado
- [ ] **10.9** Fase 3 IA: `optimize --safe` — no implementado

---

## Phase 11: Seguridad (gates)

- [x] **11.1** Confirmación final `si` en `clean --execute` (v1.5.0, `confirmFinalDeletion`)
- [x] **11.2** `core.IsAffirmative` compartido por CLI y TUI (v1.6.0)
- [x] **11.3** Solo `safe` borrable en la TUI + defensa final que aborta si hay un item no-safe marcado
- [x] **11.4** MCP: `dry_run` como `*bool`; `isDryRun(nil) == true` (v1.0.0)
- [x] **11.5** `docs/SAFETY.md` con las garantías y su ubicación en el código
- [ ] **11.6** Gate `si` y audit log para `docker cleanup --execute` y `mac_docker_cleanup` (hoy solo dry-run por defecto; no implementado)

---

## Phase 12: Estados térmicos y versión (v1.7.1)

- [x] **12.1** `internal/monitor/thermal.go`: `parseThermal` → Normal / Throttled / Warning / Unknown; `IsThermalAlert` (allowlist) con tests de tabla
- [x] **12.2** `--version` con `go install`: `resolveBuildInfo` lee `ReadBuildInfo` solo si no hay ldflags

---

## Phase 13: Higiene pendiente

- [ ] **13.1** `gofmt -l` marca `internal/analyzer/*`, `internal/docker/ops.go` e `internal/tui/helpers_test.go`: PR de formato puro
- [ ] **13.2** Versión del servidor MCP: `cmd/mcp.go` anuncia `Version: "1.0.0"` fija en `mcp.Implementation` (no sigue la versión del binario)

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
