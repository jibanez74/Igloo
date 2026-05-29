# Music Scanner Optimization Work Plan

## Summary

This plan improves the music scanner in two areas:

- Stop or greatly reduce SQLite lock errors during music scans, especially messages that appear as `"database is locked"` or similar database log output.
- Make scans faster without adding scanner worker goroutines, background worker pools, or parallel scan routines.

The core strategy is to keep slow external work out of SQLite write transactions. The scanner should probe files, resolve Spotify metadata, and build write inputs first, then open short database transactions only when it is ready to write. The implementation should also avoid repeated Spotify lookups and repeated database writes for data that is already known.

## Safety Checkpoint And Rollback

Before any scanner implementation work starts, create a git checkpoint. The current worktree already contains changes outside this scanner plan, including `server/cmd/api/settings_handler.go`. Per the selected strategy, the checkpoint commit should include the current dirty tree so rollback returns to the exact current state.

Run these commands before making scanner code changes:

```bash
git status --short
git add -A
git commit -m "checkpoint: before music scanner optimization"
git rev-parse HEAD
```

Record the checkpoint SHA in the work notes before continuing.

Preferred rollback path:

```bash
git revert <implementation-commit-sha>
```

If the implementation is split across multiple commits, revert them from newest to oldest.

Alternative rollback path when working on a branch:

```bash
git switch <baseline-branch>
```

Last-resort local rollback:

```bash
git reset --hard <checkpoint-sha>
```

Use `git reset --hard` only with explicit user approval because it discards uncommitted work. The safer default is to commit implementation steps and revert the bad commits.

Rollback if any of these happen:

- The scanner fails to complete on a representative music library.
- Tracks, albums, musicians, or genres are missing after a scan.
- Duplicate albums, musicians, or relationships are created.
- The database lock message still appears during normal scan usage.
- The benchmarked scan is not at least 15% faster in the agreed scenario.
- Spotify metadata behavior regresses in a way that makes common albums or artists worse than before.

## Baseline Measurements

Before changing code, capture a baseline scan. This gives the rollback decision something concrete to compare against.

Use a representative music library with Spotify credentials configured if that is how the scanner is normally used. Capture both a changed-file scan and an unchanged rescan if possible.

Record:

- Git checkpoint SHA.
- Music directory path.
- Track count in the directory.
- Existing database track count before the scan.
- Whether Spotify is configured.
- Whether the server is in debug mode or normal logging mode.
- Start timestamp.
- End timestamp.
- Scanner summary: scanned, skipped, errors, duration.
- Any log lines containing `database`, `locked`, `data base`, `spotify`, `ffprobe`, or `music scanner`.

Suggested commands:

```bash
git rev-parse HEAD
grep -iE "database|locked|data base|spotify|ffprobe|music scanner" logs/igloo.log
```

If logs are written to stdout instead of `logs/igloo.log`, capture the server process output instead.

Acceptance target:

- Database lock messages disappear during the same scenario.
- Scan duration improves by at least 15%.
- Scanner summary has no new errors.
- Music library counts remain correct.

## Current Workflow Problems

The current music scanner batches up to `helpers.SCANNER_BATCH_SIZE` files and opens one transaction for the entire batch. For each changed file, it runs `ffprobe`, Spotify lookups, album/musician writes, genre writes, and track writes inside that transaction.

This creates two risks:

- A batch transaction can hold a SQLite write lock while slow external work is still running.
- Normal app requests also use SQLite, including the SQLite-backed session store, so they can collide with the scanner write lock.

The scanner also repeats work that can be cached or skipped:

- It checks each file against the database one at a time.
- It may call Spotify repeatedly for the same artists, albums, or known misses.
- It clears Spotify in-memory caches at the end of every scan.
- It updates existing Spotify-backed rows even when stored values are unchanged.
- It runs full ffprobe metadata extraction for music, including data that is more useful for movies than audio tracks.

## Implementation Phases

### Phase 1: Fix Scan Trigger Semantics

