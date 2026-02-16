// AUDIO PLAYER TYPES
// Types for the global audio player state and controls

import type { TrackType } from "./music";

// Album info passed to player when starting playback
export type AlbumInfoType = {
  cover: string | null;
  title: string;
  musician: string | null;
};

// State for the global audio player
export type AudioPlayerState = {
  currentTrack: TrackType | null;
  tracks: TrackType[];
  albumCover: string | null;
  albumTitle: string;
  musicianName: string | null;

  // indicates if we are playing tracks in shuffle mode
  isShuffleMode: boolean;

  // Whether "play all" mode is enabled (plays through entire library)
  isPlayAllMode: boolean;

  // Set of track IDs already played during shuffle (to avoid repeats)
  shufflePlayedIds: Set<number>;
};

// Controls and actions available for the audio player
export type AudioPlayerControls = {
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

  togglePlay: () => void;
  isPlaying: boolean;

  // indicates if the player is in full screen mode or not
  isExpanded: boolean;

  // Expand the player to fullscreen mode
  expand: () => void;

  // Minimize the player to the bottom bar
  minimize: () => void;
};

// Combined audio player context type
export type AudioPlayerContextType = AudioPlayerState & AudioPlayerControls;
