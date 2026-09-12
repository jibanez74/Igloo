# TV show scanner specification and validation

Igloo maintains a local TV catalog alongside movie and music scanning. Only locally represented shows, seasons, and episodes are persisted. Show-directory paths identify shows independently of TMDB identity; physical files own technical metadata and link to one or more episodes.

The requirements below describe the completed implementation. Validation is tracked separately in the dated record below; implementation completion does not imply that every platform has been exercised. [Media behavior](ffmpeg.md#tv-show-scanning) and the [API contract](openapi.json) remain authoritative.

## Phase 1 — SQL schema and queries (implemented)

Modify the current schema directly, without migrations. Add shows, seasons, episodes, files, ordered episode/file links, fingerprints, and enrichment retry tables. Enforce ownership, same-season links, uniqueness, numbering constraints, and cascades. Reuse people, companies, genres, and extra videos; add networks and TV relationships, aggregate credits, episode guest and crew credits, and file-owned streams and chapters. Generate sqlc code and verify relationships, rollback, and pruning.

## Phase 2 — TMDB TV support (implemented)

Add typed TV search, show details (aggregate credits, ratings, external IDs, videos), and season details (episodes with their guest stars and crew, aggregate credits, videos); episode credits are read from the season payload rather than requested per episode. Use English metadata and prefer US certification. Search optional premiere years with unfiltered fallback; reuse movie ranking and deterministic ties. Fetch stored IDs directly without automatic rematching. Validate identities and numbering, cache successes with TV namespaces, and reuse HTTP cancellation and retries. Cover requests and failures with deterministic HTTP tests.

## Phase 3 — TV scanner (implemented)

Use the existing dependencies, start-status guard, worker pool, metadata-only fingerprints, 60-second quiet period, cancellation, shared database mutex, and reconciliation. Capture shows_dir at start. Accept Show/Season N/file, Show/Season N - Title/file, Show/SN/file, Show/Specials/file, and episode files directly under the show folder. Accept case-insensitive S01E02, S01E02-E04, S01E02E03, and 1x02. Reject conflicting seasons, malformed numbering, and reversed ranges. Exclude hidden entries and nested extras; ignore NFO and external subtitles. Follow movie video-extension and symlink behavior.

Probe each new or changed file once. Atomically persist fallback catalog entries, file metadata, streams, chapters, links, fingerprints, and retry markers. Retain changed file IDs; inspection is metadata-only, so a touched file is re-probed rather than hashed. Preserve absolute stream indices and file duration without guessed episode boundaries. Enrich sequentially after technical processing and safe cleanup, grouped by show/season, with the entity total published first, a 30-second budget per lookup, and a provider circuit breaker. Commit each entity only after required responses succeed; preserve old metadata and pending work on failures. Report definitive no-matches as unmatched and back them off like movies. Recheck ownership and TMDB identity before applying network responses. Unchanged successful entities are skipped and metadata-only retries never probe.

Delete only confirmed missing files within the captured, identity-checked root. Protect observed failures and deferred files. Prune the touched season's episodes without files, then the empty season and show, transactionally, retaining shared metadata entities. Log file outcomes separately from episode and enrichment counts.

## Phase 4 — Integration (implemented) and verification

Wire startup and shutdown tracking; add admin POST /api/settings/scan/shows (triggerShowScan), returning 200 started, 409 running, 500 unconfigured, and existing 401/403 authorization errors. Add admin GET /api/settings/scan/shows (getShowScanStatus) reporting the current or latest in-memory run. Update OpenAPI and generated artifacts. Saving settings does not launch scans.

Validate naming, hidden backups, combined episodes, duplicate copies, offline import and later enrichment, partial failures, stable matches, fingerprints, cancellation, rollback, stale responses, concurrent writes, safe cleanup, real Jellyfin ffprobe fixtures, startup, authorization, and route coverage. Run focused and race tests, make generate, make generate-openapi, make test-openapi, and make check. Run a read-only sample-library scan with an isolated database if available; report macOS ARM64 validation availability.

## Scope and defaults

Use standard TMDB numbering; defer alternate/DVD ordering. Keep TMDB totals separate from local counts derived from file links. Store remote artwork and trailer metadata, without downloads or thumbnails. No TV browsing, playback, watch progress, web controls, NFO ingestion, manual identification, filesystem watcher, or background metadata refresh. A technical file change never re-queues a matched entity; only entities without a TMDB identity are queued. Failures retry on later scans.

## Reintegration — 2026-09-11

This work was developed on `feature/tv-shows-scanner`, whose branch ref was deleted before it was pushed or merged. The four commits were recovered from the reflog and rebuilt on `dev`, which had meanwhile refactored the shared scanner core and removed unused SQL columns. The scanner's behavior is unchanged; its plumbing is not:

- The package-local `show.StartStatus`/`StartResult` and hand-rolled `ScanGuard` were replaced by the shared `scanner.Launcher` and `scanner.StartResult`, matching movies and music.
- Progress reporting (`scanner.Progress`/`Report`) did not exist when the scanner was written. It now publishes a run, and `GET /api/settings/scan/shows` serves it.
- The three hand-written transactions now use the shared `scanner.TxRunner`, and cleanup uses `Reconciliation.DeleteUnseen`/`DeleteConfirmed`.
- Tests adopted `scanner/scannertest` doubles.
- `production_companies.logo`/`country` and `extra_videos.official` no longer exist on `dev`, so those TMDB fields are not stored. Chapter start times lost their raw-ticks fallback; the shared `scanner.ChapterStartTimeSeconds` is now the single implementation.

The validation record below was produced against the pre-refactor code and has **not** been re-run. `make check`, the race suite over `./cmd/internal/scanner/... ./cmd/internal/tmdb/... ./cmd/api`, and the mocked `libraries-settings` Playwright spec all pass after reintegration; the 415 GB sample-library run and the live TMDB integration run have not been repeated.

## Optimization — 2026-09-12

Content hashing was removed (inspection is metadata-only, matching movies), the local phase moved onto the shared two-worker pool with discovery completed first, pruning became season-scoped, episode credits are read from the season payload, enrichment publishes its total up front and gained the movie scanner's lookup timeout, provider circuit breaker, unmatched outcome, and miss backoff, and the naming rules above were widened. The validation record below predates these changes: its hashing statements, four-hour timeout, and episode-credits identity assertions no longer describe the scanner.

### Validation record — 2026-09-12 (post-optimization)

- Commit `e65b8fe9` on `fix/tv-shows-scanner`, Linux x64, Go from `server/go.mod`, repository Jellyfin ffprobe/ffmpeg payloads selected with `IGLOO_FFMPEG_PATH`/`IGLOO_FFPROBE_PATH`, sample at `/home/jose-ibanez/samba/tvshows` (read-only CIFS). The sample now holds three shows; `Alien Earth (2025)` is no longer present, so the independent inventory expects 313 files, 16 seasons, and 345 logical episodes.
- `python3 scripts/tv-sample-inventory.py` → `TestSampleLibraryReadOnly` → reconciliation, all exit zero.

| Scan outcome | Initial scan | Unchanged second scan |
| --- | ---: | ---: |
| Wall time | 36.5 s (was 5,713 s with hashing) | 0.47 s |
| Real probes | 313 | 0 additional |
| Imported / unchanged files | 313 / 0 | 0 / 313 |
| Deferred / rejected / deleted files | 0 / 0 / 0 | 0 / 0 / 0 |
| Local episodes processed | 345 | 345 |
| Enriched / pending entities | 0 / 364 | 0 / 364 |

All show catalog table snapshots were identical after the rescan. `make test-tmdb-integration` passed with the season payload supplying episode guest stars and crew. Native macOS ARM64 remains deferred.

## Implementation status

The schema, typed TMDB TV client, scanner, startup/shutdown wiring, and admin endpoints are implemented. The API contract and generated types are present. This validation task changes tests, documentation, and an independent inventory utility; no demonstrated production defect has required a scanner, schema, API, or media-contract change. SQL generation is therefore not required for this task. `make check` verifies OpenAPI route coverage and generated-type currency.

Deterministic tests retain coverage for malformed and conflicting numbering, combined episodes and duplicate copies, hidden backups, quiet-period deferral, fingerprints, offline import and enrichment retries, cancellation, rollback, stable identities, stale responses, concurrent writes, cleanup, startup, and authorization. Real Jellyfin ffprobe fixtures cover technical metadata, absolute stream indices, moving MJPEG, attached artwork, and artwork-only rejection.

`TestSampleLibraryReadOnly` is opt-in through `IGLOO_TV_SAMPLE_DIR`. It uses the real quiet period, an isolated in-memory SQLite database, real ffprobe, and a nil TMDB client even when credentials exist in the environment. It logs scanner outcomes, every real probe, catalog paths with ordered episode numbers, and all TV table counts. It snapshots every column in the show catalog tables (`shows` and `show_*`), including IDs, ownership, ordered links, technical streams and chapters, fingerprints, timestamps, relationships, and retry markers. An unchanged second scan must add zero real probes and leave every snapshot unchanged. Count-only or single-file success is insufficient: pair this test with the independent inventory reconciliation below.

`TestTVMetadataIntegration` lives under the existing `integration` build tag and reuses `loadIntegrationEnv` and `make test-tmdb-integration`. Each request has a 30-second context. Fresh clients exercise title-only search, premiere-year search (2008), and unfiltered fallback from a deliberately unmatched year (1850). Breaking Bad's ID is resolved from search; show details and season 1 details validate required structures and identities, including episode 1's guest stars and crew from the season payload. Ratings, popularity, exact credit counts, and exact total episode/season counts are not fixed assertions. Missing credentials remain a skip, not evidence of live validation.

## Validation record — 2026-09-09 (pre-reintegration)

- Validation ran on base commit `ad9def5c7bafe5c62f0207a15eb8d1353704e2e0`, before the shared-scanner-core reintegration described above, with the test, documentation, and inventory changes included in that change.
- Host: native Linux x64 (`linux/amd64`), kernel `7.0.0-31-generic`.
- Tools: Go `1.26.7`, GCC `13.3.0`, Bun `1.4.2`, Python `3.12.3`; OpenAPI generation reported `openapi-typescript 7.13.0`.
- Media tools: both repository Linux x64 payloads report `7.1.4-Jellyfin`, selected with the existing `IGLOO_FFMPEG_PATH` and `IGLOO_FFPROBE_PATH` overrides. Generic FFmpeg installed by CI is not Jellyfin-specific validation evidence. No CI changes are included.
- Sample: `/home/jose-ibanez/samba/tvshows`, a read-only CIFS mount; accepted files total `414,611,529,722` bytes (about 415 GB). Initial hashing reads all accepted bytes, so the sample test uses a four-hour timeout. Logs and inventory output are written outside the media root.

| Check | Command (from repository root unless noted) | Result |
| --- | --- | --- |
| Independent inventory | `python3 scripts/tv-sample-inventory.py "$IGLOO_TV_SAMPLE_DIR" > /tmp/igloo-tv-inventory.json` | Passed enumeration; counts independently reconciled below. |
| Small real-media fixture sample | Sample command below with `-race` and a temporary fixture root | Passed: three imports/probes, one malformed-season rejection, hidden backup excluded, duplicate/combined links retained, zero additional rescan probes, all TV snapshots unchanged. This verifies the test itself, not full-library acceptance. |
| Full sample and unchanged rescan | Sample command below | Passed in 5,713.28 s (95 min 13 s), completed by 17:54 UTC. All 316 files imported with one probe each; second scan added zero probes and changed no catalog rows/columns. Independent reconciliation exited zero with no discrepancies. |
| Live TMDB, including existing movie checks | `make test-tmdb-integration` | Passed; TV's six subtests passed, full package in 1.671 s. Search ID `1396`, season ID `3572`, episode and credits ID `62085`. |
| Missing-key behavior | From `server`: `TMDB_API_KEY= CGO_ENABLED=1 go test -count=1 -v -tags 'externalbin sqlite_fts5 integration' ./cmd/internal/tmdb -run '^TestTVMetadataIntegration$'` | Explicit `TMDB_API_KEY not set` skip; no live request. |
| Focused scanner and TMDB tests | From `server`: `CGO_ENABLED=1 go test -count=1 -tags 'externalbin sqlite_fts5' ./cmd/internal/scanner/... ./cmd/internal/tmdb/...` | Passed all five packages, including real-probe fixtures. |
| Race-enabled scanners, TMDB, and API | From `server`: `CGO_ENABLED=1 go test -count=1 -race -tags 'externalbin sqlite_fts5' ./cmd/internal/scanner/... ./cmd/internal/tmdb/... ./cmd/api` | Passed all six packages, including TV API authorization and lifecycle coverage. |
| Required repository checks | `make check` | Passed OpenAPI lint/route coverage/generated types, Go vet/deadcode/tests, web lint, 83 web test files / 746 tests, and production web build. Sample test skips here unless explicitly opted in. |
| Inventory script checks | Temporary filesystem with combined, duplicate, hidden, sidecar, and invalid-numbering entries; complete/missing/unfinished scan logs | Passed expected counts and reconciliation failure detection. |
| Native macOS ARM64 | Commands below | **Deferred** by agreement; never executed or marked passed. |

The earlier undated notes reported generation and Linux test runs but did not record their commit, date, or complete commands. They are historical context, not substitutes for this dated validation. This checkout now has sample access and working TMDB credentials, superseding the earlier unavailable-access notes.

### Independent sample inventory and reconciliation

`scripts/tv-sample-inventory.py` uses Python directory enumeration and its own interpretation of this sample's separated `SxxEyy[-Ezz]` filenames. It does not call the scanner, import its parser, hash media, or probe files. It reports every accepted relative path and ordered episode list, hidden files, sidecars, unsupported/special entries, and overlapping logical episode copies. Unrecognized naming requires manual review; this script is a sample validator, not an alternate implementation of every supported filename form.

| Show directory | Files | Seasons | Logical episodes | Episode/file links | Combined files |
| --- | ---: | ---: | ---: | ---: | ---: |
| Alien Earth (2025) | 3 | 1 | 3 | 3 | 0 |
| Friends (1994) | 234 | 10 | 236 | 236 | 2 |
| Pinky and the Brain (1995) | 65 | 4 | 95 | 95 | 27 |
| The Mandalorian (2019) | 14 | 2 | 14 | 14 | 0 |
| **Verified total** | **316** | **17** | **348** | **348** | **29** |

The historical 316 videos, four shows, and 29 combined files are independently confirmed. The 29 combined files contribute 32 extra episode links: 26 two-episode files and three three-episode files. No visible files overlap the same show/season/episode coordinates, so this sample has zero duplicate episode copies; deterministic `TestLocalFilesCopiesFingerprintsAndCleanup` supplies that coverage. This is a logical-identity inventory, not a byte-duplicate claim.

The hidden `.pre-sonarr-tv-import-20260714-162801` directory contains 313 videos and eight other files. Four visible NFO sidecars are also excluded. Thus the complete tree has 629 videos, of which 316 are eligible; adding the 12 non-video files gives 641 files overall. Hidden backup contents are excluded before parsing/probing, not counted as scanner rejections. There are zero visible malformed/unrecognized videos, nested extras, or special entries requiring rejection in this inventory. The scanner matched every accepted path and ordered episode list, with no missing paths, unexpected paths, or episode mismatches. Re-enumeration after the scan matched the initial inventory exactly. Every accepted path was probed exactly once.

| Scan outcome | Initial scan | Unchanged second scan |
| --- | ---: | ---: |
| Real probes | 316 | 0 additional |
| Imported files | 316 | 0 |
| Updated files | 0 | 0 |
| Skipped files | 0 | 316 |
| Deferred / rejected / deleted files | 0 / 0 / 0 | 0 / 0 / 0 |
| Local episodes processed | 348 | 348 |
| Enriched / pending entities | 0 / 369 | 0 / 369 |

All 26 show catalog table snapshots were identical after the rescan, including 316 fingerprints, 316 video streams, 384 audio streams, 3,168 subtitle rows, and 993 chapters. The pending entities are expected with enrichment disabled: four shows, 17 seasons, and 348 episodes. There are no unexplained discrepancies against the independent inventory or the historical reference figures.

Execution artifacts on this host are `/tmp/igloo-tv-inventory.json`, `/tmp/igloo-tv-sample.log`, and `/tmp/igloo-tv-reconciliation.json`; the counts and outcomes above are the durable record. The sample-library and live-TMDB gaps are closed on Linux x64. Native macOS ARM64 execution remains the sole deferred validation item in this plan.

### Linux reproduction

Install the Go version required by `server/go.mod` (or a compatible newer release), a C compiler for CGO, Bun with the locked web dependencies (`cd web && bun install --frozen-lockfile`), and Python 3 for the independent inventory. Use executable Jellyfin FFmpeg/ffprobe from the same release. The Makefile also checks for both names on `PATH`, even when overrides are supplied.

From the repository root, verify binaries and record the environment:

```bash
export IGLOO_FFMPEG_PATH="$PWD/server/cmd/internal/ffmpeg/ffmpeg_linux_amd64"
export IGLOO_FFPROBE_PATH="$PWD/server/cmd/internal/ffprobe/ffprobe_linux_amd64"
export IGLOO_TV_SAMPLE_DIR=/home/jose-ibanez/samba/tvshows
"$IGLOO_FFMPEG_PATH" -version
"$IGLOO_FFPROBE_PATH" -version
date -u
git rev-parse HEAD
git status --short
uname -sm
go version
cc --version
bun --version
python3 --version
python3 scripts/tv-sample-inventory.py "$IGLOO_TV_SAMPLE_DIR" > /tmp/igloo-tv-inventory.json
(
  cd server
  CGO_ENABLED=1 go test -count=1 -v -timeout 4h \
    -tags 'externalbin sqlite_fts5' ./cmd/internal/scanner/show \
    -run '^TestSampleLibraryReadOnly$' > /tmp/igloo-tv-sample.log 2>&1
)
python3 scripts/tv-sample-inventory.py "$IGLOO_TV_SAMPLE_DIR" \
  --scan-log /tmp/igloo-tv-sample.log > /tmp/igloo-tv-reconciliation.json
```

Require the sample command and reconciliation command both to exit zero. The reconciler checks every accepted path and ordered episode list, fails for missing/unexpected paths or changed episode links, and requires a passing test log. Inspect exclusions and the scan's imported/updated/skipped/deferred/rejected/deleted counts; explain every deviation. With this inventory, expect 316 first-scan probes/imports, zero rejected/deferred/deleted files, 348 local episodes processed, zero enrichment, and 369 pending entities (four shows + 17 seasons + 348 episodes). The second scan should skip all 316 files, add zero probes, and preserve every TV table.

Run the focused/race commands from the validation table and `make check` with the Jellyfin overrides retained but `IGLOO_TV_SAMPLE_DIR` unset to avoid repeating the full network read. Provide `TMDB_API_KEY` through the existing environment or repository `.env`, then run `make test-tmdb-integration`; never print the key. The helper preserves pre-existing environment variables. A missing-key skip leaves live validation pending on that host.

### Native macOS ARM64 — execution deferred

Execution is **deferred**, as requested. Cross-compilation or Linux success cannot mark this platform passed.

Use a native Apple Silicon macOS shell with `uname -m` reporting `arm64`, Go reporting `darwin/arm64`, and Xcode Command Line Tools (`xcode-select -p`, `clang --version`) for CGO. Install Bun, locked web dependencies, and Python 3. Obtain the matching native Jellyfin FFmpeg and ffprobe pair described in [the binary strategy](ffmpeg.md#binary-strategy); generic Homebrew FFmpeg does not satisfy this validation prerequisite. Set absolute paths to the binaries and a readable, quiet TV sample root. No write permission on the media library is required.

```bash
export IGLOO_FFMPEG_PATH=/absolute/path/to/jellyfin/ffmpeg
export IGLOO_FFPROBE_PATH=/absolute/path/to/jellyfin/ffprobe
export PATH="$(dirname "$IGLOO_FFMPEG_PATH"):$(dirname "$IGLOO_FFPROBE_PATH"):$PATH"
export IGLOO_TV_SAMPLE_DIR=/Volumes/Media/tvshows
uname -sm
go env GOOS GOARCH
xcode-select -p
clang --version
file "$IGLOO_FFMPEG_PATH" "$IGLOO_FFPROBE_PATH"
"$IGLOO_FFMPEG_PATH" -version
"$IGLOO_FFPROBE_PATH" -version
```

Require native ARM64 executables (or universal binaries containing ARM64) and matching Jellyfin version banners; reject an x86_64-only/Rosetta run as native ARM64 evidence. Then run the inventory, sample, reconciliation, focused, race, live-TMDB, and `make check` commands above with these macOS paths. Preserve required `externalbin sqlite_fts5` tags and `CGO_ENABLED=1`. If using the same sample inventory, expect the same counts and zero additional second-scan probes; another sample needs its own independently reconciled expectations. Real-probe fixtures must pass, and neither sample nor credential skips count as execution success.

When this deferred work is performed, append a separate dated record with commit plus working-tree state, macOS version (`sw_vers`), architecture, compiler/Go/Bun/Python versions, Jellyfin binary paths/versions, sample inventory, commands, exit statuses, and results. Playback, transcoding, and hardware acceleration are outside this scanner validation; these commands make no claim about those paths.
