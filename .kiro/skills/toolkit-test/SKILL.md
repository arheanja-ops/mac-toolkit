---
name: toolkit-test
description: Run mac-toolkit tests and analyze failures
---

# Run Toolkit Tests

Run tests for mac-toolkit and analyze any failures.

1. Run: `cd mac-toolkit && go test ./... -v -count=1`
2. If $ARGUMENTS is provided, run only that package: `go test ./internal/$ARGUMENTS/... -v -count=1`
3. Analyze any failures:
   - Read the failing test file
   - Read the implementation file
   - Identify the root cause
   - Fix the issue
   - Re-run tests to verify
