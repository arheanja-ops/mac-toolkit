# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.9.0] - 2026-11-25

### Added
- **Menú de la TUI por categorías**: HEALTH, DISK, CLEAN, MEMORY, CPU, PROCESSES, SYSTEM,
  REPORTS y SESSION. El cursor salta las cabeceras y la categoría activa siempre se ve
  (cabe en 80x24).
- **Vistas nuevas** (solo lectura): Memory (Overview, Swap & paging, Top apps), CPU
  (Overview, Load, Top apps, Thermal), Processes (Top by CPU, Top by memory, By app,
  Summary) y Reports (Last report, Saved reports, Save report). `r` refresca cualquiera.
- **Save report** en la TUI: guarda `report.md` y `report.json` en la carpeta de reportes
  y muestra la ruta (única entrada que escribe, solo ahí).
- Insight de `doctor` "Carga alta con CPU libre"; el snapshot de IA añade solo
  `cpu_idle_pct` (número).

### Fixed
- **Processes en la TUI** mostraba datos crudos de Go; ahora son tablas por ancho.
- **Last report** decía "no reports found" según el directorio desde el que se lanzaba;
  ahora usa una carpeta fija y, sin reportes, explica cómo crear el primero.
- **CPU% del header y de `system`**: venía del acumulado desde el arranque y costaba
  ~1.4 s de CPU por lectura; ahora sale del intervalo de 1 s de `iostat` (~0 CPU).
- **Veredicto "Saturada"** con la CPU casi ociosa: con carga alta e idle > 50 % ahora dice
  "Carga alta con CPU libre: procesos esperando memoria o disco".
- El footer de la TUI ya no trunca `q salir` en terminales de ~100–120 columnas.

### Changed
- **Ubicación de reportes y audit**: de `reports/` relativo al directorio de trabajo a
  `os.UserConfigDir()/mac-toolkit/reports` (en macOS `~/Library/Application Support/
  mac-toolkit/reports`). Los reportes anteriores no se migran.

## [1.8.0] - 2026-11-23

### Added
- **Vistas dedicadas de Memory y CPU** (solo lectura): comandos `mac-toolkit memory` y
  `mac-toolkit cpu`, acciones `Memory`/`CPU` en el grupo Monitors de la TUI y atajos
  `m`/`u`. `r` refresca la vista abierta (Doctor, Memory o CPU); el refresco es solo
  manual. Memory muestra uso, nivel de presión, desglose, swap con tráfico desde el
  arranque, uptime y top 5 apps por RAM; CPU muestra user/sys/idle, load 1/5/15 con
  ratio por núcleo y veredicto, núcleos P/E, estado térmico y top 5 apps por CPU. Las
  apps se agregan por aplicación y nunca se muestran PIDs ni rutas.
- **Insights nuevos en `doctor`** (sección PERFORMANCE, CLI/TUI/`--json`): CPU
  saturada, tiempo de kernel elevado y paginación intensa, con textos prudentes. El
  snapshot suma solo números (`load_per_core`, `cpu_user_pct`, `cpu_sys_pct`,
  `swap_thrash`, `memory_pressure_level`).
- Clasificadores puros con umbrales documentados en `internal/monitor/classify.go`.

### Changed
- **La memoria usada ahora sigue la definición de Monitor de Actividad**: App Memory
  (Anonymous − Purgeable) + Wired + Compressed, sobre `hw.memsize`. Antes era
  `active + wired + compressed`. `system`, el header de la TUI, `mac_system`, Memory y
  Doctor comparten el mismo cálculo, así que las cifras coinciden (y pueden ser menores
  que antes).
- El footer de la TUI incluye `m memory · u cpu`; en modo compacto (< 90 columnas) se
  abrevia (`p prev`, `m mem`) y omite `/ filtrar`.
- Los tests de la TUI ya no ejecutan los monitores reales (suite más rápida).

## [1.7.2] - 2026-11-23

