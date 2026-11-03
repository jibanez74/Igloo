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

### A source whose audio starts late writes an unplayable Dolby init.mp4

When a source's audio track starts more than one HLS segment after its video (an audio delay of
several seconds), explicit E-AC-3/AC-3 output has no audio packet by the time the first segment
closes. The fMP4 muxer then writes `init.mp4` without `dec3` or `mvex` ("Cannot write moov atom
before EAC3 packets parsed"), as resumed copy-video sessions did before `-noaccurate_seek`, and
Media3 cannot play it. It happens at any start, including zero; a lavfi fixture with `-itsoffset 10`
on the audio input reproduces it. No library file is affected: on 2026-11-03 an ffprobe sweep of the
432 files in the movies library found 3 whose audio starts more than 0.5 s after the video, the latest
by 2.42 s (`marvel/Spider-Man.Into.the.Spider-Verse.mkv`), inside the 4 s first segment. Options: pad
the encoded audio with silence back to the video's first timestamp, or delay the init segment until
every track has a packet.

### Seeking in Oppenheimer reads the file linearly

`historical/Oppenheimer.2023.mkv` in the movies library has a SeekHead whose Cues entry points
past the end of the file. Without a usable index, every seek FFmpeg makes reads the file from the
start up to the target, so HLS sessions that start past zero are slow to begin. This is
the file's fault; remuxing it (for example `mkvmerge -o fixed.mkv Oppenheimer.2023.mkv`) writes a
correct index.

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
