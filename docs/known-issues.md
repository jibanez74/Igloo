# Known issues

Problems found while building and testing subtitle streams for the TV (Igloo #212, IglooTV #18
and #19, 2026-10-30 to 2026-11-01) and while checking the Rust port of the server against this
one (2026-11-02, at `37454e22`), and deliberately left unfixed, with enough detail to act on
them without rediscovering them. Delete an entry when it lands.

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
Media3 cannot play it. It happens at any start, including zero. The library was not searched for
such files; a lavfi fixture with `-itsoffset 10` on the audio input reproduces it. Options: pad the
encoded audio with silence back to the video's first timestamp, or delay the init segment until
every track has a packet.

### Seeking in Oppenheimer reads the file linearly

`historical/Oppenheimer.2023.mkv` in the movies library has a SeekHead whose Cues entry points
past the end of the file. Without a usable index, every seek FFmpeg makes reads the file from the
start up to the target, so HLS sessions that start past zero are slow to begin. This is
the file's fault; remuxing it (for example `mkvmerge -o fixed.mkv Oppenheimer.2023.mkv`) writes a
correct index.

## Subtitles

### Starting playback with a subtitle opens its stream twice

When the subtitle chosen before playback is not one Media3 would pick on its own (no matching
preferred language or default flag), Media3 first prepares the merged source without it. The
stream it has started loading is then cancelled, and the TV's track override restarts it from
byte 0. The server sees two requests at every such start: a short first one (767 bytes for Free
Guy, 15 KB for Captain Marvel) and then the real stream. Each one starts FFmpeg and takes one of
the server's 8 stream slots. Fix: make Media3 select the merged subtitle on the first selection,
for example by setting the chosen track's language as the preferred text language before
`prepare`, and check the server log for a single request.

### A cancelled subtitle stream stays open up to 15 seconds

Media3 cancels a load by setting a flag, but the loader thread stays blocked in the socket read
until data arrives. While FFmpeg is reading toward a sparse track's next cue, the next data is
the server's Void keepalive, written after 15 seconds of silence. Until then, the server keeps
FFmpeg running and the stream slot taken. The first Free Guy request above lasted 15.4 seconds
for 767 bytes. With the double start above, every start can briefly hold two slots. Options: a
shorter keepalive interval (`subtitleStreamKeepalive`), or a TV data source that closes its
connection when the load is cancelled.

## TV app (IglooTV)

### The Chapters dialog shows no D-pad focus and may let focus escape

On the Shield, no row in the Chapters dialog showed a focus highlight while moving with the
D-pad. After pressing Up from Done and then Select, the player closed, as if focus had left the
dialog for the player's Back button. Taps on chapter rows did not select a chapter either,
while taps work in the Subtitles and Quality dialogs. This was not confirmed with a debug build.
Check that the dialog traps focus and that rows draw a focus indicator
(`ChapterMenuDialog` in `feature/player/VideoPlayerScreen.kt`).

### The player's buffer budget is a loading target, not a cap

IglooTV #19 fixed an OutOfMemoryError by asking for a large heap and stopping loading at a third
of it (`playback/media3/PlayerBufferBudget.kt`). `DefaultLoadControl` stops calling for more
loading at that point but does not cap the allocator: a load in progress finishes, and a
subtitle stream reads up to 1 MiB after it was last continued. On the Shield, with PGS subtitles
on, the heap measured 150–168 MB of 512 MB, so the margin is wide there. A device without a
large heap has less room. If an OutOfMemoryError comes back, the next step is a hard bound on
sample allocations.

## Test suite

### Web tests time out under heavy machine load

With another job using about 18 cores (load average 18–20), `make test-web` failed: first two
tab-transition tests (`movies-route.test.tsx`, `settings-route-transition.test.tsx`), then 97
tests across 15 files, all timing out at 10–15 seconds. The same files pass on their own, and the
whole suite passes with fewer workers: `cd web && bun run test --maxWorkers=3`. Neither CI run was
affected. On 2026-11-02, at load average 12, `music playlist route > renders the header, the
owner actions and the tracks` (`src/test/music/playlist-route.test.tsx`) timed out the same way,
after 11 seconds of waiting for the text "Alabaster". It then passed 5 runs alone and 2 full-suite
runs. Options: cap vitest's workers in the config, or raise the timeouts of the transition tests
and this one.