### Security
- **`docker cleanup --execute` had no human gate.** It ran `docker rm -f`, `rmi`,
  `volume rm` and `builder prune -a -f` with only the flag, no confirmation, no audit
  log, and ignored every exec error. `internal/docker` now separates `Plan` (read-only:
  what is removed, what is kept, reclaimable build cache) from `Apply` (executes only
  the plan, reports each item as success/failure/skipped, never swallows errors) behind
  an injectable `Runner`. The CLI shows the full plan, highlights volumes
  ("pueden contener datos"), requires a typed affirmative (`core.IsAffirmative`) and
  writes an audit log (`docker-cli`). An empty or unknown `--keep` container aborts
  the plan instead of leaving its volumes unprotected.
- **MCP destructive tools deleted without a technical gate.** `mac_docker_cleanup` and
  `mac_clean_batch` with `dry_run=false` now require the server to run with
  `MAC_TOOLKIT_MCP_ALLOW_DELETE=1` (set by the human in the MCP client `env`) and a
  `confirm_plan` equal to the `plan_digest` of the plan recomputed at that moment.
  Previews return `plan_digest`. Both tools write an audit log (`mcp`) and return
  per-item results including failures.

### Changed
- **Behavior change (MCP):** destructive MCP tools no longer execute on `dry_run=false`
  alone. Without the opt-in env var they return an error pointing to the terminal
  commands; without a matching `confirm_plan` they refuse and ask for a new preview.
  Agents and clients must pass `confirm_plan` and the human must enable the opt-in.
- `mac_docker_cleanup` output is now `{dry_run, plan, plan_digest, results, ...}`
  (was `deleted_containers`/`deleted_images`/`deleted_volumes`).

## [1.7.1] - 2026-11-23

### Fixed
- **Thermal state was always reported as `Warning`.** `SystemMonitor` looked for the
  exact phrase "No thermal warnings" in `pmset -g therm`, but recent macOS answers
  "No thermal warning level has been recorded", so it never matched. This polluted
  the TUI header, the "hot process" insight of `doctor`, the Doctor panel and the
  `mac_system` MCP tool. A pure, tested `parseThermal` now returns `Throttled`
  (`CPU_Speed_Limit` < 100), `Warning` (a warning level really recorded), `Normal`
  (any "no thermal warning" wording, or `CPU_Speed_Limit` = 100) or `Unknown`
  (empty/unrecognised output or `pmset` failure — ignorance never raises an alarm).
  `monitor.IsThermalAlert` (allowlist: `Throttled`, `Warning`) is shared by the
  header and the heuristic engine.
- **`--version` showed `dev (commit none, built unknown)` after `go install`.**
  `resolveBuildInfo` now reads `runtime/debug.ReadBuildInfo()` when no ldflags were
  injected, so `go install ...@v1.7.1` reports `v1.7.1`. Values injected by
  GoReleaser always win; commit and date are only real in release binaries.

## [1.7.0] - 2026-11-20

### Added
- TUI: **Doctor** panel (action in the Disk group + `d` shortcut, `r` to refresh):
  PERFORMANCE / QUICK WINS / REQUIERE REVISIÓN groups, disk context and engine
  (Groq/heuristic). Read-only, no cache (every press re-analyzes), spinner while it
  runs, responsive down to 80x24.

### Changed
- The `doctor` snapshot builder moved to `internal/ai/collect.go`
  (`BuildDoctorSnapshot`, `systemMetrics`, `topCPUProcess`, `EngineLabel`) and is
  shared by the CLI, the `mac_doctor` MCP tool and the TUI, with no output changes.

