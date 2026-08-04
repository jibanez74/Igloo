# Music Scanner Audit and Metadata Provider Replacement

**Date:** 2026-08-03
**Branch audited:** `feature/audio-metadata` (at `3b48f9a6`)
**Scope:** the end-to-end music scanning pipeline — file discovery, tag extraction, entity
resolution, persistence, artwork, and the Spotify metadata enrichment that the scanner depends
on — plus a design for replacing Spotify with MusicBrainz + Cover Art Archive + TheAudioDB.

This document is an investigation and recommendation report. It changes no source file. Every
claim carries a `file:line` reference or a reproducible command against the development
library and database.

> **Premise check.** The brief assumed the scanner contains bugs and performance problems. It
> does. Thirteen correctness findings and ten performance/reliability findings are recorded
> below, six of them rated high. Four are already visible as wrong data in the development
> database and are reproduced here with the queries that show them.

---

## 1. Scope and method

### What was audited

| Area | Files |
|---|---|
| Scan orchestration | `server/cmd/api/music_scanner.go`, `server/cmd/api/scanner.go`, `server/cmd/internal/helpers/scanner.go` |
| Tag extraction | `server/cmd/internal/ffprobe/ffprobe_metadata.go`, `server/cmd/internal/helpers/sql_helpers.go`, `server/cmd/internal/helpers/files.go` |
| Metadata provider | `server/cmd/internal/spotify/` (5 files), `server/cmd/api/spotify_handler.go` |
| Persistence | `server/sqlc/schema.sql`, `server/sqlc/queries/{tracks,albums,musicians,genres,track_genres,track_musicians,musician_albums,music_spotify_matches}.sql` |
| Startup / config | `server/cmd/api/application.go`, `server/cmd/api/startup.go`, `server/cmd/api/config.go`, `server/cmd/api/shutdown.go` |
| Trigger surface | `server/cmd/api/settings_handler.go`, `server/cmd/api/routes.go`, `web/src/routes/_auth/settings/libraries.tsx` |
| Consumers of scanned data | `server/cmd/api/{album,musician,track}_handler.go`, `web/src/routes/_auth/music/`, `web/src/components/music/` |

### What was deliberately not audited

Playback (direct, HLS, transcoding) — covered by `docs/web-direct-playback-audit.md` and
`docs/web-hls-playback-audit.md`. Playlists, likes, user stats, and search ranking, except
where a scanner change forces a change in them. The movie scanner, except for the structural
comparison in §8.

### Method and evidence quality

Three classes of evidence, labelled throughout:

- **Confirmed** — read directly in the working tree at `3b48f9a6`, or measured against the
  live development library and database.
- **Likely** — follows necessarily from confirmed code, but the failing input was not produced
  locally.
- **Hypothesis** — inference; the report says what would settle it.

Live measurements were taken against `MUSIC_DIR=/home/jose-ibanez/samba/music` (`.env:26`) and
`db/igloo.db`. They are reproducible:

```bash
sqlite3 db/igloo.db "
select 'tracks', count(*) from tracks
union all select 'albums', count(*) from albums
union all select 'musicians', count(*) from musicians
union all select 'tracks_with_year', count(*) from tracks where year is not null
union all select 'musicians_with_thumb', count(*) from musicians where thumb is not null
union all select 'albums_with_cover', count(*) from albums where cover is not null;"

sqlite3 db/igloo.db "select container, count(*) from tracks group by container;"
sqlite3 db/igloo.db "select entity_type, status, reason, count(*)
                     from music_spotify_matches group by 1,2,3;"
sqlite3 db/igloo.db "select title, count(*) c from albums group by lower(title) having c > 1;"
sqlite3 db/igloo.db "select summary from musicians where summary is not null limit 3;"
```

Results at the time of writing:

| Measurement | Value |
|---|---|
| Tracks | 2267 — **all `m4a`**, zero mp3, zero flac |
| Albums / musicians | 211 / 325 |
| Tracks with a non-null `year` | **110 of 2267** |
| Musicians with a thumb | 298 (all `https://i.scdn.co/...`) |
| Albums with a cover | 189 (all `https://i.scdn.co/...`) |
| Spotify artist matches | 322 matched, 3 `score_below_threshold` |
| Spotify album matches | 189 matched, 20 `score_below_threshold`, 2 `no_results` |
| Duplicate album titles | `Greatest Hits` ×7 (legitimate), `Hamilton: An American Musical (Original Broadway Cast Recording)` ×2, `Lilo & Stitch (Original Motion Picture Soundtrack)` ×2 (**both defects**) |
| Sample generated bio | `Adele is an independent artist with 0 followers on Spotify.` |

A representative tag dump, used repeatedly below:

```bash
ffprobe -v quiet -print_format json -show_format \
  "/home/jose-ibanez/samba/music/Adriana Partimpim/Adriana Partimpim/01 Lição de Baião.m4a"
```

```json
"album": "Adriana Partimpim",  "album_artist": "Adriana Partimpim",
"artist": "Adriana Partimpim", "compilation": "0",
"date": "2004-06-17T12:00:00Z", "disc": "1/1", "genre": "Brazilian",
"sort_album": "Adriana Partimpim", "sort_artist": "Adriana Partimpim",
"sort_name": "Lição de Baião",   "title": "Lição de Baião", "track": "1/10"
```

---

## 2. Architecture inventory

### 2.1 Component responsibilities

| File | Responsibility |
|---|---|
| `music_scanner.go:24` `ScanMusicLibrary` | Entry point. Checks `MusicDir`, takes the single-flight guard, spawns `runMusicScan` |
| `music_scanner.go:41` `runMusicScan` | Loads the scan index, walks the tree, buffers files, flushes batches, logs the summary |
| `music_scanner.go:117` `processMusicBatch` | Iterates the batch **serially**: resolve then persist |
| `music_scanner.go:287` `resolveTrackFile` | ffprobe → `UpsertTrackParams`, plus artist/album resolution (which makes network calls) |
| `music_scanner.go:713` `persistResolvedTrack` | One `BeginTx`/`Commit` per file under `app.ScannerDBMu` |
| `helpers/scanner.go:107` `walkMediaLibrary` | `filepath.WalkDir` with an extension filter and a context check |
| `helpers/scanner.go:35` `ScanIndexUnchanged` | Change detection: cleaned path + size |
| `ffprobe_metadata.go:197` `GetAudioMetadata` | Forks ffprobe per file with a 60 s timeout |
| `internal/spotify/` | Client-credentials Spotify client, matching/scoring, 15-minute success cache |
| `scanner.go:10` `scannerBatchSize` | `54` |
| `settings_handler.go:318` `TriggerMusicScan` | `POST /api/settings/scan/music`, 409 if a scan is running, 200 immediately otherwise |
| `libraries.tsx:291` `handleScan` | The only UI that starts a scan |

### 2.2 Flow

```
startup (application.go:177)  ──┐
POST /api/settings/scan/music ──┴─→ musicScanGuard.TryBegin() ─→ go runMusicScan()
                                        │
                                        ├─ ListMusicTrackScanIndex  (whole tracks table into a map)
                                        ├─ WalkMediaLibraryContext  (mp3 | flac | m4a only)
                                        │     └─ trackUnchanged(path, size) ? skip : buffer
                                        └─ every 54 files → processMusicBatch
                                              └─ per file, serially:
                                                   resolveTrackFile
                                                     ├─ fork ffprobe                 (subprocess)
                                                     ├─ resolveTrackMusicians        (DB + Spotify HTTP)
                                                     └─ resolveAlbum                 (DB + Spotify HTTP)
                                                   persistResolvedTrack
                                                     └─ ScannerDBMu → BeginTx → … → Commit  (one fsync)
```

Everything in that diagram runs on **one goroutine**. There is no worker pool, no `errgroup`,
and no semaphore anywhere in the scanner.

### 2.3 Data model

`musicians` (`schema.sql:82-94`), `albums` (`:97-112`), `tracks` (`:115-146`), the join tables
`track_musicians` (`:165-176`), `musician_albums`, `musician_genres`, `album_genres`,
`track_genres`, the shared `genres` table keyed `(tag, genre_type)`, and the Spotify decision
cache `music_spotify_matches` (`:179-195`) with two `AFTER DELETE` cleanup triggers (`:197-203`).

Provider-specific columns live directly on the entity tables: `musicians.spotify_id`,
`spotify_popularity`, `spotify_followers`, `summary`, `thumb`; `albums.spotify_id`,
`spotify_popularity`, `cover`.

### 2.4 What is correct today

Worth stating plainly, because the rest of this report is critical:

- **Transaction isolation of the scan caches is right.** `musicScanContext.clone()` /
  `mergeFrom()` (`music_scanner.go:215-244`) ensure a rolled-back transaction never poisons
  the id caches, and `trackIndex` is only written after commit (`:743`). The invariant is
  documented in the struct comments (`:162-189`). This is careful code.
- **The persistent match cache is the right idea.** `music_spotify_matches` distinguishes
  final negatives (`matched`, `unmatched`) from transient failures (`failed`, retried next
  scan) — `musicSpotifyMatchStatusIsFinal:1315`. The replacement design keeps this and extends
  it.
- **`UpsertMusician`/`UpsertAlbum` use `COALESCE(excluded.x, table.x)`** for the enrichment
  columns (`musicians.sql`, `albums.sql:53-85`), so a later scan without a provider never
  nulls existing data.
- **The stream-file cache is invalidated after commit** (`:738`), so a moved or retyped track
  re-resolves.
- **Single-flight guarding and shutdown-aware cancellation** are in place
  (`helpers/scanner.go:55`, `shutdown.go:52`).
- **Test coverage is substantial.** `music_scanner_test.go` is ~2200 lines with a hand-written
  Spotify stub (`:176`), and it is the main safety net for everything proposed below.

---

## 3. Correctness findings

### 3.1 Finding M1 (high, Confirmed) — dates are silently discarded for 95% of the library

`helpers.ParseDate` (`sql_helpers.go:119-141`) tries six layouts:

```go
"2006-01-02", "2006-1-2", "2006-01-02T15:04:05", "2006", "01/02/2006", "02-01-2006"
```

The library's tags carry `"date": "2004-06-17T12:00:00Z"` — RFC 3339 with a zone. The list has
the naive `T15:04:05` variant but **not** the `Z`/offset variant, so `time.Parse` fails for
every file, `resolveTrackFile:347-353` skips the assignment, and `release_date` and `year` stay
NULL.