Goal: the API should not report that a scan started if a scan is already running.

Implementation:

- Move the `tryBeginMusicScan` call out of `MusicScanLibrary` when the scan is started from the HTTP handler, or add a new helper that lets the handler reserve the scan before launching the goroutine.
- Make `TriggerMusicScan` return an error response when a music scan is already active.
- Preserve startup scan behavior: startup should still attempt a scan and log if one is already running.
- Keep the route and response shape stable for successful requests.

Expected behavior:

- First manual scan request returns success.
- Second manual scan request during the active scan returns an already-in-progress error.
- The scanner does not start two concurrent music scans.

### Phase 2: Add Read-Only Scan Index

Goal: avoid one database query per file just to decide whether it is unchanged.

Implementation:

- Add a sqlc query that returns the music scan index from `tracks`, at minimum `file_path` and `size`.
- Load this index once at the start of `runMusicScan`.
- Store it in a `map[string]int64` keyed by clean file path.
- During `WalkDir`, skip unchanged files by checking the map instead of calling `CheckTrackExistsByPathAndSize` for every file.
- Keep path handling consistent with existing scanner behavior. Do not introduce path normalization that changes existing database identity unless that is a separate explicit task.

Important detail:

- The existing query can remain for other callers or tests, but the scanner hot path should use the loaded index.

Expected result:

- Unchanged rescans become faster because they avoid thousands of small SQLite reads.
- The scanner does less work before it knows a file changed.

### Phase 3: Separate Resolution From Writes

Goal: never hold a SQLite write transaction while running ffprobe or Spotify HTTP requests.

Implementation:

- Introduce a resolved track input type in the scanner layer. It should contain all values needed for database writes after external work has completed.
- Split `processTrackFile` into two conceptual steps:
  - Resolve: ffprobe file metadata, derive track params, determine artist/album lookup inputs, call Spotify when needed, and build a write plan.
  - Persist: open a short transaction, upsert musicians, albums, genres, relationships, and track rows.
- Do not call `app.Ffprobe.GetMetadata`, `app.Spotify.SearchArtistByName`, or `app.Spotify.SearchAndGetAlbumDetails` while a database transaction is open.
- Keep savepoints or per-track transaction boundaries so one bad track does not roll back unrelated tracks.
- Prefer per-track short transactions first. If later measurement shows commit overhead is significant, batch only the write phase, not external resolution.

Expected result:

- SQLite write locks are held for milliseconds instead of during ffprobe and network calls.
- Authenticated UI/API requests are less likely to fail while scans run.

### Phase 4: Add Scan-Local Caches

Goal: avoid repeated work within a single scan without adding workers.

Implementation:

- Add a scan context struct that is passed through the scanner workflow.
- Include maps for:
  - Musician name plus sort name to musician ID.
  - Album title plus album artist to album ID.
  - Genre tag to genre ID.
  - Musician-album, musician-genre, album-genre, and track-genre relationship pairs already handled during the scan.
  - Spotify artist lookup input to miss or failure for the current scan.
  - Spotify album lookup input to miss or failure for the current scan.
- Use these caches before re-querying/upserting the same local entities or repeating relationship writes.
- Do not add a second scanner-owned cache for successful Spotify artist or album API results. The Spotify package already owns successful in-memory artist and album result caching with TTL eviction.
- Cache Spotify misses and failures for the duration of the scan so no-result, below-threshold, or transient failure cases are not retried repeatedly for tracks with the same metadata.
- Keep cache keys normalized with trimmed lowercase values. Do not over-normalize in a way that merges distinct artists or albums.

Expected result:

- Albums with many tracks do not repeatedly resolve the same album.
- Artists with many tracks do not repeatedly resolve the same artist.
- Known Spotify misses do not repeatedly call Spotify during the same scan.
- Successful Spotify artist and album results continue to be served by the existing Spotify client cache.

### Phase 5: Persist Spotify Match Results

Goal: use the existing `music_spotify_matches` table to avoid repeated Spotify calls across scans and provide explainable metadata results.

