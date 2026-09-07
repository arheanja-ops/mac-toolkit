---
name: add-analyzer
description: Add a new domain analyzer to mac-toolkit
---

# Add Analyzer

Add a new domain analyzer called `$ARGUMENTS` to the mac-toolkit project.

Steps:
1. Read mac-toolkit/internal/analyzer/analyzer.go for the Analyzer interface
2. Read an existing analyzer (e.g., mac-toolkit/internal/analyzer/browser.go) as a template
3. Create mac-toolkit/internal/analyzer/$ARGUMENTS.go implementing:
   - Domain() returning the domain name
   - Risk() returning the appropriate core.RiskLevel
   - Analyze() returning core.AnalysisResult
   - init() calling Register(&XxxAnalyzer{})
4. Create a test in mac-toolkit/internal/analyzer/${ARGUMENTS}_test.go
5. Run `go build ./...` and `go test ./...` to verify
6. The analyzer will auto-register via init() — no other changes needed