```
tracks_with_year | 110      (of 2267)
```

The 110 that do parse are the files whose `date` happens to be a bare year or bare ISO date.

**Fix.** Add `time.RFC3339`, `"2006-01-02T15:04:05Z07:00"`, `"2006-01"`, and `"2006-01-02 15:04:05"`
to the format list. Also worth handling ID3v2.3's `TYER`+`TDAT` pair, which ffmpeg surfaces as
a bare 4-digit `date` (already covered by `"2006"`).

**Related, same function family.** `ParseSlashNumber` (`sql_helpers.go:68-79`) parses `"1/10"`
to `1` and **throws away the total**. `tracks` has no `total_tracks` and `albums.total_tracks`
is populated only from Spotify — so an album's track count is unknown without a network match,
even though every file states it.

### 3.2 Finding M2 (high, Confirmed) — the metadata provider rewrites album identity, creating duplicates

`albums.musician` is a denormalized artist name that is **also half of the identity key**
`UNIQUE (title, musician)` (`schema.sql:111`). It is written from two different sources:

- Miss path — `persistAlbum:995-997`: `params.Musician = input.albumArtist`, i.e. the raw tag.
- Hit path — `persistAlbum:967-969`: same field, but `input.albumArtist` at that point may have
  come from a different track, and `UpsertAlbum`'s conflict target is `(title, musician)`.

When two tracks of the same album carry different `album_artist` spellings — the norm for cast
recordings and soundtracks, where iTunes writes a full credit list on some tracks and a lead
name on others — the two spellings produce two different keys, and `ON CONFLICT` does not fire.
Two albums result.

```
Hamilton: An American Musical (…) | Lin-Manuel Miranda                                        | 2015 | spotify:…
Hamilton: An American Musical (…) | Lin-Manuel Miranda, Leslie Odom, Jr., … & Christopher Jackson | NULL | NULL
Lilo & Stitch (Original Motion Picture Soundtrack) | Mark Keali'i Ho'omalu                     | 2025 | spotify:…
Lilo & Stitch (Original Motion Picture Soundtrack) | Mark Keali'i Ho'omalu, Iam Tongi & Dan Romer | NULL | NULL
```

Both pairs are in the development database now. The user-visible symptom is an album that
appears twice in the grid, one copy with a cover and a year, one without, each holding a
subset of the tracks.

**Root cause, stated generally:** *identity must not be derived from a field a metadata
provider is allowed to write.* The fix in §6.4 separates a computed, tag-only `album_key` from
the display string.

### 3.3 Finding M3 (high, Confirmed) — `ON CONFLICT (title, musician)` never fires for NULL artists

SQLite treats NULLs as distinct in a UNIQUE index. `albums.musician` is nullable, and
`helpers.NullString("")` returns `{Valid: false}` (`sql_helpers.go:13-19`), so an album with no
`album_artist` **and** no `artist` tag gets `musician = NULL`.

For such an album, `UNIQUE (title, musician)` admits unlimited duplicate rows and
`UpsertAlbum ON CONFLICT (title, musician)` (`albums.sql`) silently degrades to a plain
`INSERT`. Today this is masked by the read-before-write path — `findExistingAlbum:599` uses
`GetAlbumByTitleAndMusician` with the NULL-safe `musician IS ?` — and by the in-scan
`albumIDs` cache. The constraint the schema *appears* to provide, however, does not exist.

**Grade: Confirmed** for the constraint semantics; **Likely** for reaching it in practice,
since the read path currently covers it. It becomes reachable the moment anything writes albums
concurrently, or the cache is bypassed.

### 3.4 Finding M4 (high, Confirmed) — no orphan cleanup, ever

`grep` for `DeleteTrack`/orphan/prune across `server/cmd` finds only the join-table helpers
(`DeleteTrackGenres`, `DeleteTrackMusicians`) and the admin-only `DeleteAlbum` handler
(`album_handler.go:180`). Consequences:

- A file deleted from disk keeps its `tracks` row indefinitely. It stays listed, searchable,
  and addable to playlists; playback 404s at stream time.
- A file **moved or renamed** produces a new `file_path`, so a new row is inserted and the old
  one is left behind. Reorganising a library doubles it.
- `musicians`, `albums`, and `genres` that lose all their tracks are never pruned, so the
  artist and album grids accumulate empty entries.

**Fix.** After a successful full walk (and only then — a partial or cancelled walk must not
prune), diff the walked path set against `trackIndex` and delete the residue, then delete
albums/musicians/genres with no remaining references. This must be gated on the walk having
completed without error, or an unmounted network share wipes the library.

### 3.5 Finding M5 (high, Confirmed) — change detection cannot see a retag

```go
func ScanIndexUnchanged(index map[string]int64, path string, size int64) bool {
	existingSize, ok := index[filepath.Clean(path)]
	return ok && existingSize == size
}
```
`helpers/scanner.go:35-38`

The key is `(cleaned path, size)`. No mtime, no ctime, no content hash. Tag editors reuse the
existing ID3v2 padding or FLAC padding block whenever the new tag fits, which is the common
case, so **the file size does not change and the track is never rescanned.** A user who fixes an
artist name in Picard and rescans sees nothing happen, with no error and no log line.

This interacts badly with M9: even when a rescan *is* triggered, existing musicians and albums
short-circuit before their scalar fields are refreshed.

**Fix.** Extend the index to `(path, size, mtime_unix)` — `fs.DirEntry.Info()` already returns
`ModTime()` in the walk (`helpers/scanner.go:136`), so this is nearly free, and `tracks` needs
one `mtime` column. A content hash is not warranted; mtime plus size catches every realistic
retag.

### 3.6 Finding M6 (high, Confirmed) — compilations fracture into one album per artist

`resolveTrackFile:399-402`:

```go
effectiveAlbumArtist := tags.AlbumArtist
if effectiveAlbumArtist == "" {
	effectiveAlbumArtist = tags.Artist
}
```

For a Various Artists disc with no `album_artist` tag, every track's own artist becomes the
album's `musician` — and `musician` is half the identity key (M2), so a 15-track compilation
becomes up to 15 one-track albums.

The `compilation` tag that would prevent this **is present and populated in this library**
(`"compilation": "0"` in the dump in §1) and is **not parsed at all**: `FormatTags`
(`ffprobe_metadata.go:88-102`) has no field for it. The tag is standard across all three
containers — ID3v2 `TCMP`, Vorbis `COMPILATION`, MP4 `cpil` — and `normalizeTagKey`
(`:157-163`) already folds all three spellings onto `compilation`.

**Fix.** Parse it, and model album artist properly (§6.4).

### 3.7 Finding M7 (medium, Confirmed) — the artist-split decision is delegated to a network failure

`resolveTrackMusicians:414-442` decides whether `"A & B"` is one artist or two. The decision
path is:

1. `shouldSplitCompoundArtistCreditsLocally` (`:671-687`) — split if there is a comma **and**
   every comma-part has ≥ 2 words, or if a part repeats.
2. Otherwise, ask Spotify for the whole string, and split only if Spotify replies `no_results`
   or `score_below_threshold` — `shouldSplitCompoundArtistCredits(err)` at `:689-696`, wired
   through `resolvedMusician.splitCompoundOnNoMatch`.

Two problems.

**The shape of the library depends on an API's mood.** A Spotify outage returns
`search_failed`, not `no_results`, so the string stays whole; a rate-limit window returns
something else again. To keep the outcome stable across scans the decision has to be persisted
in `music_spotify_matches.reason` and re-read (`:471`), which is why that column is load-bearing
rather than diagnostic.

**The local heuristic is wrong in both directions.** `Earth, Wind & Fire` has a comma and two
multi-word parts, so rule 1 splits it into `Earth` and `Wind & Fire`. `Brooks & Dunn` survives
only because it has no comma. This library contains `Tom Petty & The Heartbreakers`,
`Keith Strachan & Matthew Strachan`, and `Los Enanitos Verdes & La Gusana Ciega` — all single
acts or MusicBrainz-registered pairs.

A wrongly-split name is structurally destructive (two junk artist rows, tracks attributed to
neither real artist, and an entity that then fails to match any provider). A compound name kept
whole is cosmetic. The policy should be asymmetric accordingly — see §6.5.

### 3.8 Finding M8 (medium, Confirmed) — artist dedup is case- and diacritic-sensitive

`musicians.name TEXT NOT NULL UNIQUE` (`schema.sql:84`) with `GetMusicianByName` doing an exact
`=` comparison (`music_scanner.go:589`). SQLite's default `BINARY` collation makes `Beatles`
and `beatles` two rows.

`COLLATE NOCASE` is the obvious fix and is the wrong one here: SQLite's `NOCASE` folds ASCII
A–Z only. This library is heavily Latin-American — `Alceu Valença`, `Héctor Lavoe`,
`Adriana Partimpim`, `Los Enanitos Verdes` — so `Beyoncé`/`Beyonce`, `Valença`/`Valenca`, and
`P!nk`/`Pink` all stay split under `NOCASE`.

**Fix.** A normalized `name_key` column (§6.4), computed with the normalizer the codebase
already has and already unit-tests — `normalizeComparisonText` in `spotify_match.go:118` does
NFD decomposition, combining-mark stripping, `&`/`+` → `and`, punctuation collapsing, and
lowercasing. Promoting it to `helpers` makes the database key identical to the scoring key.

### 3.9 Finding M9 (medium, Confirmed) — existing entities are never refreshed from changed tags

`persistMusician:901-907` and `persistAlbum:983-989`:

```go
if input.hasExistingID {
	err = app.upsertMusicSpotifyMatchAndCacheID(...)
	...
	return input.existingID, nil
}
```

When an entity already exists and its match row is final, the function returns without calling
`UpsertMusician`/`UpsertAlbum` at all. A corrected `sort_name`, a corrected `sort_title`, or a
newly-added `year` tag is therefore never written, even on a forced rescan. Combined with M5
(the rescan usually does not happen at all), tag corrections are effectively a one-way door:
the first scan wins forever.

### 3.10 Finding M10 (medium, Confirmed) — generated artist biographies are fabricated and wrong

