# Server Follow-up Findings

This document records deferred findings from the server audit performed on 2026-07-10. The constants findings have been resolved; the remaining control-flow cleanup is intentionally deferred.

## Inline assignments in control flow

A static scan found 154 production, non-generated `if` statements with inline assignments, contrary to the server control-flow rule. Generated sqlc files and test files were excluded.

The largest concentrations at audit time were:

- `playlist_handler.go`: 12
- `watch_progress_handler.go`: 11
- `playback_settings_handler.go`: 11
- `user_handler.go`: 10
- `music_scanner_helpers.go`: 10

Reproduce the scan with:

```bash
rg -n --glob '*.go' --glob '!*_test.go' --glob '!**/database/**' '^\s*if\b[^\n]*:=' server/cmd
```

Address these incrementally by assigning function results, errors, map lookups, and comparisons before the `if`. Keep each cleanup behavior-preserving and run focused tests followed by `make check`.
