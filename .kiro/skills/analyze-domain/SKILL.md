---
name: analyze-domain
description: Run analysis on a specific mac-toolkit domain and explain results
---

# Analyze Domain

Run the mac-toolkit analyzer for domain `$ARGUMENTS` and explain the results.

Steps:
1. Build the toolkit if needed: `cd mac-toolkit && go build -o bin/toolkit .`
2. Run: `./mac-toolkit/bin/toolkit analyze --domain $ARGUMENTS`
3. Explain what was found, the severity, and what can be safely cleaned
4. If the domain is unknown, list available domains by running `./mac-toolkit/bin/toolkit status`
