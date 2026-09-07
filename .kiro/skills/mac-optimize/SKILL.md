---
name: mac-optimize
description: Run full Mac optimization analysis and recommendations
---

# Mac Optimization

Run a comprehensive Mac optimization analysis.

1. Build toolkit: `cd mac-toolkit && go build -o bin/toolkit .`
2. Run full analysis: `./bin/toolkit analyze`
3. Run system check: `./bin/toolkit system`
4. Run battery check: `./bin/toolkit battery`
5. Run process check: `./bin/toolkit processes`
6. Analyze results and provide:
   - Top 3 items consuming the most space
   - Items that are safe to clean immediately
   - System health summary (CPU, memory, battery)
   - Specific recommendations for optimization
   - If $ARGUMENTS is 'clean', also suggest the exact clean commands
