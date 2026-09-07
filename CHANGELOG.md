# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
