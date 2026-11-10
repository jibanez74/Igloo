# Server performance and resource pass, 2026-11-09

Branch `perf/server-resources` against `dev` fc27fddc. Target: a household
server, 10–60 accounts, a handful of streams, some viewers outside the house.

Method: both binaries built with the same tags (`externalbin sqlite_fts5
pprofdebug`), server pinned to CPUs 0–7 and `oha` to 8–19, order alternated
per round, AC power. The catalog is the real library ported into the current
schema (426 movies, 726 episodes, 2,267 tracks, 63k people). Scan runs are a
first import of the real library over SMB into an empty database, with TMDB
and Spotify enrichment, while `oha` reads ten list endpoints at 40 req/s.
k6 was not installed, so `oha` stands in for `baseline.sh browse`.

## What shipped

| Change | Measured |
|---|---|
| Session decoded once, not per request (`memoSessionCodec`) | decode 26.2 µs / 190 allocs → 0.34 µs / 2 allocs per cookie request |
| JSON encoded into a pooled buffer | 83 KB → 1.1 KB allocated for a 70 KB response |
| gzip (BestSpeed) for JSON, HLS playlists and WebVTT subtitles of one packet or more | movie details 73.1 → 17.2 KB, tracks 16.8 → 3.6 KB, album details 13.2 → 1.9 KB |
| TMDB response cache capped at 64 entries | scan peak RSS 108 → 70 MB; after the scan 103 → 61 MB |
| Read-only transactions keep the connection on a client abort | test proves the discard and the fix; frequency in real use not measured |
| `journal_size_limit` 16 MiB | WAL 12 MB → 1 MB after a large transaction in a check with a 1 MiB limit |
| Log file 1 MiB → 8 MiB before rotation | about an hour → about half a day of request lines |

## API, server CPU per request (µs, mean of 2 rounds)

Client without `Accept-Encoding`, so both binaries send identical bytes
(verified: 17 of 17 JSON bodies byte-identical).

| Endpoint | c | before | after | ratio |
|---|---|---|---|---|
| auth user | 1 | 109 | 69 | 0.64 |
| unread count | 1 | 103 | 66 | 0.64 |
| movies latest | 1 | 141 | 96 | 0.68 |
| movies library | 1 | 255 | 197 | 0.77 |
| albums | 1 | 249 | 190 | 0.76 |
| album details | 1 | 583 | 487 | 0.84 |
| tracks | 16 | 964 | 768 | 0.80 |
| movie details (73 KB) | 16 | 1916 | 1595 | 0.83 |
| search `love` | 1 | 1059 | 1059 | 1.00 |
| browse mix, 16 URLs | 1 / 8 / 32 | 573 / 633 / 630 | 517 / 574 / 545 | 0.90 / 0.91 / 0.87 |

Under the browse mix the garbage collector's share of CPU went from 23% to
18%, and bytes allocated over the run from 7.46 GB to 6.43 GB.

With `Accept-Encoding: gzip` the new binary compresses:

| | before | after |
|---|---|---|
| browse mix bytes sent, c=32 | 34.4 MB/s at 3,416 req/s | 8.3 MB/s at 3,208 req/s |
| browse mix CPU per request, c=1 / 8 / 32 | 564 / 630 / 637 µs | 554 / 557 / 535 µs |
| movie details CPU per request, c=1 | 1754 µs | 1839 µs (+5%) |
| tracks CPU per request, c=1 | 633 µs | 666 µs (+5%) |

Compression costs about what the other two changes save, so a compressed
browse mix uses no more CPU than the old uncompressed one. BestSpeed was
chosen over the default level: the default gains a tenth more for twice the
time (movie details 17.2 KB at 0.57 ms against 15.4 KB at 1.07 ms on an
efficiency core). The standard library and klauspost produced identical
sizes; the standard library is used.

Over real HTTP against the imported library: the playlist of a two-hour
720p transcode is 245,913 bytes plain and 6,070 compressed, a young remux
playlist 2,121 and 331, a film's WebVTT cues 23,049 and 10,937. Segments,
`init.mp4` and the Matroska subtitle stream carry no `Content-Encoding`,
and a direct-play range request still reaches `sendfile(2)` (strace).

The real-instance Playwright suite (Chromium, which asks for gzip on every
request) against the new binary: 102 passed, 0 failed, 21 skipped, the same
as before the change. `make check` is green.

## Reads during a library scan (the single-connection question)

20 s windows, 40 req/s over ten list endpoints, first import.

| Phase | p50 | p99 | worst read |
|---|---|---|---|
| probing files (most of the scan) | 0.6 ms | 3–10 ms | 20–38 ms |
| TMDB enrichment (about a minute) | 0.6 ms | 38–73 ms before, 52–66 ms after | 244 ms before, 230 ms after |

Reads are not starved: the worst case is a quarter of a second for one
request during the minute the scanner commits its credit transactions. The
2026-08-13 decision to keep one connection stands; a read/write split is not
proposed. That file's note about FIFO hand-off was corrected: `database/sql`
gives a released connection to a random waiter.

## Measured and dropped

- **Skip the boot-time artwork revert when downloads are off:** the ten
  `UPDATE … LIKE` statements take 2.5–4.5 ms in total on this catalog.
- **Fewer full artwork passes (boot catch-ups, movie edits):** a full pass
  with nothing to do lists 21,512 images in 42 ms, plans in 21 ms and holds
  11 MB while it runs. Not worth changing when orphaned copies are pruned.
  It does hold every image row at once (about 500 B each), which would matter
  for a library ten times this size with downloads on.
- **Search typo index memory:** 147 B per term; this library has 9,705 terms
  across the five vocabularies, about 1.4 MB.
- **Middleware consolidation:** `Request.WithContext` copies are 1.5% of
  allocated bytes; nothing in the middleware chain shows in the CPU profile.
- **HLS segment handler trims (ticker, double TTL refresh):** a stream asks
  for one segment every four seconds; not measurable.
- **Draining failed image downloads:** the image CDNs speak HTTP/2, where an
  unread body costs a stream, not a connection.
- **ETag and gzip for `/api/openapi.json`:** no client fetches it in normal
  use.
- **GOGC / GOMEMLIMIT:** raising GOGC would trade memory for the GC share
  above; allocation was reduced instead.

## Not in this pass

HLS run-ahead: FFmpeg encodes or remuxes to the end of the file whatever the
viewer does, every segment stays on disk until the session ends, and idle
permits are reclaimed only from the same user. It needs its own change with
a playback matrix on the TV and browsers.