`generateMusicianSummary` (`music_scanner.go:1327-1367`) synthesises prose from
`artist.Popularity` and `artist.Followers.Count`. Spotify does not populate those fields on
search results — only on a full `GetArtist` — so both are zero for every artist the scanner
resolves through `SearchArtistByName`.

```
Acoustic Heartstrings is an independent artist with 0 followers on Spotify.
Adele is an independent artist with 0 followers on Spotify.
Adriana Partimpim is an independent artist with 0 followers on Spotify.
```

322 of 325 musicians carry a sentence of this form. Adele is not an independent artist with
zero followers. This is presented to the user as a biography on the artist detail page.

Note that Spotify has **no biography field at all** — the synthesis exists because there was
nothing to show. TheAudioDB's `strBiographyEN` is real prose, which makes this a categorical
improvement rather than a lateral move.

### 3.11 Finding M11 (medium, Confirmed) — there is no artwork pipeline

- **No embedded art extraction.** `music_scanner_test.go:734`
  (`TestProcessMusicBatchIgnoresEmbeddedArtworkWithoutSpotifyMatch`) asserts this as intended
  behaviour. It is also currently *impossible*: `GetAudioMetadata`'s `-show_entries` list
  (`ffprobe_metadata.go:203`) does not include `stream_disposition`, so `attached_pic` is never
  in the JSON.
- **No folder art.** Zero occurrences of `cover.jpg`, `folder.jpg`, or `front.png` anywhere in
  the repository.
- **No download, no resizing, no local cache.** `albums.cover` and `musicians.thumb` store the
  raw `images[0].URL` from Spotify (`firstImageURL:1369`) — 189 and 298 rows of `i.scdn.co`
  URLs respectively. These are CDN URLs on a third-party host that Spotify rotates; they will
  rot, and they break entirely offline.
- **`static/albums` and `static/musicians` are created at boot and never written**
  (`startup.go:174-182`), under a comment that reads *"Scanner-downloaded artwork is stored
  beneath static/"*.
- **The `download_images` setting is dead.** It exists in `config.go:105`, `startup.go:143`,
  the settings API, and the admin UI, and is read by no scanner code path.

The plumbing for the fix is already there: `getMediaImageUrl` (`web/src/lib/media-image-url.ts:13`)
already accepts a same-origin `/api/...` path, and `GET /api/static/*` is already routed
(`routes.go:63`) with long-lived cache headers (`static_handler.go:80`).

### 3.12 Finding M12 (medium, Confirmed) — format coverage is three containers

`helpers.ValidAudioExtensions` (`files.go:15-19`) is `mp3`, `flac`, `m4a`. No ogg, opus, wav,
aiff, wma, or ALAC-in-CAF. The restriction is repeated as `CHECK` constraints on
`tracks.container` and `tracks.mime_type` (`schema.sql:120-124`), so widening it is a schema
change, not a map edit.

Opus and Ogg Vorbis are the notable gaps for a self-hosted music server — both are natively
playable in every target browser, so they would need no transcoding.

### 3.13 Finding M13 (low, Confirmed) — three small extraction defects

- **The title fallback includes the file extension.** `resolveTrackFile:293,305`:
  `fileName := filepath.Base(file.Path)`, then `params.Title = fileName`. An untitled file
  becomes the track `01 Untitled.m4a`, extension and all.
- **`tracks.channels` duplicates `channel_layout`.** `:363-369` assigns the same value to both
  columns when a layout is present, and the same stringified count to both when it is not. One
  of the two columns carries no information.
- **`sample_rate` is never captured.** `ffprobe.Stream` has the field (`ffprobe_metadata.go:26`)
  but `-show_entries` omits it and `tracks` has no column, so a FLAC library cannot show
  24/96 vs 16/44.1 — the single most requested piece of metadata for a lossless collection.

---

## 4. Performance and reliability findings

### 4.1 Finding P1 (high, Confirmed) — one transaction, and one fsync, per audio file

`persistResolvedTrack:713-747` opens and commits a transaction for every single file, holding
`app.ScannerDBMu` across the whole thing. The connection pool is one connection
(`startup.go:53-54`), so the scan and every concurrent API request contend on it — which also
makes `ScannerDBMu` largely redundant.

`scannerBatchSize = 54` (`scanner.go:10`) buys nothing at the database layer: `processMusicBatch`
just loops over the batch one file at a time.

WAL is enabled (`startup.go:62`) but nothing else is tuned:

```go
sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
```

No `_synchronous=NORMAL`, no `cache_size`, no `mmap_size`, no `temp_store=MEMORY`. Under WAL,
`synchronous=FULL` fsyncs the WAL on every commit. For a 50 000-track library that is 50 000
fsyncs; `NORMAL` is the standard and safe choice under WAL (a crash can lose the last
transaction, not the database).

**Fix.** Batch N tracks per transaction, and add `_synchronous=NORMAL&_cache_size=-32000&_temp_store=MEMORY`.
The clone/merge cache discipline (`:215-244`) already models transaction-scoped state
correctly, so widening the transaction is a change of granularity, not of design.

### 4.2 Finding P2 (high, Confirmed) — one ffprobe fork+exec per file, serially

`ffprobe_metadata.go:216` forks the 107 MB embedded ffprobe binary once per file, on the scan
goroutine, with nothing else in flight. On a large library this dominates wall-clock.

It is trivially parallelizable: `resolveTrackFile` is pure with respect to the database except
for the read-only existence lookups, and the design *already* separates resolve from persist.
A worker pool over resolve, feeding a single persist goroutine, is the natural shape — and it
composes with P1 (batch the persist side) and with §6.6 (move the network out of resolve
entirely).

### 4.3 Finding P3 (high, Confirmed) — network calls are inline in the file walk

`resolveMusician:494` and `resolveAlbum:568` issue Spotify requests from inside
`processMusicBatch`. An album lookup is up to three HTTP requests
(`spotify_albums.go:60,109,150`: field-query search, plain-text search, then `GetAlbum`).

This is survivable against Spotify's generous undocumented limits. It is **not survivable
against MusicBrainz's 1 request/second**, which would impose a hard one-second floor between
tracks. It is the single strongest argument for the two-phase split in §6.6.

### 4.4 Finding P4 (high, Confirmed) — no rate limiting, no retry, no `Retry-After`

```go
client := spotify.New(httpClient)
```
`spotify.go:55`

`spotify.WithRetry(true)` is not passed, so `autoRetry` is false in `zmb3/spotify v2.4.3` and a
**429 is returned immediately as a hard error**. The library's `Retry-After` handling is dead
code in this configuration. There is no rate limiter, no backoff, and no inter-request pacing
anywhere in the package or the scanner.

The consequence: under a 429 storm the scanner records every affected entity as
`status='failed'` (`resolvedSpotifyMatchFromError:1271`) and keeps hammering. Nothing surfaces
to the user; the only evidence is a `Warn` line per track.

`internal/tmdb` next door does all three correctly — three attempts, `Retry-After` honoured as
either integer seconds or HTTP-date, exponential backoff otherwise, `helpers.TMDB_HTTP_TIMEOUT`.
The pattern to copy already exists in the repository.

The Spotify package's own tests exercise 500 and 502 only (`spotify_test.go:309,693,708`);
**there is no 429 test.**

### 4.5 Finding P5 (medium, Confirmed) — N+1 database round trips on a cold scan

Before the in-scan caches warm, each distinct artist costs `GetMusicianByName` (`:457`) plus
`GetMusicSpotifyMatch` (`:465`), and each distinct album costs `GetAlbumByTitleAndMusician`
(`:533`) plus `GetMusicSpotifyMatch` (`:541`) — all outside the transaction, all on the
single-connection pool.

Additionally, `syncTrackMusicians:1053-1061` issues one `CreateTrackMusician` per artist, and
`DeleteTrackMusiciansExcept` is a `sqlc.slice` query (`database/track_musicians.sql.go:54-68`),
so its SQL is rebuilt and re-parsed on every call instead of using a prepared statement.

`ListMusicTrackScanIndex` also pulls the entire tracks table into memory on every scan, and
`trackUnchanged` is evaluated twice per file (walk callback `:87` and `processMusicBatch:123`).

### 4.6 Finding P6 (high, Confirmed) — resolve failures are counted but never logged

```go
resolved, err := app.resolveTrackFile(ctx, scan, file)
if err != nil {
	errCount++
	continue
}
```
`music_scanner.go:128-132`

The path and the error are both discarded. A corrupt file, an ffprobe timeout, a permission
error, or a hard Spotify error during resolve all vanish into a bare integer in the summary
line. The movie scanner does this correctly two files away:

```go
app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.Path, err.Error()))
```
`movies_scanner.go:134`

Related: join-row failures are logged at `Warn` and **swallowed**, so the transaction commits
with partial relationships — musician↔album (`:783-788`), musician↔genre (`:829-834`),
album↔genre (`:840-845`), and every Spotify-genre write (`processSpotifyEntityGenres:1119-1133`).
Persist failures log at `Warn` (`:136`) rather than `Error`.

### 4.7 Finding P7 (medium, Confirmed) — the scan context does not reach ffprobe

```go
ctx, cancel := context.WithTimeout(context.Background(), metadataTimeout) // 60s
```
`ffprobe_metadata.go:213`

`runMetadata` builds its own context from `context.Background()`. The scan context is not
threaded through, so on SIGTERM `app.Wait.Wait()` (`shutdown.go:40`) can block for up to 60
seconds on an in-flight probe. This is the same class of bug as the subtitle-extraction timeout
already recorded elsewhere in this project, and the fix is the same: accept a `context.Context`
parameter.

### 4.8 Finding P8 (low, Confirmed) — a cancelled scan logs as a completed scan

`processMusicBatch:119-121` returns early on `ctx.Err()` without signalling. Control returns to
`runMusicScan`, which falls through to `:113` and logs
`"music scanner completed: %d scanned, %d skipped, %d errors"` as though the scan finished. The
`"music library scan canceled"` line at `:104` is only reached when the *walk* is cancelled, not
the final flush.

### 4.9 Finding P9 (low, Confirmed) — FTS re-indexing on every enrichment write

```sql
CREATE TRIGGER IF NOT EXISTS albums_au AFTER UPDATE ON albums BEGIN
```
`schema.sql:755`, and `musicians_au` at `:780`

