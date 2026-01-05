/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useState,
  useRef,
  useCallback,
} from "react";
import type {
  AudioPlayerState,
  AudioPlayerControls,
  AudioPlayerContextType,
} from "@/types";
import AudioPlayer from "@/components/AudioPlayer";

// Default context value
const defaultContext: AudioPlayerContextType = {
  currentTrack: null,
  tracks: [],
  albumCover: null,
  albumTitle: "",
  musicianName: null,
  playTrack: () => {},
  playAlbum: () => {},
  shuffleAlbum: () => {},
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
  const [state, setState] = useState<AudioPlayerState>({
    currentTrack: null,
    tracks: [],
    albumCover: null,
    albumTitle: "",
    musicianName: null,
  });
  const [isPlaying, setIsPlaying] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const audioRef = useRef<HTMLAudioElement>(null);

  // Play a specific track with a playlist
  const playTrack: AudioPlayerControls["playTrack"] = (
    track,
    playlist,
    albumInfo
  ) => {
    setState({
      currentTrack: track,
      tracks: playlist,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
    });
    setIsExpanded(true);
  };

  // Play an album starting from the first track
  const playAlbum: AudioPlayerControls["playAlbum"] = (tracks, albumInfo) => {
    if (tracks.length === 0) return;
    setState({
      currentTrack: tracks[0],
      tracks,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
    });
    setIsExpanded(true);
  };

  // Shuffle and play an album
  const shuffleAlbum: AudioPlayerControls["shuffleAlbum"] = (
    tracks,
    albumInfo
  ) => {
    if (tracks.length === 0) return;

    // Fisher-Yates shuffle algorithm
    const shuffled = [...tracks];
    for (let i = shuffled.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
    }

    setState({
      currentTrack: shuffled[0],
      tracks: shuffled,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
    });
    setIsExpanded(true);
  };

  // Change to a different track (used by prev/next)
  const setTrack: AudioPlayerControls["setTrack"] = track => {
    setState(prev => ({
      ...prev,
      currentTrack: track,
    }));
  };

  // Stop playback and clear the player
  const stop: AudioPlayerControls["stop"] = () => {
    setState({
      currentTrack: null,
      tracks: [],
      albumCover: null,
      albumTitle: "",
      musicianName: null,
    });
    setIsPlaying(false);
    setIsExpanded(false);
  };

  // Toggle play/pause
  const togglePlay = useCallback(() => {
    if (!audioRef.current) return;

    if (isPlaying) {
      audioRef.current.pause();
    } else {
      audioRef.current.play();
    }
  }, [isPlaying]);

  // Expand player to fullscreen
  const expand = useCallback(() => {
    setIsExpanded(true);
  }, []);

  // Minimize player to bottom bar
  const minimize = useCallback(() => {
    setIsExpanded(false);
  }, []);

  // Handle play state changes from AudioPlayer
  const handlePlayStateChange = useCallback((playing: boolean) => {
    setIsPlaying(playing);
  }, []);

  const contextValue: AudioPlayerContextType = {
    ...state,
    playTrack,
    playAlbum,
    shuffleAlbum,
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

// Hook to access the audio player context
export function useAudioPlayer() {
  const context = useContext(AudioPlayerContext);
  if (!context) {
    throw new Error(
      "useAudioPlayer must be used within an AudioPlayerProvider"
    );
  }
  return context;
}

// Export the context for use in router
export { AudioPlayerContext };
