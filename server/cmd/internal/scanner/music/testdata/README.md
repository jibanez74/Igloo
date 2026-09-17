These six 0.1-second silent audio fixtures were generated with the repository's
FFmpeg 7.1.4-Jellyfin Linux x64 payload. They contain no copyrighted recordings.

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
