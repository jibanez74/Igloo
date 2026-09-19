# Deferred Cleanups

This document records improvements that were identified during code review but deliberately not made at the time, because each one is larger than the task that surfaced it, changes externally observable behavior, or needs a decision that the reviewing task had no mandate to make.

This file is not authoritative. `docs/openapi.json`, `docs/ffmpeg.md`, and `docs/design-system.md` remain the authoritative documents, and nothing here overrides them. Treat any entry added above the last section as a backlog item: when one is picked up, do the work, update whichever authoritative document it touches, and delete the entry. The backlog is currently empty.

The last section records questions that were investigated and answered "leave it alone". It exists so the same suggestions are not re-raised in a future review.

## Settled: Do Not Change These

Each of the following looks like an inconsistency, was investigated, and is correct as written.

**`ParseBitRate` does not trim whitespace.** It is the only parser in `server/cmd/internal/helpers/parsing.go` that does not, and its siblings all do. This is deliberate strictness, asserted by a test: `parsing_test.go` requires `ParseBitRate(" 5000000 ")` to return `0`. ffprobe does not emit padded `bit_rate` values, so a padded value signals that the input did not come from where the caller believes it did.

**`parseSubtitleStartSec` duplicates `ParseDurationSeconds`.** `server/cmd/api/subtitle_handler.go` re-implements the empty, unparsable, NaN, infinite, and negative checks that `helpers.ParseDurationSeconds` already performs. It does so in order to return two distinct client-facing messages, `"invalid start"` and `"start must not be negative"`, which the shared parser's single boolean cannot express. Folding it into the helper would change API error text.

**`static_handler.go` returns plaintext errors rather than the JSON envelope.** It is the only file in `server/cmd/api` that calls `http.Error` instead of `helpers.ErrorJSON`. This matches the contract: `docs/openapi.json` documents `text/plain` responses for 403, 404, and 500 on `/api/static/{path}`. Converting it to the JSON envelope would break the documented contract, not restore it.