Neither has an `OF <columns>` clause, so any column update — including `cover`, `summary`,
`spotify_popularity` — deletes and re-inserts the FTS5 row. The vocab triggers next to them do
it correctly (`search_vocab_albums_au AFTER UPDATE OF title, musician ON albums`, `:897`;
`search_vocab_musicians_au AFTER UPDATE OF name, sort_name`, `:915`), which shows the pattern is
known and simply was not applied to the main FTS triggers.

This matters more after §6.6, where an enrichment pass writes an artwork path and a biography
to every album and artist row.

### 4.10 Finding P10 (high, Confirmed) — no scan progress exists, and the UI lies about it

There is no progress model, no status endpoint, no SSE, and no notification row. The scanners
never touch the notifications table. `TriggerMusicScan` (`settings_handler.go:318`) starts the
goroutine and returns `200 {"message": "Music library scan started"}`. `docs/openapi.json:3185-3211`
documents it as *"The scan runs asynchronously"* with no companion status operation.

The UI compounds this. `libraries.tsx:318-328`:

```ts
setBanner(...)                            // ":318"
showSuccess("Scan started", ...)          // ":323"
invalidateScanQueries(queryClient, scan)  // ":327"  ← while the walk is still running
setActiveScan(current => …)               // ":328"  ← clears the "Scanning…" state
```

The React Query caches are invalidated the instant the POST returns, so the refetch races the
scan and almost always shows the pre-scan state. The `aria-busy`/"Scanning…" affordance
(`:517-540`) lives for the duration of the HTTP request, not the scan. From the user's side, a
scan of a large library appears to complete instantly and change nothing.

---

## 5. Provider evaluation

### 5.1 What Spotify actually provides today, and at what cost

| Field | Source | Quality |
|---|---|---|
| `musicians.thumb` | `images[0].URL` | Good coverage (298/325), but a rotating CDN URL on a third-party host |
| `musicians.summary` | synthesised locally | **Wrong** (M10) — Spotify has no biography field |
| `musicians.spotify_popularity` / `_followers` | search result | **Always 0** — not populated on search results |
| `albums.cover` | `images[0].URL` | Good coverage (189/211), same rotation problem |
| `albums.release_date` / `year` / `total_tracks` | album details | Good — and currently the *only* source, because of M1 |
| genres | artist + album `genres[]` | Spotify's genre vocabulary is idiosyncratic (`"escape room"`, `"pop rap"`) |
| identity | opaque Spotify ID | Not portable, not stable across catalogue changes, not shared with any other tool |

Spotify's structural problem is not coverage — it is that it is a **streaming catalogue, not a
music database**. It has no biographies, no release-vs-release-group distinction, no reliable
compilation flag, no artist-credit decomposition, and no identifier that any tagger (Picard,
beets, Lidarr) or any other metadata service understands.

### 5.2 Candidate comparison

| | Key | Rate limit | Identity | Artist images | Bios | Album art | Non-Anglophone | Licence |
|---|---|---|---|---|---|---|---|---|
| **MusicBrainz** | none | **1 req/s per IP**, 503 over | **MBID — the industry standard** | none | none | none (see CAA) | Excellent (community-curated, aliases) | CC0 core data |
| **Cover Art Archive** | none | none enforced | keyed by MBID | none | none | **Excellent** | Good | per-image third-party rights |
| **TheAudioDB** | shared `123` free | **30/min shared**, 429 over | own IDs + MBID lookup | **Yes — the only source** | **Yes, real prose** | Yes | Moderate | attribution required |
| Last.fm | yes | generous | own | **No** — placeholder star since 2019 | Yes | Weak | Good | restrictive |
| Discogs | yes (token) | 60/min | own | Yes | Weak | Yes | Excellent for vinyl/regional | non-commercial |
| fanart.tv | yes (personal) | generous | keyed by MBID | **Yes, high quality** | No | Yes | Moderate | attribution |
| AcoustID | yes | 3 req/s | → MBID | No | No | No | n/a | non-commercial |
| *Spotify (incumbent)* | yes (OAuth) | undocumented | opaque | Yes | **No** | Yes | Good | ToS-restricted |

Verified endpoint facts used above:

- **MusicBrainz** — `https://musicbrainz.org/ws/2/`, no key, a hard **1 request/second per IP**
  with HTTP 503 over the limit, a mandatory descriptive `User-Agent` (blank/generic agents are
  throttled harder), JSON via `?fmt=json`, and `inc=` expansions for `aliases`, `genres`,
  `tags`, `artist-credits`, and `release-groups`. Every search result carries its own 0–100
  relevance `score`.
- **Cover Art Archive** — `https://coverartarchive.org/`, no key, **no rate limit currently
  enforced**. `GET /release-group/{mbid}/front-500` returns a **307** to archive.org; 404 when
  no front image is designated; 400 on a malformed UUID. Thumbnail sizes are 250, 500, 1200.
- **TheAudioDB** — the free tier was restricted; the shared public test key is `123` at
  **30 requests/minute**, HTTP 429 over the limit, and a private key requires the $8 Patreon
  tier (100/min). `artist-mb.php?i={artist_mbid}` and `album-mb.php?i={release_group_mbid}`
  give MBID-keyed lookups returning `strBiographyEN` (plus other languages), `strArtistThumb`,
  `strArtistFanart[2,3]`, `strArtistBanner`, `strArtistLogo`, `strArtistClearart`, `strGenre`,
  `strStyle`, `strMood`, `intFormedYear`, `strCountry`; albums return `strAlbumThumb`,
  `strAlbumCDart`, `strDescriptionEN`, `intYearReleased`.
- **AcoustID** — `https://api.acoustid.org/v2/lookup`, client key required, max 3 req/s, no
  commercial use, and it needs Chromaprint `fpcalc` as an additional per-platform binary.

### 5.3 Recommendation

**MusicBrainz for identity, Cover Art Archive for album art, TheAudioDB for artist images and
biographies.** Rationale:

1. **MBIDs are portable.** A library tagged by Picard already carries them, which makes the
   whole lookup free for well-tagged collections and makes Igloo interoperable with beets,
   Lidarr, Navidrome, and Jellyfin.
2. **Two of the three need no key**, so the feature works out of the box — unlike Spotify,
   which requires the user to register an application before any metadata appears at all.
3. **MusicBrainz answers questions Spotify structurally cannot**: is this a compilation
   (release-group `secondary-types`), who are the individual credited artists (the
   `artist-credit` array with join phrases), which release does this pressing correspond to.
   Those are exactly the questions behind M6 and M7.
4. **The data does not rot.** CAA images are archived at the Internet Archive; Spotify CDN URLs
   are not stable.

### 5.4 The honest downside

**Artist photo coverage will regress, visibly.** MusicBrainz has no images and CAA has no
artist images, so TheAudioDB is the *only* source — on a shared 30 req/min key, with coverage
well below Spotify's. Today 298 of 325 musicians have a thumb. Expect materially fewer.

This is the single biggest user-facing risk in the change and it is carried into §12 as an open
question rather than assumed away.

**Also lost:** the `SpotifyPopularityMeter` component (`web/src/components/music/SpotifyPopularity.tsx`),
rendered on both the musician detail page (`musician.$id.tsx:468`) and the album detail page
(`album.$id.tsx:49`). Since the underlying values are always 0 today (M10), removing it deletes
a meter that has never shown a true number — but it is a visible UI element and its removal
must be a deliberate decision, not a side effect.

---

## 6. Target design

### 6.1 Layered resolution — network last

| Tier | Source | Cost | Covers |
|---|---|---|---|
| 0 | MBIDs already in the file tags | free | Picard/beets-tagged libraries — instant, offline |
| 1 | Embedded `attached_pic`, then folder `cover.jpg`/`folder.jpg`/`front.png` | disk only | Ripped and downloaded libraries |
| 2 | MusicBrainz search | 1 req/s | Everything else — establishes the MBID |
| 3 | CAA album art, TheAudioDB artist art + biography | CAA free, TADB 30/min | Enrichment on top of a known MBID |

**Tier 0 needs no ffprobe argument change.** `GetAudioMetadata` already passes
`-show_entries "…:format_tags:…"` with no `=` after `format_tags`, which means *all* format
tags — the `iTunSMPB`, `compilation`, and `purchase_date` keys in the §1 dump prove it. The
MBIDs are already in the JSON; they are dropped by `FormatTags.UnmarshalJSON`
(`ffprobe_metadata.go:104-125`), which picks 13 keys and discards the rest.

`normalizeTagKey` (`:157-163`) lowercases and strips `_`, `-`, and space, which collapses the
container-specific spellings onto one key:

| Container | Raw ffprobe key | Normalized |
|---|---|---|
| FLAC Vorbis | `MUSICBRAINZ_ALBUMID` | `musicbrainzalbumid` |
| MP3 ID3v2 `TXXX` | `MusicBrainz Album Id` | `musicbrainzalbumid` |
| FLAC / MP3 | `MUSICBRAINZ_RELEASEGROUPID` / `MusicBrainz Release Group Id` | `musicbrainzreleasegroupid` |
| FLAC / MP3 | `MUSICBRAINZ_ARTISTID` / `MusicBrainz Artist Id` | `musicbrainzartistid` |
| FLAC / MP3 | `MUSICBRAINZ_ALBUMARTISTID` / `MusicBrainz Album Artist Id` | `musicbrainzalbumartistid` |
| FLAC | `MUSICBRAINZ_TRACKID` (recording MBID) | `musicbrainztrackid` |
| all three | `COMPILATION` / `TCMP` / `cpil` | `compilation` |
| all three | `TOTALTRACKS` / `TRACKTOTAL` | `totaltracks` |

Three caveats that must be written into the implementation:

1. **MP3 recording MBIDs are unreachable.** Picard writes the recording MBID into an ID3v2
   `UFID` frame, and ffmpeg's ID3v2 reader does not surface `UFID` as metadata. Only
   `TXXX`-backed IDs are visible for MP3. Do not design anything that requires a recording MBID
   from an MP3.
2. **M4A freeform atoms may keep a mean prefix.** Picard writes
   `----:com.apple.iTunes:MusicBrainz Album Id`. `normalizeTagKey` strips `_`, `-`, and space
   but **not `.` or `:`**, so a writer that emits `com.apple.iTunes:MusicBrainz Album Id` lands
   on `com.apple.itunes:musicbrainzalbumid` and never matches. Strip a
   `com.apple.itunes[.:]` prefix before lookup.
