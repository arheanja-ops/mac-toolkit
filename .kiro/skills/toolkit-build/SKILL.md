---
name: toolkit-build
description: Build mac-toolkit binary and run full verification
---

# Build Toolkit

Build and verify the mac-toolkit project.

Steps:
1. Run `cd mac-toolkit && go vet ./...` — fix any issues
2. Run `go test ./... -count=1` — fix any failures
3. Run `go build -o bin/toolkit .` — verify binary builds
4. Run `./bin/toolkit --help` — verify CLI works
5. Run `./bin/toolkit status` — verify analyzers are registered
6. Report: binary size, test count, any warnings

If $ARGUMENTS contains 'release': also run `go build -ldflags='-s -w' -o bin/toolkit .` for a stripped binary.
