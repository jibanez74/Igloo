# Movie Scanner TMDB Reliability and Matching Plan

## Summary

This plan covers completion of movie scanning, TMDB recovery and matching, and moving MJPEG imports:

1. Prevent failed TMDB lookups from erasing existing movie metadata, and retry those lookups during future scans.
2. Preserve identified movie identities during automatic refreshes and concurrent manual Identify.
3. Improve TMDB matching so title similarity outweighs release-year and popularity signals.
4. Accept moving MJPEG video while rejecting attached artwork as movie video.

The music scanner and shared filesystem lifecycle remain unchanged.

A low-confidence match must still receive metadata from the highest-ranked TMDB result. Confidence must never block, postpone, or remove TMDB enrichment.

## Plain-language explanation

- If TMDB is temporarily unavailable, Igloo will keep everything it already knows about the movie—including its title, overview, artwork, genres, cast, and collection—and try TMDB again during a later scan.
- A new movie can still be added and played while TMDB is unavailable. It will remain queued for metadata enrichment.
- Retrying metadata for an unchanged file will not rerun ffprobe or recreate its streams and chapters.
- Once a movie has a TMDB ID, automatic scans will refresh that exact movie instead of guessing its identity again. Only the manual Identify action may select a different TMDB movie.
- Matching will primarily compare titles. Release year and popularity will help rank otherwise similar results but will not overpower a clearly better title.
- Titles containing a year, such as “Blade Runner 2049” or “Wonder Woman 1984,” will be searched both as complete titles and as possible title-plus-release-year filenames.
- The scanner will always enrich a movie from the best available TMDB candidate, even when its confidence score is below the current threshold. No warning, review flag, or UI workflow will be added for low-confidence matches.

## Implementation changes

### Preserve metadata and track retries

- Add an internal `movie_tmdb_retries` table keyed by movie ID with cascading deletion. Update the current schema directly and regenerate sqlc code; do not add a migration or manually edit generated files.
- Include movie ID, stored path, TMDB ID, file size, and retry status in the scanner’s existing-file index.
- Apply these scan paths:
  - New or changed file: run ffprobe, save technical information, and attempt TMDB enrichment.
  - Unchanged movie awaiting TMDB or without an identity: retry only TMDB enrichment, including fingerprint-only updates.
  - Unchanged identified movie without a pending retry: skip it.
- Make at most one enrichment attempt per movie per scan. Report successful enrichments and outstanding pending enrichment in completion logs.
- For movies with a stored TMDB ID, retrieve details directly by that ID. Do not run search-based rematching.
- Treat TMDB unavailability, no search results, search failures, and details failures as retryable metadata outcomes.
- Add a distinct `tmdb.ErrNoMoviesFound` error so “no result” can be handled without fragile error-text checks.
- Clear retry state after successful enrichment or a successful manual Identify operation.
- Keep cancellation quiet and retain the existing shutdown-aware contexts.

### Separate technical and descriptive persistence

- Scanner conflict updates must always save file-derived fields such as file name, size, container, MIME type, valid runtime, streams, and chapters.
- Do not clear existing TMDB descriptive fields or relationships unless confirmed replacement details have been fetched.
- After confirmed TMDB details arrive, update descriptive fields and replace related records transactionally.
- Preserve the audience rating and ffprobe-derived runtime during TMDB refreshes.
- Allow confirmed TMDB refreshes to replace manually edited descriptive metadata, matching the chosen refresh policy.
- Keep external requests outside the scanner database mutex; manual Identify shares that mutex for its short write transaction.
- Before applying asynchronous enrichment, recheck the catalog ID, stored path, and TMDB identity observed when the work began. Deleted or replaced rows discard the in-flight work. If manual Identify changed it, discard the stale TMDB response while retaining valid technical updates.
- Invalidate playback caches and persisted keyframe/remux data after technical rescans, but never after metadata-only retries. Cancellation, unstable files, and transaction failures publish no partial metadata, fingerprints, or caches.

### Correct candidate ranking

- Keep the existing normalized-title matching signals while making title similarity the dominant ranking factor.
- Replace the oversized exact-year bonus with bounded weighting:
  - Exact release year: `+20`
  - One-year difference: `+12`
  - Known year mismatch: `-15`
  - Popularity and vote contribution combined: capped at `13`
- Apply sequel-number bonuses only when the scanned title itself contains a sequel marker.
- Clamp confidence to `0–100`. The existing `70` threshold may remain as a descriptive value, but it must not reject, defer, log, flag, or prevent enrichment.
- For a single non-parenthesized four-digit token that could belong to the title:
  - Search once using the parsed title and year.
  - Search again using the complete title without a year restriction.
  - Continue when either search produces usable candidates.
  - Merge duplicate results by TMDB ID and break score ties deterministically by title then ID.
  - Score each candidate using both filename interpretations and retain its stronger score.
