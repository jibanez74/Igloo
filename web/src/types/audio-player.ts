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
  musician_id?: NullableInt64;
  album_cover?: NullableString;
  musician_name?: NullableString;
};

// State for the global audio player
export type AudioPlayerState = {
  currentTrack: TrackType | null;
  tracks: TrackType[];
  albumCover: string | null;
  albumTitle: string;
  musicianName: string | null;
  isPlaying: boolean;
  isExpanded: boolean;
  isKeyboardSuspended: boolean;

  // indicates if we are playing tracks in shuffle mode
  isShuffleMode: boolean;

  // Whether "play all" mode is enabled (plays through entire library)
  isPlayAllMode: boolean;

  // Set of track IDs already played during shuffle (to avoid repeats)
  shufflePlayedIds: Set<number>;
};

// Actions available for the audio player
export type AudioPlayerActions = {
  // Play a single track within a playlist context
  playTrack: (
    track: TrackType,
    playlist: TrackType[],
    albumInfo: AlbumInfoType
  ) => void;

  // Play an entire album from the beginning
  playAlbum: (tracks: TrackType[], albumInfo: AlbumInfoType) => void;

  // Shuffle and play an album's tracks
  shuffleAlbum: (tracks: TrackType[], albumInfo: AlbumInfoType) => void;

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

// Combined audio player context type
export type AudioPlayerContextType = AudioPlayerState & AudioPlayerActions;
