# Deferred Cleanups

This document records improvements that were identified during code review but deliberately not made at the time, because each one is larger than the task that surfaced it, changes externally observable behavior, or needs a decision that the reviewing task had no mandate to make.

This file is not authoritative. `docs/openapi.json`, `docs/ffmpeg.md`, and `docs/design-system.md` remain the authoritative documents, and nothing here overrides them. Treat the entries below as a backlog: when one is picked up, do the work, update whichever authoritative document it touches, and delete the entry.

The last section records questions that were investigated and answered "leave it alone". It exists so the same suggestions are not re-raised in a future review.

## HLS Fallback Profile Selection Ignores Two Profiles

`BestFitHLSFallbackProfile` in `server/cmd/internal/helpers/hls_profiles.go` walks `HLSAllowedProfiles` in order and returns the first transcode profile whose configured height fits within the source height. Three profiles share `Height: 1080` — `1080p_8mbps`, `1080p_6mbps`, and `1080p_4mbps` — and `1080p_8mbps` comes first in the ordered list. Any source at or above 1080 therefore always selects `1080p_8mbps`, and the 6 Mbps and 4 Mbps profiles are unreachable through this function. They remain reachable when a client requests them explicitly, so this is a gap in automatic fallback selection rather than dead configuration.

The open question is what the fallback should do, and height alone cannot answer it. A 1080p source that failed remux prevalidation on a constrained server is exactly the case where the lower-bitrate profiles would help, which suggests the selection should also consider the source bitrate, the transcode limiter's current load, or the requesting client, rather than resolution alone.

This is a playback behavior change. Read `docs/ffmpeg.md` first, update it in the same task, and add coverage to `server/cmd/internal/helpers/hls_profiles_test.go`, which currently asserts only that the allowed list is ordered by descending height.

## Inline sql.Null Literals Bypass the helpers Constructors

`server/cmd/internal/helpers/nulls.go` provides `NullString`, `NullInt64`, and `NullFloat64`, and they are used widely. Alongside them, 53 composite literals in non-test code construct `sql.NullString`, `sql.NullInt64`, or `sql.NullFloat64` directly. The heaviest concentrations are `server/cmd/internal/scanner/music/persistence.go`, `server/cmd/internal/scanner/streams.go`, `server/cmd/api/settings_handler.go`, and `server/cmd/api/movie_playlist_handler.go`.

This is not a mechanical substitution, and that is the reason it was deferred. The helpers map a zero value to SQL NULL, whereas `sql.NullInt64{Int64: v, Valid: true}` stores the zero. Replacing one with the other silently changes what lands in the database wherever zero is a meaningful value.

`server/cmd/internal/scanner/show/metadata.go:22` shows why the distinction matters. A single `UpdateShowMetadataParams` literal mixes both styles: `TmdbID` uses `helpers.NullInt64`, while `VoteAverage`, `VoteCount`, `Popularity`, `TmdbSeasonCount`, and `TmdbEpisodeCount` use explicit `Valid: true` literals. For a TMDB vote average, storing a genuine `0.0` rather than NULL is very likely correct and deliberate. The mixture is therefore meaningful, but nothing in the code says so.

The useful work is to decide per call site which semantics are intended, convert only the ones where zero should become NULL, and add a short comment where an explicit `Valid: true` literal is load-bearing. Query-level tests should accompany any conversion, since the failure mode is a wrong value in the database rather than a compile error.

## Settled: Do Not Change These

Each of the following looks like an inconsistency, was investigated, and is correct as written.

**`ParseBitRate` does not trim whitespace.** It is the only parser in `server/cmd/internal/helpers/parsing.go` that does not, and its siblings all do. This is deliberate strictness, asserted by a test: `parsing_test.go` requires `ParseBitRate(" 5000000 ")` to return `0`. ffprobe does not emit padded `bit_rate` values, so a padded value signals that the input did not come from where the caller believes it did.

**`parseSubtitleStartSec` duplicates `ParseDurationSeconds`.** `server/cmd/api/subtitle_handler.go` re-implements the empty, unparsable, NaN, infinite, and negative checks that `helpers.ParseDurationSeconds` already performs. It does so in order to return two distinct client-facing messages, `"invalid start"` and `"start must not be negative"`, which the shared parser's single boolean cannot express. Folding it into the helper would change API error text.

**`static_handler.go` returns plaintext errors rather than the JSON envelope.** It is the only file in `server/cmd/api` that calls `http.Error` instead of `helpers.ErrorJSON`. This matches the contract: `docs/openapi.json` documents `text/plain` responses for 403, 404, and 500 on `/api/static/{path}`. Converting it to the JSON envelope would break the documented contract, not restore it.
