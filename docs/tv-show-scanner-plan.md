# TV show scanner implementation plan

Implement a local TV catalog alongside movie and music scanning. Only locally represented shows, seasons, and episodes are persisted. Show-directory paths identify shows independently of TMDB identity; physical files own technical metadata and link to one or more episodes.

## Phase 1 — SQL schema and queries

Modify the current schema directly, without migrations. Add shows, seasons, episodes, files, ordered episode/file links, fingerprints, and enrichment retry tables. Enforce ownership, same-season links, uniqueness, numbering constraints, and cascades. Reuse people, companies, genres, and extra videos; add networks and TV relationships, aggregate credits, episode credits, and file-owned streams and chapters. Generate sqlc code and verify relationships, rollback, and pruning.

## Phase 2 — TMDB TV support

Add typed TV search, show details (aggregate credits, ratings, external IDs, videos), season details (episodes, aggregate credits, videos), and episode credits. Use English metadata and prefer US certification. Search optional premiere years with unfiltered fallback; reuse movie ranking and deterministic ties. Fetch stored IDs directly without automatic rematching. Validate identities and numbering, cache successes with TV namespaces, and reuse HTTP cancellation and retries. Cover requests and failures with deterministic HTTP tests.

## Phase 3 — TV scanner

Use the existing dependencies, start-status guard, batching, fingerprints, 60-second quiet period, cancellation, shared database mutex, and reconciliation. Capture shows_dir at start. Require Show/Season N/file or Show/Specials/file. Accept case-insensitive S01E02, S01E02-E04, S01E02E03, and 1x02. Reject conflicting seasons, malformed numbering, and reversed ranges. Exclude hidden entries and nested extras; ignore NFO and external subtitles. Follow movie video-extension and symlink behavior.

Probe each new or changed file once. Atomically persist fallback catalog entries, file metadata, streams, chapters, links, fingerprints, and retry markers. Retain changed file IDs; identical bytes update fingerprints alone. Preserve absolute stream indices and file duration without guessed episode boundaries. Enrich sequentially after technical processing and safe cleanup, grouped by show/season. Commit each entity only after required responses succeed; preserve old metadata and pending work on failures. Recheck ownership and TMDB identity before applying network responses. Unchanged successful entities are skipped and metadata-only retries never probe.

Delete only confirmed missing files within the captured, identity-checked root. Protect observed failures and deferred files. Prune episodes without files, empty seasons, and empty shows transactionally, retaining shared metadata entities. Log file outcomes separately from episode and enrichment counts.

## Phase 4 — Integration and verification

Wire startup and shutdown tracking; add admin POST /api/settings/scan/shows (triggerShowScan), returning 200 started, 409 running, 500 unconfigured, and existing 401/403 authorization errors. Update OpenAPI and generated artifacts. Saving settings does not launch scans.

Validate naming, hidden backups, combined episodes, duplicate copies, offline import and later enrichment, partial failures, stable matches, fingerprints, cancellation, rollback, stale responses, concurrent writes, safe cleanup, real Jellyfin ffprobe fixtures, startup, authorization, and route coverage. Run focused and race tests, make generate, make generate-openapi, make test-openapi, and make check. Run a read-only sample-library scan with an isolated database if available; report macOS ARM64 validation availability.

## Scope and defaults

Use standard TMDB numbering; defer alternate/DVD ordering. Keep TMDB totals separate from local counts derived from file links. Store remote artwork and trailer metadata, without downloads or thumbnails. No TV browsing, playback, watch progress, web controls, NFO ingestion, manual identification, filesystem watcher, or background metadata refresh. Refresh successful metadata on file changes and retry failures on later scans.

The planning inspection reported 316 videos across four shows, including 29 combined-episode files and a hidden backup directory. Recheck sample-library availability during verification.

## Implementation and validation notes

All four implementation phases are complete. The scanner is wired to startup and the admin endpoint, and the API contract and generated artifacts are updated. Database tests enforce same-season ownership independently of uniqueness checks. Filesystem, transaction, enrichment, HTTP, endpoint, and real-probe tests cover the implemented behavior.

TV endpoint shapes and search parameters were checked against the [TMDB TV search reference](https://developer.themoviedb.org/reference/search-tv), [show details](https://developer.themoviedb.org/reference/tv-series-details), [season details](https://developer.themoviedb.org/reference/tv-season-details), [episode credits](https://developer.themoviedb.org/reference/tv-episode-credits), and [aggregate credits](https://developer.themoviedb.org/reference/tv-series-aggregate-credits).

The opt-in `TestSampleLibraryReadOnly` uses `IGLOO_TV_SAMPLE_DIR`, an isolated in-memory database, real ffprobe, and no TMDB credentials. It also verifies that a second unchanged scan never probes. The original 316-file library was not available in this checkout: no configured shows directory or sample-library mount was found. This validation remains pending the sample path. Native macOS ARM64 execution is unavailable on this Linux x64 host.

Validation passed on Linux x64: focused tests; race-enabled scanner, TMDB, and API tests; `make generate`; `make generate-openapi`; `make test-openapi`; and `make check` (including 83 web test files / 746 tests and the production web build). Jellyfin `7.1.4-Jellyfin` binaries were selected through the existing binary-path overrides for the full checks and real-media fixtures. OpenAPI regeneration was checked against a temporary Git index containing the intended generated artifact, leaving the workspace index unchanged.

Additional review tightened malformed-token rejection for separated episode tokens and strengthened the ownership test to avoid uniqueness masking a foreign-key defect. Focused scanner race tests passed again after these changes. Live credentialed TMDB requests and native macOS ARM64 execution were not run.
