// AUDIO PLAYER TYPES
// Types for the global audio player state and controls

import type { TrackType } from "./music";
import type { NullableInt64, NullableString } from "./nullable";

// Album info passed to player when starting playback
export type AlbumInfoType = {
  cover: string | null;
  title: string;
  musician: string | null;
};

export type PlayableTrackData = {
  id: number;
  title: string;
  file_path: string;
  duration: number;
  codec: string;
  bit_rate: number;
  album_id: NullableInt64;
  musician_id: NullableInt64;
  album_cover: NullableString;
  musician_name: NullableString;

  // Used to keep the player header accurate in mixed queues. Optional because
  // not every producer of this shape carries it; the list, search, shuffle and
  // play-all endpoints all do.
  album_title?: NullableString;
};

// The queue the player is working through. Held in provider state and passed
// to AudioPlayer as props — it is deliberately NOT exposed as a context, so a
// queue append never re-renders the app shell or a track list.
export type AudioPlayerQueueState = {
  // Bumped every time a queue is started or cleared. An endless-queue batch is
  // fetched against one queueId and appended only if that is still the live
  // one, so a batch that lands late cannot splice itself into a queue the user
  // has since replaced.
  queueId: number;

  currentTrack: TrackType | null;
  tracks: TrackType[];
  albumCover: string | null;
  albumTitle: string;
  musicianName: string | null;

  // indicates if we are playing tracks in shuffle mode
  isShuffleMode: boolean;

  // Whether "play all" mode is enabled (plays through entire library)
  isPlayAllMode: boolean;

  // How many played tracks were trimmed from the front of an endless
  // shuffle/play-all queue; keeps the "Track N of M" counter monotonic
  trimmedCount: number;
};

// The small slice of player state that app chrome and track rows subscribe
// to. Everything here is a primitive that changes only on real playback
// events, so queue growth never re-renders subscribers.
export type AudioPlayerNowPlaying = {
  currentTrackId: number | null;
  isPlaying: boolean;
  isExpanded: boolean;
};

// Actions available for the audio player
export type AudioPlayerActions = {
  // Play a single track within a playlist context
  playTrack: (
    track: TrackType,
    playlist: TrackType[],
    albumInfo: AlbumInfoType
  ) => void;

  // Play a track from a mixed list (search results, library tracks tab),
  // queueing the whole list with per-track cover/musician metadata
  playTrackFromList: (
    rawTracks: PlayableTrackData[],
    startTrackId: number
  ) => void;

  // Start a finite queue (album, playlist, musician page) from the top. Unlike
  // playTrack this always restarts, even when the first track is already the
  // current one — it is the explicit "start over" entry point.
  //
  // rawTracks is optional and only matters for mixed queues (playlists): pass
  // it and the player resolves each track's own cover/artist/album as the queue
  // advances instead of showing the queue-wide albumInfo for every track. Album
  // and musician queues are single-artist, so they omit it on purpose.
  playQueue: (
    tracks: TrackType[],
    albumInfo: AlbumInfoType,
    rawTracks?: PlayableTrackData[]
  ) => void;

  // Shuffle a finite queue and start it from the top. Same rawTracks contract
  // as playQueue.
  shuffleQueue: (
    tracks: TrackType[],
    albumInfo: AlbumInfoType,
    rawTracks?: PlayableTrackData[]
  ) => void;

  // Start shuffle playback across entire music library
  startShufflePlayback: () => Promise<void>;

  // Start "play all" mode - plays through entire library
  startPlayAllPlayback: () => Promise<void>;

  // Change to a specific track (used for next/previous navigation)
  setTrack: (track: TrackType) => void;

  // Stop playback and clear the player
  stop: () => void;

  // Pause playback without clearing the player
  pause: () => void;

  togglePlay: () => void;

  // Expand the player to fullscreen mode
  expand: () => void;

  // Minimize the player to the bottom bar
  minimize: () => void;

  // Suspend/resume global keyboard shortcuts (used when another player has focus)
  suspendKeyboard: () => void;
  resumeKeyboard: () => void;
};