Implementation:

- Add sqlc queries for `music_spotify_matches`:
  - Get match by entity type and entity ID.
  - Upsert matched result.
  - Upsert unmatched result.
  - Upsert failed result.
  - Optionally clear match for an entity when a user explicitly re-identifies metadata in a future feature.
- When an album or musician row already has a usable persisted match, skip Spotify search and use the stored Spotify ID if possible.
- When a search returns no result or score below threshold, persist an `unmatched` status with reason, score, threshold, candidate, query, and strategy.
- When Spotify fails due to network or API error, persist `failed` with error details, but consider retrying failed rows on a later scan.
- Do not permanently suppress retries for transient failures unless a retry policy is added.

Important detail:

- The table is keyed by entity type and entity ID, so the entity must exist before the final match record can be saved.
- For pre-insert lookup decisions, scan-local miss and failure caches still matter because the entity ID might not exist yet.
- Persisted match rows solve cross-scan matching decisions and explainability; they are not a replacement for the Spotify package's short-lived successful API result cache.

Expected result:

- Repeated scans do not keep searching Spotify for the same unmatched local metadata.
- Troubleshooting Spotify matching becomes easier because failed and unmatched reasons are stored.

### Phase 6: Avoid No-Op Database Writes

Goal: reduce write volume and avoid unnecessary FTS trigger updates.

Implementation:

- Before updating an existing musician thumb, compare the stored `thumb` with the Spotify image URL.
- Before updating an existing album cover, compare the stored `cover` with the Spotify image URL.
- Avoid reprocessing Spotify genres for an existing entity if the exact relationship set is already present, or cache that the entity’s Spotify genres were processed once during this scan.
- Keep `ON CONFLICT DO NOTHING` relationship inserts for safety, but do not call them repeatedly for the same entity pair in the same scan.
- Reuse scan-local relationship caches for musician-album, musician-genre, album-genre, and track-genre writes.

Expected result:

- Fewer writes to `albums`, `musicians`, relationship tables, and FTS tables.
- Faster scans and less SQLite lock pressure.

### Phase 7: Revisit Spotify Runtime Cache Clearing

Goal: keep useful metadata caches alive when safe.

Implementation:

- Remove `app.Spotify.ClearAllCaches()` at the end of every music scan unless there is a concrete correctness reason to clear it.
- Keep TTL-based eviction in the Spotify package.
- If cache invalidation is needed after settings changes, clear or replace the Spotify client when Spotify credentials change, not after every scan.

Expected result:

- Back-to-back scans can reuse recent Spotify results.
- Metadata lookup latency drops for repeated scans.

### Phase 8: Add Audio-Specific ffprobe Metadata

Goal: reduce ffprobe work per audio file.

Implementation:

- Add a music-specific method to the ffprobe interface, for example `GetAudioMetadata(filePath string)`.
- Use ffprobe arguments that only collect fields needed by the music scanner:
  - Format tags.
  - Format duration, bit rate, size, and format name if needed.
  - Stream fields for audio codec, profile, channels, channel layout, and language.
- Do not request chapters for music scanning.
- Keep the existing `GetMetadata` method unchanged for movies and playback-sensitive code.
- Update the music scanner to call the audio-specific method.

Expected result:

- Less JSON output from ffprobe.
- Lower process time per scanned track.
- No behavior change for movie scanning.

## Data Flow After Refactor

Target flow:

1. Start scan and reserve scan lock.
2. Load track scan index from SQLite.
3. Walk music directory.
4. Skip files that match path and size in the scan index.
5. For changed files, run audio-specific ffprobe without a database transaction.
6. Resolve Spotify artist and album metadata using persisted match rows, scan-local miss/failure caches, and the Spotify package's successful-result cache.
7. Build a resolved track write plan.
8. Open a short transaction.
9. Upsert musician, album, genre, relationships, Spotify match rows, and track.
10. Commit.
11. Continue to next file.
12. Log final scanned, skipped, and error counts.

