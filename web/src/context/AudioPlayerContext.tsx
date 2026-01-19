import { createContext, useState, useRef, useEffect } from "react";
import type {
  AudioPlayerState,
  AudioPlayerControls,
  AudioPlayerContextType,
} from "@/types";
import AudioPlayer from "@/components/AudioPlayer";
import { getShuffleTracks, getTracksPaginated } from "@/lib/api";
import {
  convertToAudioTrack,
  extractTrackMetadata,
  shuffleArray,
  type PlayableTrackData,
} from "@/lib/audio-utils";

// Initial state - used for both default context and useState
const initialState: AudioPlayerState = {
  currentTrack: null,
  tracks: [],
  albumCover: null,
  albumTitle: "",
  musicianName: null,
  isShuffleMode: false,
  isPlayAllMode: false,
  shufflePlayedIds: new Set(),
};

const defaultContext: AudioPlayerContextType = {
  ...initialState,
  playTrack: () => {},
  playAlbum: () => {},
  shuffleAlbum: () => {},
  startShufflePlayback: async () => {},
  startPlayAllPlayback: async () => {},
  setTrack: () => {},
  stop: () => {},
  togglePlay: () => {},
  isPlaying: false,
  isExpanded: false,
  expand: () => {},
  minimize: () => {},
};

const AudioPlayerContext =
  createContext<AudioPlayerContextType>(defaultContext);

