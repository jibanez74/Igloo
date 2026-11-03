# Known issues

Problems found and deliberately left unfixed, each with enough detail to act on it without
rediscovering it. Every entry below was still open on 2026-11-03.

When a fix lands, delete its entry in the same change. When adding one, put it under the section
it belongs to and cover:

- what happens, and where it was seen (source file, start position, device, date, commit);
- how to reproduce it;
- what is known about the cause, and what is not;
- where to look, or the options for a fix.

## Deployment

### The QA server does not serve subtitle streams yet

The Shield runs IglooTV `dev`, which plays every subtitle under HLS from
`GET .../subtitles/{trackIndex}/stream.mkv`. The server it uses (`192.168.172.241:8080`) was not
redeployed from Igloo `dev`, so that route is missing there: picking a subtitle during an HLS
session shows "Subtitles unavailable for this track" and the movie plays on without it. Direct
play is unaffected. Fix: deploy Igloo `dev` (at or after `ac8a4d1e`) to that server.

## Playback

### A copy-video resume early in a late-audio source writes an unplayable Dolby init.mp4

Explicit E-AC-3/AC-3 audio is padded with silence back to output time zero, so a source whose
audio starts more than a segment after its video still gets a playable `init.mp4` at zero and
after a transcoded seek. A copy-video session past zero is not padded: its audio starts at the
keyframe the seek lands on, below zero, and `first_pts=0` would trim it. When such a session lands
more than one 4 s segment before the source's audio begins, the first segment closes before any
Dolby packet and the fMP4 muxer writes `init.mp4` without `dec3` or `mvex`, which Media3 cannot
play. Only a resume within the first seconds of a late-audio source does this, and no library file
can: on 2026-11-03 an ffprobe sweep of the 432 files in the movies library found the latest audio
start at 2.42 s (`marvel/Spider-Man.Into.the.Spider-Verse.mkv`). Reproduce with a lavfi fixture
whose audio input has `-itsoffset 10`, remuxed with `start=3`. Fix: pad to the measured landing
(`first_pts` of the landing minus the start, in samples), which needs `SeekLanding` to finish
before FFmpeg starts.

## TV app (IglooTV)

### The player's buffer budget is a loading target, not a cap

IglooTV #19 fixed an OutOfMemoryError by asking for a large heap and stopping loading at a third
of it (`playback/media3/PlayerBufferBudget.kt`). `DefaultLoadControl` stops calling for more
loading at that point but does not cap the allocator: a load in progress finishes, and a
subtitle stream reads up to 1 MiB after it was last continued. On the Shield, with PGS subtitles
on, the heap measured 150–168 MB of 512 MB, so the margin is wide there. A device without a
large heap has less room. If an OutOfMemoryError comes back, the next step is a hard bound on
sample allocations.

## Test suite

### A web test run under heavy load reported one unhandled error

After the test and testing-library timeouts were raised (30 s and 10 s), three full `bun run test`
runs beside 18 busy loops (load average 13-38) passed all 1,584 tests. The first of them still
exited 1 with "Errors 1 error", an unhandled error outside any test; its text was not captured,
and the next two runs were clean. If it recurs, capture the full output (`bun run test > log 2>&1`)
and look for the error section above the summary.
