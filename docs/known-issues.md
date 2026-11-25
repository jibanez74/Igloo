# Known issues

Problems found and deliberately left unfixed, each with enough detail to act on it without
rediscovering it. The Playback entry was still open on 2026-11-03.

When a fix lands, delete its entry in the same change. When adding one, put it under the section
it belongs to and cover:

- what happens, and where it was seen (source file, start position, device, date, commit);
- how to reproduce it;
- what is known about the cause, and what is not;
- where to look, or the options for a fix.

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

Left unfixed on 2026-11-03: no library file reaches it, and the fix would delay every copy-video
Dolby resume by the measurement (0.05–0.3 s). Reopen if a library sweep finds audio starting more
than 4 s after video, or a TV resume fails with an `init.mp4` that has no `dec3`.

## Music

### Spotify matching does not use ISRCs

Albums and tracks are matched on Spotify by searching names, never by ISRC. The scanner stores the
ISRC tag (`music_track_metadata.isrc`, since 2026-11-25), but nothing matches on it, and
`spotify_types.go` does not decode `external_ids`, so a track whose title differs from Spotify's (a
translated title, different qualifiers) can stay unpaired, and an album whose title search misses
stays unmatched even though its tracks carry the codes that would identify it. Seen while triaging
leftovers on 2026-11-13.

Not known: whether Spotify still returns `external_ids` to Development Mode apps, which lost
popularity, followers and genres in February 2026; check with a live request first. A search with
the `isrc:` filter may work either way. Options: prefer an ISRC pairing in the link pass
(`linkAlbum` in `scanner/music/retry.go`), and, for an album the title search misses, search one of
its tracks with `isrc:` and take the hit's album.

### AIFF tracks play only in Safari

AIFF (`.aiff`, `.aif`) tracks are scanned and streamed as stored, with `audio/aiff`, and only
Safari decodes AIFF: Chrome and Firefox show the player's playback error. Ogg Vorbis and Opus play
in Chrome and Firefox; whether a given Safari version plays them, and whether the TV app's Media3
player plays AIFF, was not checked. Decided on 2026-11-25 to scan AIFF anyway. Options: convert
AIFF to FLAC when it is streamed (Igloo's FFmpeg build has the FLAC encoder and muxer; seeking in a
converted stream needs thought), or have the web player test `canPlayType` and explain the
failure.

### MP3 and M4A multi-value tags keep only their first value

ffprobe joins the values of a repeated Vorbis comment (FLAC, Opus, Ogg) with `;`, but returns only
the first value of an ID3 or MP4 multi-value tag. An MP3 or M4A `ARTISTS` tag therefore names one
credit and is ignored, so its credits come from splitting the `ARTIST` string, and its MusicBrainz
artist id list holds one id, which pairs only with a lone credit that does not read as several
names: a lone `A & B` credit gets no id rather than A's. ffprobe also
hides ID3's `UFID` frame, where Picard stores an MP3's MusicBrainz recording id. Accepted on
2026-11-25 rather than adding an ID3/MP4 tag reader next to `isobmff`; reproduce with any
Picard-tagged MP3 whose `ARTISTS` holds two names.

### A new or changed album image waits for the next library scan

The library watcher reacts only to audio files, so adding, replacing or deleting a `cover.jpg` (or
any folder image the music worker reads) without touching a track changes nothing until the next
library scan, whose background pass reads every album again. Runs of changes also read only the
albums of the tracks they touched, so a deleted track whose embedded picture was its album's cover
leaves that copy in place until then. Decided on 2026-11-25 for the phase 2 music scanner work
(`server/cmd/internal/scanner/music/localart.go`); reproduce by dropping a `cover.jpg` into a
scanned album folder with the watcher on. Options: let the watcher pass image files of a music
library through to a run of changes that reads only the album art stage, or a per-album Refresh
metadata action (planned for the Fix match phase).