3. **First-wins collision.** `normalizedTagValues:147-149` keeps the first value per normalized
   key and Go map iteration is randomized, so a file carrying two spellings with different
   values resolves nondeterministically. Acceptable, but document it.

New `FormatTags` fields (no ffprobe argument change required for any of these):

```go
type FormatTags struct {
	// … existing 13 fields …
	Compilation      string // "0"/"1"
	MbReleaseID      string
	MbReleaseGroupID string
	MbArtistID       string
	MbAlbumArtistID  string
	MbRecordingID    string
	TotalTracks      string
	TotalDiscs       string
}
```

Plus a `helpers.NormalizeMBID` that lowercases, trims, and validates the 8-4-4-4-12 UUID shape
— a malformed MBID returns 400 from CAA and 404 from MusicBrainz, and both would otherwise be
recorded as spurious failures.

**Tier 1 does need argument changes**, and they are the same ones M13 asks for:

```
-show_entries "format=duration,bit_rate:format_tags:stream=index,codec_name,codec_type,profile,channels,channel_layout,sample_rate:stream_disposition=attached_pic:stream_tags=language"
```

Embedded art is then detectable as any stream with `CodecType == "video"` and
`Disposition.AttachedPic == 1`, extracted via a new
`ffmpeg.ExtractAttachedPic(ctx, src, dst)` wrapping
`ffmpeg -v error -y -i <src> -map 0:v:0 -c copy -frames:v 1 <dst>`. Folder art searches the
**immediate directory only** (never recursively — this library contains `.itlp` iTunes-LP
bundles full of `photo01.jpg` and `background.jpg` that would be mistaken for covers), with the
per-directory result memoized in the scan context so a 20-track album stats the directory once.

**Tier 2** prefers `arid:<mbid>` over `artist:"<name>"` for the release-group query once the
artist MBID is known — this removes the free-text ambiguity that is Spotify's main weakness and
is the principal structural win. Request `inc=aliases` on artist lookups and score against every
alias, taking the maximum: MusicBrainz aliases are how `The Beatles`/`Beatles` and Latin-script
variants match, and they are where MusicBrainz's raw search is otherwise weaker than Spotify's.

**Tier 3** uses **MBID-keyed lookups only** against TheAudioDB (`artist-mb.php`, `album-mb.php`).
Do not use `search.php?s=` by name; it reintroduces exactly the fuzzy-matching problem
TheAudioDB is worst at.

### 6.2 Field precedence

| Field | Precedence |
|---|---|
| `musicians.name` / `sort_name` | tag only — **never** overwritten by a provider |
| `musicians.mb_artist_id` | tag MBID → MusicBrainz search |
| `musicians.summary` | TheAudioDB `strBiographyEN` → NULL (delete `generateMusicianSummary`) |
| `musicians.thumb` | TheAudioDB `strArtistThumb` → `strArtistFanart` → NULL |
| `albums.title` / `sort_title` | tag only |
| `albums.release_date` / `year` | tag `date` (after the M1 fix) → MB `first-release-date` → TheAudioDB |
| `albums.total_tracks` | tag `totaltracks` → MB release track count |
| `albums.cover` | embedded → folder file → CAA release-group → TheAudioDB → NULL |
| genres | tag `genre` → MB `genres[]` → TheAudioDB `strGenre`/`strStyle` |

The first row is the rule M2 violates and is the most important line in the table.

### 6.3 A provider-neutral interface

New package `server/cmd/internal/musicmeta/`, one file per concern, mirroring the existing
`spotify/` and `tmdb/` layout:

```
musicmeta.go   Provider interface, DTOs, New(), Config
match.go       MatchInfo, MatchError, reason constants, scoring (ported from spotify_match.go)
musicbrainz.go MB search + lookup
coverart.go    CAA front-art fetch
audiodb.go     TheAudioDB artist/album lookups
httpclient.go  shared getJSON with retry + Retry-After (modeled on tmdb)
limiter.go     stdlib token bucket
aggregate.go   the chained Provider implementation
cache.go       go-cache instances (already a dependency)
```

DTOs are **Igloo-owned**. This is the change that fixes the current architecture's worst
property: `*spotify.FullArtist` and `*spotify.FullAlbum` leak from a third-party library into
`music_scanner.go:272,283` and into `spotify_handler.go`, so the scanner is coupled to a vendor
SDK's types.

```go
type ArtistRef struct {
	MBID, Name, SortName string
	JoinPhrase           string // MusicBrainz's literal join text: " & ", ", ", " feat. "
}

type Artist struct {
	MBID, Name, SortName, Disambiguation string
	Type, Country                        string
	BeginYear                            int
	Genres                               []string

	AudioDBID, Biography, ThumbURL string // empty when TheAudioDB had no entry
	FanartURLs                     []string
}

type Album struct {
	ReleaseGroupMBID, ReleaseMBID string
	Title, Disambiguation         string
	PrimaryType                   string   // "Album", "EP", "Single"
	SecondaryTypes                []string // "Compilation", "Soundtrack", "Live"
	FirstReleaseDate              string   // "YYYY" | "YYYY-MM" | "YYYY-MM-DD"
	ArtistCredit                  []ArtistRef
	TotalTracks                   int
	Genres                        []string

	AudioDBID, Description, ThumbURL string
}

func (a *Album) IsCompilation() bool

type Artwork struct {
	Data        []byte
	ContentType string
	SourceURL   string
	Source      string // "embedded" | "folder" | "coverart" | "audiodb" | "remote"
}

type Provider interface {
	LookupArtist(ctx context.Context, q ArtistQuery) (*Artist, error)
	LookupAlbum(ctx context.Context, q AlbumQuery) (*Album, error)
	FetchAlbumArtwork(ctx context.Context, album *Album) (*Artwork, error)
	FetchArtistArtwork(ctx context.Context, artist *Artist) (*Artwork, error)
	ClearAllCaches()
}
```

The error type carries every field `music_spotify_matches` records today, plus the provider
dimension and the upstream's own score:

```go
type MatchInfo struct {
	Provider, Lookup, Input   string
	SearchQuery, Strategy     string
	CandidateID, CandidateName, CandidateArtist string
	Score, ProviderScore, Threshold int
	Reason                    string
}

type MatchError struct {
	Info MatchInfo
	Err  error
}

// Terminal reports whether this is a settled negative answer that must not be
// retried on a later scan, as opposed to a transient failure.
func (e *MatchError) Terminal() bool
```

`Terminal()` is a real improvement over the status quo: `musicSpotifyReasonIsUnmatched`
(`music_scanner.go:1319`) currently duplicates this classification in the `main` package by
string-comparing reasons that the `spotify` package produced. Moving it onto the error type
means the scanner simply asks.

**Composition: one aggregating client, not three.** `New(cfg) (Provider, error)` builds a single
`musicmeta.client` satisfying `Provider`; `app.MusicMeta` is one nil-able field like `app.Tmdb`.
Reasons:

1. The fallback policy is provider-crossing — `FetchAlbumArtwork` is CAA-then-TheAudioDB;
   `LookupArtist` is MusicBrainz-for-identity-then-TheAudioDB-for-bio. Splitting these across
   three scanner call sites re-scatters exactly the logic being consolidated.
2. Ordering policy inside `aggregate.go` is unit-testable against three `httptest.Server`s with
   no database involved. Ordering policy inside a 1375-line scanner is not.
3. One nil check replaces one nil check. Three clients means 2³ partial-availability
   combinations for the scanner to reason about.
4. The rate limiters **must** be shared across every caller of a given provider. Owning them
   inside one client makes that structural rather than conventional.

Per-provider methods stay unexported on the same struct (`c.mbSearchArtist`, `c.caaFrontArt`,
`c.audiodbArtistByMBID`), so they remain independently testable.

### 6.4 Rate limiting and retry

`golang.org/x/time` is **not** a build dependency — `server/go.sum` carries only `/go.mod`
hashes for it, meaning it is a module-graph entry with no module zip. Using
`golang.org/x/time/rate` would require adding a `require` line, which the project rules
discourage. Use the standard library.

