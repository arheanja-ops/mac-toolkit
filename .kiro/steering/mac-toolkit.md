---
inclusion: always
description: "Mac Toolkit Go project conventions and patterns"
---

## Mac Toolkit Conventions

### Project Structure
- Go module: `mac-toolkit` at `mac-toolkit/`
- Binary output: `mac-toolkit/bin/toolkit`
- Cobra CLI in `mac-toolkit/cmd/`
- All business logic in `mac-toolkit/internal/`

### Patterns
- Analyzers self-register via `init()` calling `analyzer.Register()`
- Monitors implement `Monitor` interface: `Name()`, `Snapshot()`, `Display()`
- Cleaners check blacklist before every delete via `cleaner.IsBlacklisted()`
- All macOS commands via `exec.Command()` — never shell=true
- Sizes stored as `int64` bytes internally, formatted only for display with `core.FormatBytes()`
- Parallel execution via goroutines + sync.Mutex in `core.RunAnalyzers()`

### Safety
- Dry-run by default — `--execute` flag required
- Blacklist: `/System`, `/usr`, `/bin`, `/sbin`, `/private/var/db`, Apple bundle IDs
- Audit log written for every cleanup session
- Never delete without user approval

### Testing
- Tests use Go stdlib `testing` — no external test frameworks
- Run: `cd mac-toolkit && go test ./... -count=1`
- New analyzers/monitors need tests

### Build
- `cd mac-toolkit && go build -o bin/toolkit .`
- `go vet ./...` must pass clean
- Single binary, zero runtime dependencies beyond cobra