- Always choose and enrich from the highest-ranked usable candidate, including candidates below the confidence threshold.
- Use the corrected ranking in both the automatic scanner and the manual movie picker.

### Accept moving MJPEG video

- Remove MJPEG from the codec-only artwork exclusion. Keep `attached_pic` exclusion for every codec and retain PNG, GIF, and BMP handling.
- Require at least one accepted video stream before committing a movie.
- Preserve absolute stream indices; imported MJPEG follows existing playback/transcoding behavior.
- Add real Jellyfin ffprobe fixtures for moving MJPEG AVI with audio before video, attached MJPEG artwork alongside video, and artwork-only rejection.

## Interfaces and documentation

- Keep HTTP routes, request/response shapes, authentication, status codes, and user-visible movie fields unchanged. Update relevant OpenAPI descriptions and regenerate derived documentation comments.
- Keep retry state entirely internal; do not expose it through the API or UI.
- Update `docs/ffmpeg.md` during implementation to document metadata-only retries and the separation between probing and TMDB enrichment.

## Test plan

- Verify that a failed TMDB refresh preserves all existing scalar metadata and relationships while still saving changed technical information.
- Verify that a later unchanged scan retries TMDB by stored ID without invoking ffprobe, then refreshes metadata and clears retry state.
- Verify that new movies remain usable and queued for retry after TMDB unavailability, no results, search errors, or details errors.
- Verify that manual Identify clears retry state and that stale scanner results cannot overwrite a newly selected TMDB identity.
- Verify that successful enrichment applies every supported TMDB field for low-confidence matches without producing a warning or review flag.
- Verify that a strongly matching title outranks an unrelated popular movie from the same year.
- Cover remakes, one-year release-date differences, sequel titles, parenthesized years, `Blade Runner 2049`, `Wonder Woman 1984`, and filenames containing both a title year and a release year.
- Verify identical corrected ordering in the automatic scanner and manual picker.
- Cover transactional rollback, cancellation, retry cleanup, deleted/replaced catalog rows, and manual Identify during an in-flight scan. Assert retained catalog IDs, descriptions, relationships, audience rating, file-derived runtime, streams/chapters, and playback caches where appropriate.
- Run focused scanner/TMDB/API tests, race-enabled affected tests, `make generate`, `make test-openapi`, and `make check`.
- Validate media fixtures with Jellyfin binaries on Linux x64 and macOS ARM64; record unavailable platform validation.

## Assumptions

- TMDB metadata is applied whenever at least one usable candidate and its details can be retrieved, regardless of confidence.
- When TMDB returns no usable candidate or its details cannot be retrieved, the movie retains local or existing metadata and is retried during later startup or manual scans.
- No continuous background retry worker is introduced.
- Broader library reconciliation, music scanning, new dependencies, background retry workers, forced-refresh UI, and metadata-edit locking remain outside this work.

## Implementation and validation — 2026-09-08

Implemented the complete scope, including moving MJPEG. No music-scanner or shared filesystem lifecycle files were changed.

Passed:

- Focused movie-scanner, TMDB, helper, and API tests, including HTTP Identify during a blocked scanner lookup.
- Race-enabled affected tests, including recovery without a retry row, one attempt per scan, remote timeout fallback, metadata-only chapter/stream preservation, cancellation, rollback, and stale catalog/identity checks.
- Real Jellyfin `7.1.4-Jellyfin` ffprobe fixture tests on Linux x64: moving MJPEG AVI, attached artwork alongside video, and artwork-only rejection. Fixture generation commands are in `server/cmd/internal/scanner/movie/testdata/README.md`.
- `make generate` and `make test-openapi`.
- `make check`: backend vet/dead-code checks and tests, frontend lint, all 746 web tests, type checking, and production build.

Verification setup: `IGLOO_FFMPEG_PATH` and `IGLOO_FFPROBE_PATH` pointed to the repository's Linux x64 Jellyfin payloads. The checkout initially lacked `web/src/routeTree.gen.ts`; it was generated using the installed `@tanstack/router-generator` before frontend verification and removed afterward as a temporary test prerequisite. To recreate it from `web/`:

```sh
bun -e 'import { Generator, getConfig } from "@tanstack/router-generator"; const root = process.cwd(); await new Generator({config: getConfig({}, root), root}).run();'
```

The OpenAPI generated-file comparison in `make check` used a temporary Git index containing this task's intentional generated documentation-comment update. The working staging area was not changed. The ordinary comparison against the original index reports that expected update until it is staged.

One initial full-suite attempt encountered `count rows: interrupted` in the unchanged music cancellation test. It did not recur in 30 runs on the baseline commit or in subsequent full checks; no music code or tests were altered.

macOS ARM64 execution was unavailable in this Linux environment, so that platform's real media-fixture validation remains outstanding. Browser playback and hardware transcoding were not exercised; their implementation was unchanged.
