# HLS limiter: blocking acquire — 2026-08-13

Measured on the same hardware, library, and protocol as
`hls-cold-start-2026-08-13.md`, with the limiter forced to the baseline's
default size: `HLS_MAX_CPU_TRANSCODES=2`, dev build, k6 `stream-hls`, movies
59/146/203 as three concurrent 1-VU runs.

## What changed

`fix/hls-limiter-blocking-acquire`: `hlsTranscodeLimiter.acquire(ctx, wait)`
parks on the permit channel after the non-blocking fast path, and the
post-reclaim retry in `GetOrCreateHLSSession` now runs unconditionally with a
15 s budget (`hlsTranscodeAcquireWait`). Previously the retry ran only when
same-owner LRU reclaim found an idle victim, so a pool held entirely by
streams that were genuinely playing refused new sessions forever.

## Three concurrent streams at 2 slots (5 min)

| | Before (cold-start branch) | After |
|---|---|---|
| Streams 1–2 | start, 100% checks | start, 100% checks |
| Stream 3 | 60 × `503`, never starts | 15 × `503`, never starts |
| Stream 3 rejection cadence | instant, on every poll | one per 15 s wait |

Stream 3 still does not start here, and that is the correct answer: two 1-VU
streams hold both permits for the entire 5 minutes and never go idle, so
there is no permit to hand over. Two slots means two concurrent transcodes.
What changed is that each refusal now costs a real wait — the request is
parked and admitted the instant capacity appears, rather than being told
"busy" while the server does nothing to seat it.

k6 records the stream-3 run as a threshold failure (`checks: rate==1`, 1 of
16 succeeded). That is the saturated-pool outcome, not a regression.

## Permit handoff (the measurement the run above cannot produce)

Both slots filled, a third stream requested, then one holding stream stopped
3 s later:

```
20:02:03.641  hls transcode limiter rejected      active=2 max=2 waited_ms=0
20:02:03.641  hls limiter reclaim found no idle session   movie_id=203
20:02:06.675  hls session stopped                 movie_id=59  (permit freed)
20:02:06.675  hls session starting                movie_id=203 (admitted)
```

The parked request was seated in the **same millisecond** the permit freed,
and the manifest returned `200` in 3.04 s total — the 3 s scripted wait plus
~40 ms. No `waited_ms=15000` line follows the fast-path rejection, because
the retry never reached its budget. Before this change the same sequence
returned `503` immediately and the third stream depended on the other two
going 30 s idle, which an actively playing stream never does.

Cold-start numbers from `hls-cold-start-2026-08-13.md` are unaffected: single
stream still serves its first segment in ~1.4 s, and streams 1–2 above showed
`ttfs_ms` of 2,031 and 5,129 under the 2-slot contention.