### Docs
- Session hand-off note updated to v1.6.1 (#12).

## [1.6.1] - 2026-11-02

### Fixed
- TUI is responsive at 80 columns: tables adapt to the panel width (SHARE column
  dropped, names truncated with an ellipsis), the full frame fits 80x24 without
  phantom scroll, the left panel has a 22-column minimum and the "Select & clean"
  label is no longer clipped. A test guarantees no line exceeds 80 columns and the
  frame stays within 24 rows.

### Added
- "✓ Listo (<cmd>)" completion line after `analyze`, `doctor`, `system`, `battery`,
  `network`, `processes` and `status` (not emitted in `--json` output).

## [1.6.0] - 2026-11-01

### Added
- TUI-2: interactive **clean** inside the TUI. Selection mode with checkboxes —
  only `safe` items can be marked; `warn`/`danger` rows are shown dimmed with their
  reason and cannot be selected. Keys: `space` toggle, `a` all safe, `n` none,
  `⏎` continue, `Esc` cancel. A confirmation modal requires typing `si` before
  anything is deleted. Deletion goes through the cleaner (permanent blacklist) and
  writes the same audit log as the CLI.
- `core.IsAffirmative`: single shared gate (`si`/`sí`/`yes`/`y`) used by the CLI
  confirmation and the TUI modal.

### Docs
- `docs/SAFETY.md` updated with the interactive clean flow; `doctor` documented in
  the README (#9).

## [1.5.0] - 2026-11-01

### Added
- **Final confirmation gate in `clean --execute`:** after the per-mode approval the
  user must type `si` after seeing the exact item count and total size; anything
  else (including EOF) aborts without deleting.
- `docs/SAFETY.md`: verifiable safety guarantees.

### Changed
- TUI visual redesign: coherent palette, panel borders with embedded titles
  (ACTIONS/DETAILS), header with separators and thermal alert, k9s-style selection
  caret in the left panel, aligned severity-colored analysis table and a footer
  with key hints.

## [1.4.0] - 2026-11-01

### Added
- **`doctor` command** (AI roadmap, phase 1): runs every analyzer plus the system
  monitors and produces a prioritized, read-only diagnosis (performance, quick
  wins, items needing review). Flags: `--no-ai` (force the deterministic engine)
  and `--json` (structured output for scripting).
- PII-free snapshot (`internal/ai`): aggregates only — no paths, file names or PIDs.
- OpenAI-compatible Groq client on `net/http` (no external SDK), active only when
  `GROQ_API_KEY` is set; configurable with `GROQ_MODEL` and `GROQ_BASE_URL`.
- Deterministic heuristic engine used when there is no key or the AI call fails.
- `mac_doctor` MCP tool (the server now exposes 12 tools).

### Fixed
- The `disk` domain is context only and is no longer added to the reclaimable-safe
  bytes.

## [1.3.2] - 2026-09-21

### Fixed
- `repos` analyzer timed out on large trees (1242+ `node_modules`): it measured
  every target sequentially with one `WalkDir` each. It now collects targets in a
  single cheap walk (never descending into a found target) and measures sizes with
  a worker pool bounded to `runtime.NumCPU()`. Output is sorted by path and
  identical to the previous behaviour; the analyzer timeout stays as a guardrail.

### Docs
- README documents the k9s-style TUI (#5).

## [1.3.1] - 2026-09-21

### Added
- TUI welcome dashboard (ASCII banner plus a live system summary).
- Explicit **Quit / Salir** action (Session group); `q`, `Ctrl+C` and `Esc` remain
  global shortcuts.

### Fixed
- Total reclaimable in the analysis table excluded neither the `disk` domain nor
  domains that errored; it now does, shows `disk` as a separate context line and is
  labelled "Total reclaimable (excl. disk)".
- Panel width math (2 borders + gap) and a 1-column separator so panel borders no
  longer touch and the table is no longer cut.

## [1.3.0] - 2026-09-21

### Added
- k9s-style two-pane **TUI** (phase TUI-1): action list on the left, scrollable
  detail viewport on the right, and a live header refreshed every 2 seconds (disk,
  memory, CPU, thermal state; degrades field by field and always shows the version).
- Read-only actions: analyze, status and the four monitors.
- `mac-toolkit` with no arguments opens the TUI; `mac-toolkit menu` keeps the
  previous promptui menu as a fallback.
- Dependencies: charmbracelet `bubbletea`, `lipgloss`, `bubbles`, plus
  `mattn/go-isatty` for TTY detection.

### Docs
- AI roadmap (Groq) and the phase-1 `doctor` spec (#2).

## [1.2.0] - 2026-09-08

### Added
- Start-up banner with version and a live disk-usage bar.
- Menu grouped into sections (Disk/Monitors/Reports) with aligned descriptions.
- Reporter: domains and items sorted by size, proportion bars, top 8 per domain
  with a summary of the rest, colored checkmarks.
- `core` helpers: `ReadDiskSnapshot`, `ProgressBar`, `Colorize` and exported colors.
- `docs/RELEASE_WORKFLOW.md`: PR → merge → tag → GoReleaser contribution flow.

### Changed
- `mac-toolkit` is now the command name everywhere (Cobra command, GoReleaser
  binary, README); the shorter `toolkit` alias/symlink was dropped.

## [1.1.0] - 2026-09-07

### Added
- Interactive arrow-key menu (promptui), grouped into Disk / Monitors / Reports.
- Dedicated menu entry **"Cleanup preview (dry-run, shows risks)"** separate from
  the real clean action.
- `reporter.PrintCleanPlan`: cleanup preview grouped by domain showing risk, size,
  age, safe flag and path per item, with per-domain and grand totals and a risk
  legend.

### Changed
- **Cleanup now always shows the full plan first.** `mac-toolkit clean` renders the
  dry-run preview (with risks) before anything happens. Without `--execute` it
  stops there; with `--execute` it proceeds to approval and a final
  to-be-deleted table before deleting.

## [1.0.1] - 2026-09-07

### Added
- Multi-client MCP setup guide (`docs/MCP_CLIENTS.md`) for Claude Desktop,
  Claude Code, VS Code, Cursor, Windsurf, and Zed, plus a README section.
- `toolkit --version` reporting version, commit and build date (injected at build
  time via ldflags).
- Automated release pipeline: pushing a `vX.Y.Z` tag builds macOS arm64/amd64
  binaries with GoReleaser and publishes a GitHub release with archives and
  checksums.

### Changed
- CI actions upgraded to `actions/checkout@v5` and `actions/setup-go@v6`
  (Node 24 runtime), removing the Node 20 deprecation warning.

## [1.0.0] - 2026-09-07

### Fixed
- **MCP cleanup tools could delete by default.** `mac_docker_cleanup` and
  `mac_clean_batch` used a plain `bool` for `dry_run`, so an omitted flag
  decoded as `false` and executed real deletions — the opposite of the
  documented safe default. The flag is now `*bool`: an omitted value means
  dry-run, and only an explicit `dry_run=false` performs deletions. The
  contradictory branching in `handleDockerCleanup` was replaced with a single
  `isDryRun` helper.
- **Process monitor reported 0.0% CPU/MEM for every process.** System tools like
  `ps` format decimals using the user's locale; on comma-decimal locales
  (e.g. `es_CO`) the output was `87,1` instead of `87.1`, so `ParseFloat`
  failed and yielded `0.0`. All system commands that are parsed numerically now
  run with `LC_ALL=C` via a shared `cmdC` helper.
- **Battery health reported 1%.** On Apple Silicon, `MaxCapacity` is already a
  percentage (0–100), not mAh. The old formula divided it by `DesignCapacity`
  (mAh), producing ~1%. Health now uses `MaxCapacity` directly when it is a
  percentage, and falls back to `AppleRawMaxCapacity / DesignCapacity` on Intel.
- **Network monitor showed 0.0 MB sent/received.** The interface was hardcoded
  to `en0`, but the active interface can be another (e.g. `en9` with a
  USB-Ethernet/dock adapter). The monitor now resolves the default-route
  interface via `route -n get default` and displays it.
- **`repos` and `downloads` analyzers timed out.** `repos` traversed large trees
  including `.git`; `downloads` descended without depth limits. `repos` now skips
  `.git`/`.hg`/`.svn`/`.Trash`, `downloads` is limited to two directory levels,
  and the per-analyzer timeout was raised from 120s to 180s.

### Added
- `cmdC` helper (`internal/monitor/exec.go`) forcing `LC_ALL=C` for deterministic
  numeric parsing of system command output.
- Active network interface shown in the `network` monitor output.
- Implementation & execution guide (`docs/GUIA_IMPLEMENTACION_Y_EJECUCION.md`).

[Unreleased]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.7.1...HEAD
[1.7.1]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.6.1...v1.7.0
[1.6.1]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.3.2...v1.4.0
[1.3.2]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/arheanja-ops/mac-toolkit/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/arheanja-ops/mac-toolkit/releases/tag/v1.0.0
