# Docker Ops & Batch Clean — Design Spec

**Date:** 2026-08-30
**Status:** Draft
**Scope:** mac-toolkit CLI + MCP server

## Problem

Today's cleanup workflow for Docker and dev caches is manual: the user runs `docker inspect` to find the DB user, `pg_dumpall` with the right flags, then manually `docker rm` each container while keeping one, then prunes selectively. Same for dev/browser caches — no single command cleans them all with a preview.

The existing `docker.go` analyzer only reports the `.raw` file size. It doesn't help with backup, selective cleanup, or compaction guidance.

## Features

### F1: `toolkit docker backup <container>`

**CLI:** `toolkit docker backup backstage-postgres`
**MCP:** `mac_docker_backup { container: "backstage-postgres" }`

Behavior:
1. `docker inspect <container>` → extract `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` from env vars
2. Detect DB engine: if image contains `postgres` → `pg_dumpall -U <user>`; if `mysql`/`mariadb` → `mysqldump --all-databases -u<user> -p<pass>`
3. `docker exec <container> <dump_cmd>` → write to `~/docker-backups/<container>-<date>.sql`
4. Verify: file size > 0, ends with expected trailer (`dump complete` for pg, `Dump completed` for mysql)
5. Return: `{ path, size_bytes, databases_found, tables_found, verified }`

Constraints:
- Container must be running
- Only supports postgres and mysql/mariadb (error for others with "unsupported engine")
- Never stores passwords in output — only uses them for the dump command

### F2: `toolkit docker cleanup --keep <container>`

**CLI:** `toolkit docker cleanup --keep backstage-postgres`
**MCP:** `mac_docker_cleanup { keep: "backstage-postgres", dry_run: true }`

Behavior:
1. List all containers (`docker ps -a`)
2. Identify which to keep (by name, comma-separated)
3. Identify kept container's image and volume
4. Preview: show what will be deleted (containers, images, volumes) and what will be kept
5. If `--execute` (CLI) or `dry_run: false` (MCP): run deletions in order: containers → images → volumes → `docker builder prune -a -f`
6. Return: `{ deleted_containers, deleted_images, deleted_volumes, build_cache_freed, kept }`

Constraints:
- Default is dry-run (preview only)
- MCP always returns preview first — actual deletion only with explicit `dry_run: false`
- Uses `docker inspect` to find kept container's image + volume, never deletes those

### F3: `toolkit docker compact`

**CLI:** `toolkit docker compact`
**MCP:** `mac_docker_compact {}`

Behavior:
1. Read `.raw` file size from `core.DockerRawPath`
2. Get actual Docker usage: `docker system df --format json`
3. Calculate recommended disk limit: `max(actual_used * 2, 16GB)`
4. Return: `{ raw_size, actual_used, recommended_limit, instructions }` — instructions are human-readable steps to resize in Docker Desktop Settings

Constraints:
- Read-only — never modifies Docker Desktop settings
- If Docker not running, report `.raw` size and suggest starting Docker first

### F4: `toolkit clean batch --domains <list>`

**CLI:** `toolkit clean batch --domains dev_caches,browser,logs`
**MCP:** `mac_clean_batch { domains: ["dev_caches", "browser", "logs"], dry_run: true }`

Behavior:
1. Run analyzers for requested domains (or all if empty)
2. Filter to `safe_to_delete == true` items only
3. Preview: list items grouped by domain with sizes
4. If `--execute` (CLI) or `dry_run: false` (MCP): delete using existing `GenericCleaner`
5. Return: `{ previewed_items, deleted_items, freed_bytes, skipped_items }`

Constraints:
- Only deletes items marked `safe_to_delete` — never touches `risk: danger` items
- MCP version always previews first
- Uses existing analyzer + cleaner infrastructure

## Architecture

### New files

| File | Package | What |
|---|---|---|
| `internal/docker/ops.go` | `docker` | Backup, cleanup, compact logic |
| `internal/docker/ops_test.go` | `docker` | Tests |
| `cmd/docker.go` | `cmd` | Cobra subcommands: `docker backup`, `docker cleanup`, `docker compact` |

### Modified files

| File | Change |
|---|---|
| `cmd/mcp.go` | Add 4 new MCP tools: `mac_docker_backup`, `mac_docker_cleanup`, `mac_docker_compact`, `mac_clean_batch` |
| `cmd/clean.go` | Add `batch` subcommand |

### Patterns to follow

- Docker ops go in a new `internal/docker/` package (not in `analyzer/` — these are operations, not analysis)
- MCP handlers follow existing typed input/output struct pattern
- CLI commands use `--execute` flag for destructive ops (dry-run default)
- All docker commands shell out to `docker` binary (consistent with existing `docker.go` analyzer)

## Implementation order

1. `internal/docker/ops.go` — backup, cleanup, compact functions
2. `internal/docker/ops_test.go` — unit tests (mock docker CLI output)
3. `cmd/docker.go` — CLI subcommands
4. `cmd/mcp.go` additions — 4 new MCP tools
5. `cmd/clean.go` additions — batch subcommand
6. Build + manual test
