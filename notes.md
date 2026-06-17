## Cometline
### Bug Fixes
- **deps**: Patch dependabot security vulnerabilities (#4)
- **deps**: Upgrade vitest to eliminate remaining esbuild vulnerability (#5)
- **Sidebar**: Adjust workspace divider styling for improved visibility
### Features
- **Sidebar**: Add workspace divider and improve session grouping display
- **Sidebar**: Enhance session display with slide transition and improved styling

## CometMind
### Bug Fixes
- **deps**: Patch dependabot security vulnerabilities (#4)
- **memory**: Prevent infinite loop in compactor run method
- **tools**: Serialize workspace mutations with per-root mutex
- **agent**: Cap concurrent memory-extraction goroutines with semaphore
- **acp**: Propagate run context into permission handler to prevent leak
- **session**: Make SetTitleIfEmpty atomic to eliminate TOCTOU race
