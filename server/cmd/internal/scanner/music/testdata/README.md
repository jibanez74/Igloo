These six 0.1-second silent audio fixtures were generated with the FFmpeg
7.1.4-Jellyfin Linux x64 payload the repository pinned then. They contain no
copyrighted recordings. Igloo's own FFmpeg build, pinned now, has no MP3
encoder or muxer: it regenerates the FLAC and M4A pairs, and the MP3 pair takes
an FFmpeg with libmp3lame.

Generation uses `-f lavfi -i anullsrc=r=44100:cl=mono -t 0.1`, with
`-c:a libmp3lame`, `-c:a flac`, or `-c:a aac` for the matching extension.
Each file sets these format tags with `-metadata key=value`:

- title: `initial` or `changed`
- artist: `Artist One, Artist Two, Artist One`
- album: `Metadata Fixture`
- sort_artist: `Z, One & Two, Artist & A, One`
- track: `2`
- language: `eng` initially, `spa` in the changed fixture

MP3 and M4A also set `tracknumber=9` to test alias precedence. FLAC omits it
because the muxer maps it onto the same native field as `track`.
M4A uses `-movflags use_metadata_tags` and `-metadata:s:a:0 language=und`
initially, then `language=zxx` in the changed fixture.

The integration test copies each pair onto one temporary catalog path, probes
with the ffprobe wrapper, and checks initial and changed-file SQLite persistence.
Its injected clock makes each write eligible without sleeping.

The four `tagged.*` fixtures cover the formats Igloo's FFmpeg can neither
encode nor mux (Opus, Ogg Vorbis, WAV, AIFF), so they were made with a system
FFmpeg 4.4.2 (libopus, libvorbis) from `-f lavfi -i anullsrc=cl=mono -t 0.1`:
`-c:a libopus -b:a 16k` at 48 kHz for `.opus`, `-c:a libvorbis -q:a 0` at
44.1 kHz for `.ogg`, and at 8 kHz `-c:a pcm_s16le` for `.wav` and
`-c:a pcm_s16be -write_id3v2 1 -id3v2_version 4` for `.aiff`. Each sets
`title=Tagged`, `artist=Format Band`, `album=Format Test`, `track=2`,
`TRACKTOTAL=3`, `COMPILATION=1`,
`MUSICBRAINZ_ALBUMID=03271bb1-a396-4038-a068-b7d5945c2c37` and
`ARTISTS=Format Band;Guest`. The Ogg muxer writes every tag as a Vorbis
comment on the audio stream, leaving the container's tags empty; the AIFF
muxer writes ID3 frames; the WAV muxer keeps only the RIFF INFO fields
(title, artist, album, track).
