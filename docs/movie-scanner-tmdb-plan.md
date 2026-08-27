# Movie Scanner TMDB Reliability and Matching Plan

## Summary

This plan addresses two issues identified in the movie scanner review:

1. Prevent failed TMDB lookups from erasing existing movie metadata, and retry those lookups during future scans.
2. Improve TMDB matching so title similarity outweighs release-year and popularity signals.

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
- Include movie ID, TMDB ID, file size, and retry status in the scanner’s existing-file index.
- Apply these scan paths:
  - New or changed file: run ffprobe, save technical information, and attempt TMDB enrichment.
  - Unchanged movie awaiting TMDB: retry only TMDB enrichment.
  - Unchanged movie without a pending retry: skip it.
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
- Before applying asynchronous enrichment, verify that the movie still has the TMDB identity observed when the work began. If manual Identify changed it, discard the stale TMDB response while retaining valid technical updates.
- Invalidate playback caches after technical rescans, but not after metadata-only retries.

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
  - Merge duplicate results by TMDB ID.
  - Score each candidate using both filename interpretations and retain its stronger score.
- Always choose and enrich from the highest-ranked usable candidate, including candidates below the confidence threshold.
- Use the corrected ranking in both the automatic scanner and the manual movie picker.

## Interfaces and documentation

- Do not change public HTTP APIs, OpenAPI types, frontend behavior, or user-visible movie fields.
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
- Cover transactional rollback and cancellation behavior.
- Run focused scanner, repository, and TMDB tests, followed by `make check`.

## Assumptions

- TMDB metadata is applied whenever at least one usable candidate and its details can be retrieved, regardless of confidence.
- When TMDB returns no usable candidate or its details cannot be retrieved, the movie retains local or existing metadata and is retried during later startup or manual scans.
- No continuous background retry worker is introduced.
- MJPEG handling and broader library reconciliation remain outside this work.
