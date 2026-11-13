# Known issues

Problems found and deliberately left unfixed, each with enough detail to act on it without
rediscovering it. The Playback and TV entries were still open on 2026-11-03; the Web client
entries on 2026-11-13 (dev 67011cca).

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

## Web client

Recorded on 2026-11-13 from the web audit of fc27fddc and the review of PRs #267–#270, which
fixed the audit's bugs (BUG-01..07, OBS-01/02) and left these.

### Playing an album from its card can override a newer start or bring the player back

`AlbumCard`'s play button (`web/src/components/music/AlbumCard.tsx`, `handlePlay`) awaits
`queryClient.fetchQuery(albumDetailsQueryOpts(id))` and then calls `audioPlayer.playQueue`
unconditionally. This is the race #267 fixed for Shuffle all and Play all (BUG-01): an album
answer that lands after a newer queue started replaces it, and one that lands after Stop (or a
sign-out, which stops the player) before the login page clears the cache starts the player again.
Reproduce by holding the album-details response, starting Shuffle all, then releasing it. Fix:
give the action a start intent like `startIntentRef` in `AudioPlayerContext` (exposed as an
action), or start the queue through a provider action that checks it after the await.

### A playlist keeps loading its remaining pages after its queue is replaced

`startPlaylistQueue` in `web/src/routes/_auth/music/playlist.$id.tsx` plays the loaded rows, then
`loadRemainingTracks` drains every remaining page with no check that the queue is still the one it
started. `extendQueue` drops the result for a replaced queue, but the requests still run, and the
"Only N of M tracks could be loaded" failure can toast for a queue the reader already left. Fix:
expose a stable `isQueueLive(queueId)` action (reading `liveQueueIdRef`) and stop between pages
once it turns false, and on unmount (audit PERF-03).

### The musician page mounts a row for every track

`web/src/routes/_auth/music/musician.$id.tsx` maps the whole musician-tracks response into
`TrackItem` rows, so a large discography builds every row up front. The main tracks view and long
playlists use `useWindowVirtualizer`, each with its own copy of the setup; a shared window-virtualized
track list (scroll margin, list roles with `aria-posinset`/`aria-setsize`, memoized rows) would
serve all three. Measure a large fixture before and after (audit PERF-02).

### The signed-out login page loads music UI and the full font

`AudioPlayerProvider` sits above the router (`web/src/AppBoot.tsx`) and imports the player UI
eagerly, so `/login` loads the audio-context and volume chunks, and `boot.css` loads the full
`InterVariable.woff2` (352 kB of the ~698 kB measured on a cold production `/login`). Options:
defer the expanded player UI while keeping the provider, or a subset font if the design system's
glyph coverage allows. Set a byte budget from a throttled measurement first (audit PERF-01).

### WebKit video-only fullscreen can stay "on" if the video is torn down inside it

`useVideoFullscreen` listens for `webkitbeginfullscreen`/`webkitendfullscreen` on the document in
the capture phase. If the `<video>` unmounts while in WebKit video-only fullscreen (a playback
error replacing the player while the page stays), the end event may fire on a detached node, never
reach the document, and leave `fullscreenModeRef` at `"webkitVideo"`. Not reproduced: it depends
on iOS Safari behavior. Fix if seen: in `toggleFullscreen`, treat `"webkitVideo"` with no video, or
one not `webkitDisplayingFullscreen`, as `"none"`.

### Some movie wire types are handwritten copies of OpenAPI schemas

`web/src/types/movies.ts` declares `VideoStreamType`, `AudioStreamType`, `SubtitleType`,
`TmdbSearchResultType`, `TmdbSearchMoviesRequest` and `UpdateMovieMetadataRequest` by hand,
though `openapi.gen.ts` has named schemas for each (`VideoStream`, `AudioStream`, `Subtitle`,
`TmdbSearchResult`, …), against `web/AGENTS.md`'s alias-only rule. `check:openapi` cannot catch
drift in these copies. Differences to handle when aliasing: `TmdbSearchMoviesRequest.title` is
optional in the schema, `UpdateMovieMetadataRequest` fields are `?: T | null` there, and
`ExtraVideo` narrows `type`/`site` to enums. Likewise `PlaylistData` (`PlaylistFormDialog.tsx`)
and `OwnedPlaylist` (`PlaylistOwnerActions.tsx`) repeat one editable-playlist shape that could be
one `Pick<components["schemas"]["Playlist"], …>` in `lib/playlist-kinds.ts` (audit DRY-01/02).

## TV app (IglooTV)

### The player's buffer budget is a loading target, not a cap

IglooTV #19 fixed an OutOfMemoryError by asking for a large heap and stopping loading at a third
of it (`playback/media3/PlayerBufferBudget.kt`). `DefaultLoadControl` stops calling for more
loading at that point but does not cap the allocator: a load in progress finishes, and a
subtitle stream reads up to 1 MiB after it was last continued. On the Shield, with PGS subtitles
on, the heap measured 150–168 MB of 512 MB, so the margin is wide there. A device without a
large heap has less room. If an OutOfMemoryError comes back, the next step is a hard bound on
sample allocations.

Left as a watch item on 2026-11-03. Reopen on any OutOfMemoryError in the player, or before
supporting a device without large-heap headroom.