A lazily-refilled token bucket with no background goroutine (a `time.Ticker` implementation
would need a lifecycle and a shutdown hook, and would burn wakeups while idle, which is most of
a media server's life):

```go
type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	burst    float64
	tokens   float64
	last     time.Time
}

func (l *Limiter) Wait(ctx context.Context) error   // consumes a token, then sleeps
func (l *Limiter) reserve() time.Duration           // holds the mutex only while accounting
```

The mutex must be released before sleeping so that N concurrent callers each receive a distinct,
correctly-staggered deadline instead of serializing on the lock.

```go
const (
	musicBrainzInterval = 1100 * time.Millisecond // 1 req/s + 10% headroom
	musicBrainzBurst    = 1
	audioDBInterval     = 2 * time.Second         // 30/min
	audioDBBurst        = 5
	coverArtInterval    = 200 * time.Millisecond  // self-imposed politeness
	coverArtBurst       = 5
)
```

The 1100 ms is not arbitrary. MusicBrainz measures against its own clock; network jitter on a
1000 ms cadence produces occasional two-requests-in-one-second windows and a 503.

Retry reuses `tmdb`'s structure verbatim — three attempts, `Retry-After` honoured as integer
seconds or HTTP-date, exponential backoff otherwise, cancellable sleeps — with three deltas:

1. `Limiter.Wait(ctx)` is called **inside** the attempt loop, so retries also consume tokens.
2. `503` is retryable, because that is MusicBrainz's rate-limit response, not a server fault.
3. **`User-Agent` is mandatory**: `Igloo/<version> ( <contact> )`. The repository has no version
   constant today; add `helpers.APP_VERSION` and a `MUSIC_METADATA_CONTACT` env override so
   self-hosters can supply their own contact.

Artwork downloads use a **separate** `http.Client` with a longer timeout (60 s), since a 1200 px
cover from a slow archive.org mirror can exceed the 15 s API timeout.

### 6.5 Schema

The project is pre-production with no migrations, so this is an in-place rewrite of
`server/sqlc/schema.sql`.

```sql
CREATE TABLE IF NOT EXISTS musicians (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  name              TEXT NOT NULL,
  name_key          TEXT NOT NULL UNIQUE,   -- normalized identity key
  sort_name         TEXT NOT NULL,
  summary           TEXT,                   -- TheAudioDB strBiographyEN
  mb_artist_id      TEXT UNIQUE,
  audiodb_artist_id TEXT,
  country           TEXT,
  formed_year       INTEGER,
  thumb             TEXT,
  thumb_source      TEXT CHECK (thumb_source IN ('embedded','folder','coverart','audiodb','remote')),
  created_at        TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS albums (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  title               TEXT NOT NULL,
  sort_title          TEXT NOT NULL,
  album_key           TEXT NOT NULL UNIQUE,  -- normalized identity key, tag-derived only
  album_artist_id     INTEGER,               -- FK, provider-refinable
  musician            TEXT,                  -- display only, no longer part of any key
  is_compilation      BOOLEAN NOT NULL DEFAULT false,
  mb_release_group_id TEXT UNIQUE,
  mb_release_id       TEXT,
  audiodb_album_id    TEXT,
  summary             TEXT,
  release_date        TEXT,
  year                INTEGER,
  total_tracks        INTEGER,
  cover               TEXT,
  cover_source        TEXT CHECK (cover_source IN ('embedded','folder','coverart','audiodb','remote')),
  created_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (album_artist_id) REFERENCES musicians (id) ON DELETE SET NULL ON UPDATE CASCADE
);
```

**On `name_key` versus `COLLATE NOCASE`** (M8):

| | `name TEXT COLLATE NOCASE UNIQUE` | `name_key TEXT UNIQUE` (recommended) |
|---|---|---|
| Query changes | none | `GetMusicianByName` → `GetMusicianByNameKey`; conflict target changes |
| `Beatles` / `beatles` | fixed | fixed |
| `Beyoncé` / `Beyonce` | **not fixed** — NOCASE folds ASCII only | fixed |
| `P!nk` / `Pink`, `AC/DC` / `ACDC` | not fixed | fixed |
| Cost | zero | one column, one Go call per upsert |

Given this library's composition, `NOCASE` is the wrong tool. `name_key` is computed by
promoting `normalizeComparisonText` (`spotify_match.go:118`) to `helpers` — it already does NFD
decomposition, mark stripping, `&`→`and`, punctuation collapsing, and lowercasing, it is already
regression-tested, and reusing it makes the database key identical to the scoring key.

Consequences to write down: `GetMusicianByName` has exactly one call site
(`findExistingMusician`, `music_scanner.go:589`); `UpsertMusician`'s `ON CONFLICT (name)` becomes
`ON CONFLICT (name_key)`; **`name` must not be overwritten by a later track** (first writer wins
for display, so `Beyoncé` is not degraded to whichever spelling was scanned last); FTS search is
unaffected because it indexes `name`; and an empty `name_key` must be rejected in Go, or every
unnameable artist collapses into one row.

**On `album_key`** (M2, M3, M6):

```go
// albumIdentityKey builds the stable identity key for an album row. It is derived
// exclusively from local tags so that a metadata provider can never change which
// tracks belong to which album.
func albumIdentityKey(title, albumArtist string, isCompilation bool) string {
	artistPart := helpers.NormalizeComparisonText(albumArtist)
	if isCompilation {
		artistPart = variousArtistsKey
	}
	return helpers.NormalizeComparisonText(title) + "\x1f" + artistPart
}
```

`NOT NULL` with an empty-string sentinel instead of NULL, which fixes M3 outright.
`albums.musician` is **kept** as a denormalized display string — `GetLatestAlbums`,
`GetAlbumsAlphabetical`, and the FTS index all read it without a join — but it becomes strictly
display-only and part of no key. That alone kills the Hamilton and Lilo & Stitch duplicates.
`GetAlbumByTitleAndMusician` is replaced by `GetAlbumByKey(album_key)`.

**Compilation determination**, in order:

1. `album_artist` tag present and not a Various-Artists sentinel → use it, and key on it.
2. `compilation` tag equals `"1"`, **or** `album_artist` case-insensitively matches
   `Various Artists` / `Various` / `VA` → `is_compilation = true`, key on the VA sentinel,
   `album_artist_id` NULL, `musician` set to `"Various Artists"` for display.
3. MusicBrainz release-group `secondary-types` contains `Compilation`, or the artist credit is
   the Various Artists MBID `89ad4ac3-39f7-470e-963a-56509c546377` → may flip `false` to `true`
   in the enrichment pass, never the reverse.

Rule 1 deliberately outranks the `compilation` flag. This library contains
`Compilations/The Definitive Greatest Hits…` by Trace Adkins with `compilation=1` — a
single-artist greatest-hits record that iTunes merely filed under Compilations. Letting the flag
win would regroup it under Various Artists, which is wrong. Confirming this ordering is an open
question (§12).

**The match table** becomes provider-neutral and per-provider:

```sql
CREATE TABLE IF NOT EXISTS music_metadata_matches (
  entity_type     TEXT NOT NULL CHECK (entity_type IN ('album','musician')),
  entity_id       INTEGER NOT NULL,
  provider        TEXT NOT NULL CHECK (provider IN ('musicbrainz','coverart','audiodb')),
  external_id     TEXT,
  status          TEXT NOT NULL CHECK (status IN ('matched','failed','unmatched','skipped')),
  reason          TEXT,
  score           INTEGER,
  provider_score  INTEGER,
  threshold_value INTEGER,
  candidate_name  TEXT,
  candidate_artist TEXT,
  search_query    TEXT,
  strategy        TEXT,
  error           TEXT,
  attempts        INTEGER NOT NULL DEFAULT 1,
  next_retry_at   TEXT,
  updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (entity_type, entity_id, provider)
);
```

Changes from `music_spotify_matches`: `provider` joins the primary key (three providers now
answer per entity independently); `provider_score` records MusicBrainz's own 0–100 relevance
separately from Igloo's score; `attempts` + `next_retry_at` add backoff, because today a
`failed` row is retried on **every** scan forever — at 1 req/s against a hard-limited API that
is a slow-motion self-inflicted denial of service; `skipped` distinguishes "provider was
disabled" from a real failure, so re-enabling picks those entities up. The `matched`/`unmatched`
finality semantics and the two `AFTER DELETE` triggers are preserved.

**Should `spotify_id` stay?** No — drop `spotify_id`, `spotify_popularity`, and
`spotify_followers` from both tables. The reasoning is decisive: once the scanner stops writing
`albums.spotify_id`, the column is permanently NULL, so `GetAlbumBySpotifyID` — used by
`mapSpotifyAlbumSearchResults` (`spotify_handler.go:188`) to set `already_in_library` — returns
`sql.ErrNoRows` for everything and the flag becomes **always false**. A silently broken feature
is worse than a removed one.

Replace it with a lookup on `album_key`:

```go
key := albumIdentityKey(album.Name, firstArtistName(album.Artists), false)
existing, err := app.Queries.GetAlbumByKey(r.Context(), key)
```

This is *more* accurate than the Spotify-ID check ever was: it flags an album as already in the
library even when the local copy came from a different pressing or a rip, which is exactly what
"Request Album" needs to know. The Spotify handlers keep returning `spotify_id` in their JSON;
it simply stops being persisted.

**Settings:** add `audiodb_api_key TEXT` and `music_metadata_enabled BOOLEAN NOT NULL DEFAULT true`,
and flip `download_images` to default `true`. `spotify_client_id`/`spotify_client_secret` stay —
the Request dialogs need them.

### 6.6 Two-phase scan — the central recommendation

Today `processMusicBatch` runs ffprobe, then network lookups, then a full transaction, serially
per file. At Spotify's generous limits that is survivable. **At MusicBrainz's 1 req/s it is
not**: an inline lookup imposes a one-second floor per track, so 2267 files would take at least
38 minutes even at a 100% cache hit rate, with the database lock held in the same loop.

**Phase A — walk and persist. Local only, no network.**

`runMusicScan` keeps its current structure. `resolveTrackFile` reads tags, computes `name_key`,
`album_key`, and `is_compilation`, resolves Tier 0 MBIDs and Tier 1 local artwork, and returns.
`persistResolvedTrack` writes tracks, musicians, albums, and join rows. **No `Provider` call
anywhere in this phase.** Runtime is ffprobe-bound: a couple of minutes for this library, less
once P1 and P2 are addressed.

**Phase B — enrich. Network, entity-driven.**

A separate `runMusicEnrichment` behind its own `helpers.ScanGuard`, started after Phase A and
also invocable from a new `POST /api/settings/enrich/music`. It is driven **off the entity
tables, not the file walk** — which is precisely what makes it work on an existing database.
`trackUnchanged` is `(path, size)` (M5), so on a rescan of an unchanged library the walk visits
zero files; anything hanging off the walk would never run. The entity query has no such
dependency:

```sql
-- name: ListMusiciansNeedingEnrichment :many
SELECT m.id, m.name, m.name_key, m.mb_artist_id
FROM musicians AS m
LEFT JOIN music_metadata_matches AS mm
  ON mm.entity_type = 'musician' AND mm.entity_id = m.id AND mm.provider = 'musicbrainz'
WHERE mm.entity_id IS NULL
   OR (mm.status IN ('failed','skipped')
       AND (mm.next_retry_at IS NULL OR mm.next_retry_at <= CURRENT_TIMESTAMP))
ORDER BY m.id
LIMIT ?;
```

Benefits beyond rate limiting:

1. **Progress becomes knowable.** The total is a `COUNT(*)` available before the first request.
   Today "N scanned" is only known after the walk finishes (P10).
2. **The library is no longer blocked on the network.** Titles, artists, albums, and playback
   are available minutes after startup; artwork and biographies fill in behind them.
3. **Retry is free.** A failed enrichment is retried by re-running Phase B — no re-walk, no
   ffprobe.
4. **It structurally prevents M2.** Identity is computed in Phase A from tags and is immutable;
   Phase B only ever `UPDATE`s enrichment columns on a known `id`. A provider cannot create,
   split, or merge an entity.
5. **The scan context shrinks.** `spotifyArtistMisses`, `spotifyAlbumMisses`,
   `spotifyMusicianGenresHandled`, `spotifyAlbumGenresHandled` and their clone/merge handling
   (`music_scanner.go:182-244`) all disappear — Phase B visits each entity exactly once by
   construction, so there is nothing to memoize.

Cost for this library at 1.1 s per MusicBrainz request: 325 artists plus 211 albums at roughly
two requests each ≈ 15–20 minutes, plus ~211 CAA fetches (unlimited, ~1 minute) and ~536
TheAudioDB calls at 30/min (~18 minutes). **Budget 30–40 minutes for the first full enrichment**,
once, in the background.

### 6.7 Artwork storage

```
static/albums/{album_id}.jpg
static/musicians/{musician_id}.jpg
```

Both directories already exist at boot (`startup.go:174-182`) and have never been written to.
The extension follows the **sniffed** content type, so the column stores the real filename.

`albums.cover` / `musicians.thumb` store an API path with a cache-busting version:

```
/api/static/albums/12.jpg?v=1754150400
```

`getMediaImageUrl` (`media-image-url.ts:13`) already returns any `/api`-prefixed string
unchanged, and `ServeStaticFiles` reads `chi.URLParam(r, "*")` (`static_handler.go:33`), which
excludes the query string — so the file resolves normally and **no web change is required**. The
`?v=` token is necessary because `ServeStaticFiles` sets `Cache-Control: public, max-age=31536000`
(`:80`) on a stable filename; without it, a re-downloaded cover would never reach a browser that
had already cached the old one. The token is `updated_at` as a Unix timestamp, written in the
same transaction as the file rename. (Content-hashed filenames are cleaner semantically but
require deleting the superseded file and leak one on a crash between rename and commit; `?v=` is
one column write with no orphan risk.)

`cover_source` / `thumb_source` record provenance, which the UI needs for attribution
(TheAudioDB's terms require a linkback) and which makes a future "re-fetch only the low-quality
ones" pass possible.

**The CAA 307 needs no special handling.** `http.Client` follows up to 10 redirects by default
and the CAA → archive.org chain is two or three hops. The only requirement is to **not** set a
custom `CheckRedirect`. The `Content-Type` that matters is on the final response, which is what
`Do` returns.

**Size selection**, first success wins:

```
/release-group/{mbid}/front-500
/release-group/{mbid}/front-250
/release-group/{mbid}/front
```

500 px is the right target — the grid renders at ~200 px CSS and the detail page at ~400 px, and
a 500 px JPEG is 40–90 KB against 300 KB–2 MB for the unsized original. CAA generates the
250/500/1200 thumbnails on ingest but older items can lack them, hence the fallback. A 404 on
all three is `unmatched` with `reason = no_artwork`, final.

**Validation and write:**

1. Cap before reading — reject on `resp.ContentLength > 12 MB`, then read through
   `io.LimitReader(body, cap+1)`. A `ContentLength` of `-1` is normal from archive.org, so the
   `LimitReader` is the real defense.
2. **Sniff, do not trust the header.** `http.DetectContentType(data[:512])` must be one of
   `image/jpeg`, `image/png`, `image/webp`; the sniff decides both acceptance and the extension.
   This is what stops an HTML error page landing on disk as `12.jpg`.
3. Reject payloads under 1 KB — those are placeholders, not covers.
4. **Atomic write**: `os.CreateTemp` in the *destination* directory (a cross-filesystem rename
   returns `EXDEV` and is not atomic), write, `Sync`, `Close`, `Chmod 0644` (`CreateTemp` makes
   0600 and static files must be readable), then `os.Rename`.
5. **Write the file first, update the row second.** A crash between them leaves an orphan file
   that the next enrichment overwrites — harmless. The reverse leaves a row pointing at nothing,
   which renders as a broken image.

**Keep remote URLs as the `download_images = false` path.** This is the cleanest way to make the
currently-dead setting mean something:

| `download_images` | Behavior | `*_source` |
|---|---|---|
| `true` (new default) | download → validate → atomic write → `/api/static/…?v=` | `embedded` / `folder` / `coverart` / `audiodb` |
| `false` | store the upstream URL verbatim | `remote` |

Unlike Spotify CDN URLs, CAA and TheAudioDB URLs are stable and hot-linkable, so the opt-out is
genuinely viable. The default should still flip to `true`: hot-linking on every page view is
impolite, and offline operation is a core self-hosting expectation.

Two refinements to `ServeStaticFiles` while touching it: add `immutable` to the cache directive
(correct now that the URL carries a version), and force the content type for image extensions
rather than relying on `mime.TypeByExtension`, which is host-dependent — this repository already
learned that lesson for video MIME types (see the `VideoMimeTypes` comment in `helpers/files.go`
citing `docs/web-direct-playback-audit.md` §3.2 D1).

---

## 7. Cutover

`database.Prepare(ctx, app.DB)` (`application.go:107`) prepares **every** query at startup, so
schema drift is a hard boot failure, not a runtime error on one endpoint. `InitTables` runs
`Exec(sqlc.Schema)` (`startup.go:112`) and every statement is `CREATE TABLE IF NOT EXISTS`, so
the reshaped `musicians`/`albums` tables will **not** be created on an existing database and the
next line will crash with `no such column: name_key`.

There is no silent-degradation path. A cutover step is mandatory.

**Option 1 — delete `db/igloo.db*` and rescan.** Correct per the no-migrations rule and cheap in
scan time, but the user loses users, settings, devices, playlists, likes, and movie watch
progress.

**Option 2 (recommended) — a one-time, music-only reset in `InitTables`,** run before
`Exec(sqlc.Schema)`. It detects the legacy shape (`SELECT COUNT(*) FROM pragma_table_info('musicians') WHERE name = 'name_key'`)
and, if the old shape is present, drops the music catalog in child-before-parent order —
`music_spotify_matches`, `playlist_tracks`, `user_liked_tracks`, `track_genres`,
`track_musicians`, `musician_genres`, `album_genres`, `musician_albums`, `tracks`, `albums`,
`musicians` — because `DROP TABLE` does not fire `ON DELETE CASCADE`. Everything non-music
survives.

The FTS5 tables and their vocab counterparts (`albums_fts`, `musicians_fts`,
`tracks_search_fts`, `*_fts_vocab`, `schema.sql:737-930`) must be dropped and rebuilt in the same
step, or they retain rowids pointing at deleted rows.

Either way, **the 189 Spotify cover URLs and 298 artist thumbs must go** — they will rot and no
`cover_source` value describes them. Dropping the tables handles it.

**First scan after the change:** a full re-walk (the track index is empty), a full Phase A
rebuild, and a full Phase B enrichment. With `download_images = true` that produces roughly 400
files in `static/`, on the order of 30–50 MB.

---

## 8. Movie and music scanner duplication

`music_scanner.go` (1375 lines) and `movies_scanner.go` (1083 lines) are structurally parallel,
down to the same section-comment banners. Already shared: `helpers.ScanFile`,
`WalkMediaLibraryContext`, `BuildScanIndex`, `ScanIndexUnchanged`, `NormalizedScanCacheKey`,
`ScanGuard`, `scannerBatchSize`, `scanContext()`.

Still duplicated:

| # | Duplication | Locations |
|---|---|---|
| 1 | Entry point | `music_scanner.go:24` / `movies_scanner.go:26` — identical modulo the dir field, guard, and noun |
| 2 | Scan driver | `:41-114` / `:43-118` — ~75 lines each, identical control flow and log format |
| 3 | Batch processor | `:117-145` / `:121-150` |
| 4 | Index loader | `:147-156` / `:152-161` |
| 5 | Scan-context lifecycle (`clone`/`mergeFrom`/`*Unchanged`) | `:192-248` / `:179-205` |
| 6 | Transaction wrapper | `:713-746` / `:562-597` — identical down to the post-commit-eviction comments |
| 7 | Genre memoizer | `:1141` / `:823` — differ only in the `"music"`/`"movie"` literal, and the movie one has a `scan != nil` guard the music one lacks |
| 8 | Trigger handlers | `settings_handler.go:318-341` / `:344-366` |
| 9 | Web cache invalidation | `libraries.tsx:703-730` re-lists twelve music query keys inline instead of delegating to `invalidateMusicLibraryQueries` (`web/src/lib/music-library-cache.ts:33`) — the movie branch *does* delegate |

A generic driver parameterized over `{dirGetter, exts, guard, loadIndex, newCtx, unchanged,
resolve, persist, noun}` would collapse 1–4 and 8; a shared `getOrCreateGenreID(genreType)`
covers 7; item 9 is a one-line fix.

**Caveat.** Per `server/AGENTS.md:37` and the root `AGENTS.md`, this must not become an
over-abstraction. The two scanners are about to *diverge* — the music scanner is gaining a
second phase the movie scanner does not have — so this work belongs **after** §6.6 lands, when
the shared surface is known, not before. Item 9 can be done immediately.

---

## 9. Findings register

| ID | Sev | Summary | Evidence | Phase |
|---|---|---|---|---|
| M1 | High | `ParseDate` lacks RFC 3339; 2157/2267 tracks have no year | `sql_helpers.go:119-141`; live query | 0 |
| M2 | High | Provider rewrites `albums.musician`, which is half the identity key → duplicate albums | `schema.sql:111`; `music_scanner.go:967,995`; live query | 1 |
| M3 | High | `ON CONFLICT (title, musician)` never fires when `musician` is NULL | `schema.sql:111`; SQLite NULL semantics | 1 |
| M4 | High | No orphan cleanup for deleted or moved files, or for empty entities | absence of any `DeleteTrack*` by path | 5 |
| M5 | High | Change detection is `(path, size)`; retags are invisible | `helpers/scanner.go:35-38` | 5 |
| M6 | High | Compilations fracture into one album per artist; `compilation` tag unread | `music_scanner.go:399-402`; `ffprobe_metadata.go:88-102` | 1 |
| M7 | Med | Artist-split decision delegated to a Spotify failure reason | `music_scanner.go:671,689` | 2 |
| M8 | Med | `musicians.name UNIQUE` is case- and diacritic-sensitive | `schema.sql:84`; `music_scanner.go:589` | 1 |
| M9 | Med | Existing entities never refreshed from changed tags | `music_scanner.go:901,983` | 1 |
| M10 | Med | Fabricated artist biographies, wrong for 322/325 rows | `music_scanner.go:1327-1367`; live query | 2 |
| M11 | Med | No artwork pipeline; `download_images` is dead; remote URLs will rot | `music_scanner.go:1369`; `startup.go:174-182` | 4 |
| M12 | Med | Only mp3/flac/m4a; no opus/ogg/wav | `helpers/files.go:15-19`; `schema.sql:120-124` | 6 |
| M13 | Low | Title fallback keeps the extension; `channels` duplicates `channel_layout`; no `sample_rate` | `music_scanner.go:293,305,363-369`; `ffprobe_metadata.go:203` | 0 |
| P1 | High | One transaction and one fsync per file; untuned SQLite pragmas | `music_scanner.go:713`; `startup.go:48-62` | 3 |
| P2 | High | One serial ffprobe fork per file | `ffprobe_metadata.go:216` | 3 |
| P3 | High | Network calls inline in the file walk | `music_scanner.go:494,568` | 3 |
| P4 | High | No rate limiter, no retry, no `Retry-After`; no 429 test | `spotify.go:55`; `spotify_test.go:309` | 0/2 |
| P5 | Med | N+1 DB round trips on a cold scan; `sqlc.slice` query re-parsed per call | `music_scanner.go:457,465,533,541` | 3 |
| P6 | High | Resolve failures counted but never logged; join failures swallowed | `music_scanner.go:128-132,783-845` | 0 |
| P7 | Med | Scan context not threaded into ffprobe; shutdown can block 60 s | `ffprobe_metadata.go:213` | 0 |
| P8 | Low | Cancelled scan logs as completed | `music_scanner.go:113,119` | 0 |
| P9 | Low | FTS triggers lack `OF` clauses; every enrichment write re-indexes | `schema.sql:755,780` vs `:897,915` | 1 |
| P10 | High | No progress reporting; the UI declares the scan finished immediately | `settings_handler.go:318`; `libraries.tsx:318-328` | 3 |

---

## 10. Prioritized action plan

Each phase is independently landable and gated on `make check` (and `make test-openapi` where
routes or response shapes change).

**Phase 0 — quick wins, no schema change.** M1 date formats, M13 title/extension and
`sample_rate`, P6 logging parity with the movie scanner, P7 context threading, P8 cancel
logging, and `spotify.WithRetry(true)` on the client that remains behind the Request dialogs.
Highest value per unit of risk in the whole plan: M1 alone restores year data for 2157 tracks.

**Phase 1 — schema and identity.** `name_key`, `album_key`, `album_artist_id`,
`is_compilation`, the `music_metadata_matches` table, the `OF` clauses on the FTS triggers, and
the cutover from §7. Fixes M2, M3, M6, M8, M9, P9. Nothing here needs the network, so it can
land and be verified before any provider work exists.

**Phase 2 — the `musicmeta` package.** Interface, DTOs, limiter, retry client, the three
providers, the aggregator, and removal of Spotify from the scanner. Fixes M7, M10, P4. The
Spotify handlers and their settings stay untouched.

**Phase 3 — two-phase scan.** Phase A/Phase B split, the enrichment pass, batched transactions,
a resolve worker pool, tuned SQLite pragmas, and a real progress model plus the status endpoint
the UI needs. Fixes P1, P2, P3, P5, P10.

**Phase 4 — artwork.** Embedded extraction, folder lookup, CAA and TheAudioDB download, atomic
local storage, and making `download_images` mean something. Fixes M11.

**Phase 5 — library hygiene.** mtime-based change detection and orphan cleanup. Fixes M4, M5.

**Phase 6 — breadth.** Opus/Ogg/WAV support, the scanner de-duplication from §8, and the
`invalidateScanQueries` delegation. Fixes M12.

Rationale for the ordering: Phase 0 is free and immediately visible. Phase 1 must precede
Phase 2, because enriching entities whose identity is still broken writes correct data onto
duplicate rows. Phase 3 must precede Phase 4, because artwork download at 1 req/s inside the
file walk would be worse than what exists today. Phase 5 is deferred because orphan cleanup is
destructive and should land when the scan's completion semantics are trustworthy (P8, P10).

---

## 11. Test matrix

### 11.1 Existing coverage

| File | Lines | Role after the change |
|---|---|---|
| `server/cmd/api/music_scanner_test.go` | ~2200 | **The primary safety net.** The Spotify stub (`:176-214`) becomes a `musicmeta.Provider` stub; the thirteen `…Spotify…` test names become provider-neutral; the image tests (`:471`, `:530`, `:603`, `:663`, `:948`) shift from URL assertions to `static/` path assertions |
| `server/cmd/internal/spotify/*_test.go` | ~1170 | Stays as-is; the package survives for the Request dialogs. **Add a 429 test** — currently only 500/502 are exercised (`:309`, `:693`, `:708`) |
| `server/cmd/api/spotify_handler_test.go` | — | `already_in_library` assertions move from `GetAlbumBySpotifyID` to `GetAlbumByKey` |
| `server/cmd/internal/ffprobe/ffprobe_metadata_test.go` | — | Extend with MBID, `compilation`, and `attached_pic` fixtures per container |
| `web/src/test/music/*.test.tsx`, `web/e2e/*.spec.ts` | — | Update for the removed `spotify_popularity` / `spotify_followers` fields |

### 11.2 New tests required

| Area | Test |
|---|---|
| Tag parsing | Table-driven fixtures: FLAC Vorbis `MUSICBRAINZ_*`, MP3 `TXXX MusicBrainz * Id`, M4A `----:com.apple.iTunes:*` including the mean-prefix case; `compilation` `0`/`1`; RFC 3339 dates |
| Identity keys | `albumIdentityKey` / `name_key`: case, diacritics, `&`/`and`, punctuation, empty rejection, VA sentinel |
| Compilation grouping | A 3-track VA disc with three different `artist` tags produces **one** album row |
| Duplicate regression | The Hamilton case: two tracks of one album with different `album_artist` spellings produce one row (guards M2 permanently) |
| Limiter | Timing test: 5 requests at a 100 ms interval take ≥ 400 ms; `Wait` returns promptly on context cancel |
| Providers | `httptest.Server`-backed tests per provider: MB search + lookup, MB 503 + `Retry-After`, CAA 307 → image, CAA 404, TheAudioDB `{"artists":null}`, TheAudioDB 429 |
| Aggregator | Fallback ordering: CAA miss → TheAudioDB hit; MB hit + TheAudioDB miss still yields a matched artist |
| Artwork | Sniff rejects an HTML body; size cap rejects an oversized body; atomic write leaves no temp file on failure; the `?v=` token changes on re-download |
| Enrichment pass | `ListMusiciansNeedingEnrichment` respects `next_retry_at`; a `failed` row backs off; Phase B never inserts an entity |
| Orphan cleanup | A cancelled or errored walk prunes **nothing** (the critical negative test) |
| Live integration | `-short`-skipped tests against real MusicBrainz/CAA/TheAudioDB, mirroring `tmdb_integration_test.go` |

### 11.3 Contract obligations

Dropping `spotify_popularity` / `spotify_followers` changes the album and musician response
shapes. That requires updating `docs/openapi.json`, regenerating
`web/src/types/openapi.gen.ts`, editing `web/src/types/music.ts:30,223,224,238`, and either
deleting or repurposing `web/src/components/music/SpotifyPopularity.tsx` and its two call sites
(`musician.$id.tsx:468`, `album.$id.tsx:49`). `make test-openapi` will fail until this is done.

Adding `POST /api/settings/enrich/music` and a scan-status endpoint in Phase 3 carries the same
obligation, plus `403` documentation for the admin routes per `docs/openapi-maintenance.md`.

---

## 12. Open questions

**Q1 — Artist photo regression.** TheAudioDB is the only artist-image source and its coverage
is well below the current 298/325. Options: (a) accept it with a generated initials/monogram
avatar fallback; (b) add fanart.tv as a fourth provider — free, high-quality, MBID-keyed, but
requires a personal API key; (c) accept the gaps. **Recommendation: (a) now, (b) as a
follow-up.**

**Q2 — TheAudioDB key.** Is the $8/month Patreon key acceptable for this deployment, or must
the design assume the shared `123` key permanently? The shared key is 30 req/min *across every
user of that key*, so intermittent 429s and gaps are expected. Everything TheAudioDB provides
must be optional and must never block a match either way.

**Q3 — Compound artists.** The recommended Phase A rule splits only on unambiguous
collaboration markers (`feat.`, `ft.`, `with`, `vs.`, `;`, ` / `) and deliberately does **not**
split on `,` or ` & `, so `Anthony Ramos, Ariana DeBose, … & Original Broadway Cast of "Hamilton"`
stays one musician row. Splitting it correctly requires MusicBrainz's `artist-credit` array,
which means retroactively splitting an existing row and re-pointing `track_musicians` — a
destructive operation that should sit behind an explicit admin action, not a background scan.
**Is under-splitting acceptable for v1?**

**Q4 — Compilation precedence.** §6.5 rule 1 lets a present `album_artist` tag outrank the
`compilation` flag, so `Compilations/The Definitive Greatest Hits…` stays under Trace Adkins
rather than moving to Various Artists. **Confirm this is the desired behavior**, since the
alternative regroups a number of single-artist greatest-hits records.

**Q5 — Attribution.** MusicBrainz core data is CC0 but the project asks for attribution and a
link; TheAudioDB's terms require a visible linkback; CAA images carry third-party rights.
`cover_source` / `thumb_source` make an attribution footer implementable. **Where does it live
in the UI?**

**Q6 — AcoustID.** Explicitly deferred and recommended to stay deferred: it needs a client key,
an extra `fpcalc` binary per platform (which collides with the embedded-payload build for Linux
x64 and macOS ARM64), 3 req/s, *and* fingerprinting of every file. Its only value is files with
missing tags. **Record it as a future option, not a v1 item.**

**Q7 — Cutover choice.** Option 1 (delete the database) or Option 2 (music-only reset) from §7.
Option 2 is recommended but is more code.

---

## 13. Diagnostic changes

**None.** No source file, schema file, or configuration file was modified in the course of this
audit. All measurements were read-only queries against `db/igloo.db` and read-only `ffprobe`
invocations against files in the music library.

---

## 14. Revision history

| Date | Change |
|---|---|
| 2026-08-03 | Initial audit. 13 correctness findings (M1–M13) and 10 performance/reliability findings (P1–P10), of which 4 are reproduced as wrong data in the development database. Provider evaluation across 8 candidates; MusicBrainz + Cover Art Archive + TheAudioDB recommended. Target design, six-phase action plan, test matrix, and 7 open questions. |
