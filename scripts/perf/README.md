# Server performance measurement

Tooling and protocol for capturing repeatable performance baselines of the Go
server, so optimization PRs can report a measured before/after delta.

## Tools

- **k6** (https://k6.io) — scripted, sessionful load scenarios (`soak.js`).
- **oha** (https://github.com/hatoo/oha) — optional, quick single-endpoint checks.
- `sample-rss.sh` — samples RSS, CPU ticks and cumulative write bytes from
  `/proc/<pid>` to CSV.
- `baseline.sh` — orchestrates the scenarios below.
- pprof — build the server with the `pprofdebug` tag (`make dev-profile`);
  endpoints appear under `/api/debug/pprof` (admin session required), e.g.
  `go tool pprof 'http://localhost:8080/api/debug/pprof/profile?seconds=30'`.

## Baseline protocol

Run against a dev server (`make dev`) with the real library. Find the PID with
`pgrep -f igloo-server-dev`. All results land in `scripts/perf/results/`
(gitignored except for committed summary markdowns).

| # | Scenario | Command | Record |
|---|----------|---------|--------|
| 1 | Startup | `baseline.sh startup server/dist/igloo-server-dev` | time to `/api/health` 200, boot write bytes |
| 2 | Idle | `baseline.sh idle <pid> 10` | RSS, CPU%, write-bytes growth over 10 min |
| 3 | Browse | `baseline.sh browse <pid> 5` | request p95s + RSS/CPU envelope, 3 VUs |
| 4 | Streaming | `baseline.sh stream <pid> <movie_id> 1 10` then `... 3 10` | RSS/CPU, segment p95, `time_to_first_segment` |
| 5 | Scan | trigger via Settings → rescan; sample with `sample-rss.sh` | duration + CPU/RSS envelope |
| 6 | Sizes | `baseline.sh sizes` | release/dev binary + payload sizes |

Notes:

- CPU% from the CSV: `100 * Δcpu_ticks / (100 * Δepoch_s)` (CLK_TCK is 100).
- `write_bytes` comes from `/proc/<pid>/io` and is cumulative; growth while
  idle is almost entirely log churn.
- **Caveat:** `/proc/<pid>/io` includes reaped children. When the server
  reaps a killed ffmpeg at HLS teardown, ffmpeg's lifetime IO lands in the
  server's counters as one instantaneous jump — do not read teardown-moment
  bursts as server writes.
- Streaming scenarios need a `MOVIE_ID` that exists in the library; pick one
  from `/api/movies/library`. The HLS scenario starts a real transcode —
  expect CPU load, and give the session ~5 min to self-evict afterwards (the
  script also requests a stop on teardown).
- After each capture, summarize the numbers into a dated markdown in
  `results/` and commit the summary (not the raw CSVs).

## Microbenchmarks

Pure-Go hot paths have `go test` benchmarks (logger, search vocab BK-tree,
frontend asset serving):

```bash
cd server && go test -tags "externalbin sqlite_fts5" -bench . -benchmem -run '^$' ./cmd/internal/logger/ ./cmd/api
```