export function AudioPlayerProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [state, setState] = useState<AudioPlayerState>(initialState);
  const [isPlaying, setIsPlaying] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const isFetchingMoreRef = useRef(false);
  const audioRef = useRef<HTMLAudioElement>(null);

  // Maps for special playback modes - track ID to album cover and musician name
  const trackCoversRef = useRef<Map<number, string | null>>(new Map());
  const trackMusiciansRef = useRef<Map<number, string | null>>(new Map());

  // Ref for play all mode pagination
  const playAllOffsetRef = useRef<number>(0);
  const playAllTotalRef = useRef<number>(0);

  // Helper to populate track metadata maps
  const populateTrackMetadata = (tracks: PlayableTrackData[]) => {
    for (const track of tracks) {
      const { cover, musician } = extractTrackMetadata(track);
      trackCoversRef.current.set(track.id, cover);
      trackMusiciansRef.current.set(track.id, musician);
    }
  };

  // Helper to clear all metadata refs
  const clearMetadataRefs = () => {
    trackCoversRef.current.clear();
    trackMusiciansRef.current.clear();
    playAllOffsetRef.current = 0;
    playAllTotalRef.current = 0;
  };

  // Get current track index for auto-fetch logic
  const currentTrackIndex = state.currentTrack
    ? state.tracks.findIndex(t => t.id === state.currentTrack?.id)
    : -1;

  // Auto-fetch more shuffle tracks when queue is running low
  useEffect(() => {
    if (
      state.isShuffleMode &&
      currentTrackIndex >= 0 &&
      state.tracks.length - currentTrackIndex < 5 &&
      !isFetchingMoreRef.current
    ) {
      isFetchingMoreRef.current = true;
      getShuffleTracks(50)
        .then(response => {
          if (!response.error && response.data.tracks.length > 0) {
            const rawTracks = response.data.tracks;
            const newTracks = rawTracks
              .filter(t => !state.shufflePlayedIds.has(t.id))
              .map(convertToAudioTrack);

            populateTrackMetadata(rawTracks);

            if (newTracks.length > 0) {
              setState(prev => ({
                ...prev,
                tracks: [...prev.tracks, ...newTracks],
              }));
            }
          }
        })
        .catch(() => {
          // Silently fail - user can continue with current queue
        })
        .finally(() => {
          isFetchingMoreRef.current = false;
        });
    }
  }, [
    state.isShuffleMode,
    currentTrackIndex,
    state.tracks.length,
    state.shufflePlayedIds,
  ]);

  // Auto-fetch more play all tracks when queue is running low
  useEffect(() => {
    if (
      state.isPlayAllMode &&
      currentTrackIndex >= 0 &&
      state.tracks.length - currentTrackIndex < 10 &&
      !isFetchingMoreRef.current &&
      playAllOffsetRef.current < playAllTotalRef.current
    ) {
      isFetchingMoreRef.current = true;
      getTracksPaginated(50, playAllOffsetRef.current)
        .then(response => {
          if (!response.error && response.data.tracks.length > 0) {
            const rawTracks = response.data.tracks;
            const newTracks = rawTracks.map(convertToAudioTrack);

            populateTrackMetadata(rawTracks);
            playAllOffsetRef.current += rawTracks.length;

            if (newTracks.length > 0) {
              setState(prev => ({
                ...prev,
                tracks: [...prev.tracks, ...newTracks],
              }));
            }
          }
        })
        .catch(() => {
          // Silently fail - user can continue with current queue
        })
        .finally(() => {
          isFetchingMoreRef.current = false;
        });
    }
  }, [state.isPlayAllMode, currentTrackIndex, state.tracks.length]);

  // Play a specific track with a playlist (exits shuffle/play all mode)
  const playTrack: AudioPlayerControls["playTrack"] = (
    track,
    playlist,
    albumInfo
  ) => {
    clearMetadataRefs();
    setState({
      currentTrack: track,
      tracks: playlist,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
      isShuffleMode: false,
      isPlayAllMode: false,
      shufflePlayedIds: new Set(),
    });
    setIsExpanded(true);
  };

  // Play an album starting from the first track
  const playAlbum: AudioPlayerControls["playAlbum"] = (tracks, albumInfo) => {
    if (tracks.length === 0) return;
    clearMetadataRefs();
    setState({
      currentTrack: tracks[0],
      tracks,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
      isShuffleMode: false,
      isPlayAllMode: false,
      shufflePlayedIds: new Set(),
    });
    setIsExpanded(true);
  };

  // Shuffle and play an album
  const shuffleAlbum: AudioPlayerControls["shuffleAlbum"] = (
    tracks,
    albumInfo
  ) => {
    if (tracks.length === 0) return;

    const shuffled = shuffleArray(tracks);

    clearMetadataRefs();
    setState({
      currentTrack: shuffled[0],
      tracks: shuffled,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
      isShuffleMode: false,
      isPlayAllMode: false,
      shufflePlayedIds: new Set(),
    });
    setIsExpanded(true);
  };

  // Start shuffle playback for all tracks in the library
  const startShufflePlayback: AudioPlayerControls["startShufflePlayback"] =
    async () => {
      const response = await getShuffleTracks(50);
      if (!response.error && response.data.tracks.length > 0) {
        const rawTracks = response.data.tracks;
        const tracks = rawTracks.map(convertToAudioTrack);

        clearMetadataRefs();
        populateTrackMetadata(rawTracks);

        const firstTrack = rawTracks[0];
        const { cover, musician } = extractTrackMetadata(firstTrack);

        setState({
          currentTrack: tracks[0],
          tracks,
          albumCover: cover,
          albumTitle: "Shuffle All",
          musicianName: musician,
          isShuffleMode: true,
          isPlayAllMode: false,
          shufflePlayedIds: new Set(),
        });
        setIsExpanded(true);
      }
    };

  // Start play all playback for all tracks in the library (in order)
  const startPlayAllPlayback: AudioPlayerControls["startPlayAllPlayback"] =
    async () => {
      const response = await getTracksPaginated(50, 0);
      if (!response.error && response.data.tracks.length > 0) {
        const rawTracks = response.data.tracks;
        const tracks = rawTracks.map(convertToAudioTrack);

        clearMetadataRefs();
        populateTrackMetadata(rawTracks);

        // Set pagination refs
        playAllOffsetRef.current = rawTracks.length;
        playAllTotalRef.current = response.data.total;

        const firstTrack = rawTracks[0];
        const { cover, musician } = extractTrackMetadata(firstTrack);

        setState({
          currentTrack: tracks[0],
          tracks,
          albumCover: cover,
          albumTitle: "All Tracks",
          musicianName: musician,
          isShuffleMode: false,
          isPlayAllMode: true,
          shufflePlayedIds: new Set(),
        });
        setIsExpanded(true);
      }
    };

  // Change to a different track (used by prev/next)
  // Track played IDs when in shuffle mode and update album cover/musician
  const setTrack: AudioPlayerControls["setTrack"] = track => {
    setState(prev => {
      // If in shuffle mode, add the current track to played IDs
      const newPlayedIds =
        prev.isShuffleMode && prev.currentTrack
          ? new Set(prev.shufflePlayedIds).add(prev.currentTrack.id)
          : prev.shufflePlayedIds;

      // Update album cover and musician name in special playback modes
      const isSpecialMode = prev.isShuffleMode || prev.isPlayAllMode;
      const newAlbumCover = isSpecialMode
        ? (trackCoversRef.current.get(track.id) ?? null)
        : prev.albumCover;

      const newMusicianName = isSpecialMode
        ? (trackMusiciansRef.current.get(track.id) ?? null)
        : prev.musicianName;

      return {
        ...prev,
        currentTrack: track,
        shufflePlayedIds: newPlayedIds,
        albumCover: newAlbumCover,
        musicianName: newMusicianName,
      };
    });
  };

  // Stop playback and clear the player
  const stop: AudioPlayerControls["stop"] = () => {
    clearMetadataRefs();
    setState(initialState);
    setIsPlaying(false);
    setIsExpanded(false);
  };

  // Toggle play/pause
  const togglePlay = () => {
    if (!audioRef.current) return;

    if (isPlaying) {
      audioRef.current.pause();
    } else {
      audioRef.current.play();
    }
  };

  // Expand player to fullscreen
  const expand = () => {
    setIsExpanded(true);
  };

  // Minimize player to bottom bar
  const minimize = () => {
    setIsExpanded(false);
  };

  // Handle play state changes from AudioPlayer
  const handlePlayStateChange = (playing: boolean) => {
    setIsPlaying(playing);
  };

  const contextValue: AudioPlayerContextType = {
    ...state,
    playTrack,
    playAlbum,
    shuffleAlbum,
    startShufflePlayback,
    startPlayAllPlayback,
    setTrack,
    stop,
    togglePlay,
    isPlaying,
    isExpanded,
    expand,
    minimize,
  };

  return (
    <AudioPlayerContext.Provider value={contextValue}>
      {children}
      <AudioPlayer
        track={state.currentTrack}
        tracks={state.tracks}
        albumCover={state.albumCover}
        albumTitle={state.albumTitle}
        musicianName={state.musicianName}
        onTrackChange={setTrack}
        onClose={stop}
        audioRef={audioRef}
        isPlaying={isPlaying}
        onPlayStateChange={handlePlayStateChange}
        isExpanded={isExpanded}
        onMinimize={minimize}
        onExpand={expand}
      />
    </AudioPlayerContext.Provider>
  );
}

// Export the context for use in router and hooks
export { AudioPlayerContext };