The critical rule is that steps 5 and 6 must not run inside a SQLite write transaction.

## Acceptance Criteria

Functional:

- Music scan still indexes supported `mp3`, `flac`, and `m4a` files.
- Existing unchanged tracks are skipped correctly.
- Changed tracks update correctly when file size changes.
- Track metadata fields remain populated at least as well as before.
- Album, musician, track genre, album genre, and musician genre relationships still work.
- Compound artist behavior remains unchanged unless explicitly tested and improved.
- Manual scan API rejects concurrent scan attempts instead of reporting a false start.

Performance:

- Representative scan is at least 15% faster than baseline.
- Unchanged rescan is meaningfully faster due to scan-index loading.
- Logs do not show SQLite lock failures during normal scan usage.

Safety:

- Movie scanner behavior is unchanged.
- HLS/playback code paths are unchanged.
- Spotify credentials changes still require restart unless a separate runtime reinitialization task is implemented.

## Test Plan

Go tests:

```bash
cd server
go test -tags "externalbin sqlite_fts5" ./cmd/api ./cmd/internal/spotify
```

Add or update focused tests for:

- Manual music scan trigger returns an already-running error when a scan is active.
- Track scan index skips unchanged files.
- Changed file size causes a track to be scanned.
- Scan-local musician and album ID caches prevent repeated local entity resolution for the same metadata.
- Existing Spotify package tests continue to cover successful artist and album API result caching.
- Spotify miss cache prevents repeated calls for no-result or below-threshold matches in the same scan.
- Persisted Spotify unmatched rows are respected on later scans.
- Persisted Spotify failed rows can be retried according to the chosen retry policy.
- Existing album cover and musician thumb are not updated when unchanged.
- Short transaction persist path rolls back one failed track without breaking later tracks.

Manual validation:

- Run a first scan on a representative library.
- Run an immediate second scan with no file changes.
- Run a third scan after changing or adding a small number of tracks.
- Browse music albums, musicians, tracks, latest albums, search, and playback after scanning.
- Trigger two manual scans quickly and confirm the second request reports already in progress.
- Watch logs during scan for database lock messages.

## Implementation Order

Use small commits after the checkpoint so rollback can be precise:

1. Commit: scan trigger already-running behavior.
2. Commit: track scan index for unchanged skipping.
3. Commit: split resolution from database writes.
4. Commit: scan-local caches.
5. Commit: persisted Spotify match queries and usage.
6. Commit: avoid no-op writes.
7. Commit: Spotify cache lifetime adjustment.
8. Commit: audio-specific ffprobe path.
9. Commit: tests and benchmark notes.

After each commit:

```bash
git status --short
cd server
go test -tags "externalbin sqlite_fts5" ./cmd/api ./cmd/internal/spotify
```

If a step breaks scanner behavior, revert only that step and reassess before continuing.

## Risks And Mitigations

Risk: changing transaction boundaries can introduce partial writes.

Mitigation: keep per-track transactions and make each track persist operation self-contained.

Risk: persisted Spotify misses can hide future valid matches.

Mitigation: store reason and timestamp, and allow retry for transient failures. For unmatched low-confidence results, consider a future manual re-identify feature.

Risk: scan-local cache keys can merge distinct artists or albums.

Mitigation: use conservative keys based on trimmed lowercase original metadata, not aggressive punctuation stripping.

Risk: removing Spotify cache clearing can keep stale data briefly.

Mitigation: rely on the existing TTL and clear/reinitialize only when credentials change.

Risk: audio-specific ffprobe output can omit a field currently needed by the scanner.

Mitigation: add the new method while keeping `GetMetadata` unchanged, then compare before/after metadata for sample MP3, FLAC, and M4A files.

## Out Of Scope

- Adding scanner worker goroutines or a worker pool.
- Rewriting the scanner architecture around a queue.
- Adding new external metadata providers.
- Changing movie scanner behavior.
- Changing HLS, playback, subtitles, or transcoding behavior.
- Adding a user-facing metadata re-identification UI.
