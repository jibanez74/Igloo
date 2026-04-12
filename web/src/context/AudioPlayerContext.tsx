import {
  createContext,
  useState,
  useRef,
  useEffect,
  useCallback,
  useMemo,
} from "react";
import type {
  AudioPlayerActions,
  AudioPlayerState,
} from "@/types";
import AudioPlayer from "@/components/AudioPlayer";
import {
  getShuffleTracks,
  getTracksPaginated,
  recordPlayEvent,
} from "@/lib/api";
import {
  convertToAudioTrack,
  extractTrackMetadata,
  shuffleArray,
  type PlayableTrackData,
} from "@/lib/audio-utils";

const MINIMUM_PLAY_SECONDS = 30;
const COMPLETION_THRESHOLD = 0.8;
const PLAY_CHECK_INTERVAL_MS = 5000;

type QueueState = Omit<
  AudioPlayerState,
  "isPlaying" | "isExpanded" | "isKeyboardSuspended"
>;

function createInitialQueueState(): QueueState {
  return {
    currentTrack: null,
    tracks: [],
    albumCover: null,
    albumTitle: "",
    musicianName: null,
    isShuffleMode: false,
    isPlayAllMode: false,
    shufflePlayedIds: new Set(),
  };
}

const AudioPlayerStateContext = createContext<AudioPlayerState | null>(null);
const AudioPlayerActionsContext = createContext<AudioPlayerActions | null>(null);

export function AudioPlayerProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [queueState, setQueueState] = useState<QueueState>(() =>
    createInitialQueueState(),
  );
  const [isPlaying, setIsPlaying] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const [keyboardSuspendCount, setKeyboardSuspendCount] = useState(0);
  const isFetchingMoreRef = useRef(false);
  const audioRef = useRef<HTMLAudioElement>(null);

  const playStartTimeRef = useRef<number | null>(null);
  const hasRecordedPlayRef = useRef(false);
  const currentTrackIdRef = useRef<number | null>(null);

  const trackCoversRef = useRef<Map<number, string | null>>(new Map());
  const trackMusiciansRef = useRef<Map<number, string | null>>(new Map());

  const playAllOffsetRef = useRef(0);
  const playAllTotalRef = useRef(0);

  const populateTrackMetadata = useCallback((tracks: PlayableTrackData[]) => {
    for (const track of tracks) {
      const { cover, musician } = extractTrackMetadata(track);
      trackCoversRef.current.set(track.id, cover);
      trackMusiciansRef.current.set(track.id, musician);
    }
  }, []);

  const clearMetadataRefs = useCallback(() => {
    trackCoversRef.current.clear();
    trackMusiciansRef.current.clear();
    playAllOffsetRef.current = 0;
    playAllTotalRef.current = 0;
  }, []);

  const playAudio = useCallback(async () => {
    const audio = audioRef.current;
    if (!audio) return;

    try {
      await audio.play();
    } catch {
      // Playback can still be blocked by the browser in some cases.
    }
  }, []);

  const currentTrackIndex = queueState.currentTrack
    ? queueState.tracks.findIndex(track => track.id === queueState.currentTrack?.id)
    : -1;

  useEffect(() => {
    const shouldFetchMore =
      queueState.isShuffleMode &&
      currentTrackIndex >= 0 &&
      queueState.tracks.length - currentTrackIndex < 5 &&
      !isFetchingMoreRef.current;

    if (!shouldFetchMore) {
      return;
    }

    let isCancelled = false;

    const fetchMoreShuffleTracks = async () => {
      isFetchingMoreRef.current = true;

      try {
        const response = await getShuffleTracks(50);

        if (
          isCancelled ||
          response.error ||
          response.data.tracks.length === 0
        ) {
          return;
        }

        const rawTracks = response.data.tracks;
        const newTracks = rawTracks
          .filter(track => !queueState.shufflePlayedIds.has(track.id))
          .map(convertToAudioTrack);

        populateTrackMetadata(rawTracks);

        if (newTracks.length > 0) {
          setQueueState(prev => ({
            ...prev,
            tracks: [...prev.tracks, ...newTracks],
          }));
        }
      } catch {
        // Silently fail - user can continue with the current queue.
      } finally {
        isFetchingMoreRef.current = false;
      }
    };

    void fetchMoreShuffleTracks();

    return () => {
      isCancelled = true;
    };
  }, [
    queueState.isShuffleMode,
    currentTrackIndex,
    queueState.tracks.length,
    queueState.shufflePlayedIds,
    populateTrackMetadata,
  ]);

  useEffect(() => {
    const shouldFetchMore =
      queueState.isPlayAllMode &&
      currentTrackIndex >= 0 &&
      queueState.tracks.length - currentTrackIndex < 10 &&
      !isFetchingMoreRef.current &&
      playAllOffsetRef.current < playAllTotalRef.current;

    if (!shouldFetchMore) {
      return;
    }

    let isCancelled = false;

    const fetchMorePlayAllTracks = async () => {
      isFetchingMoreRef.current = true;

      try {
        const response = await getTracksPaginated(50, playAllOffsetRef.current);

        if (
          isCancelled ||
          response.error ||
          response.data.tracks.length === 0
        ) {
          return;
        }

        const rawTracks = response.data.tracks;
        const newTracks = rawTracks.map(convertToAudioTrack);

        populateTrackMetadata(rawTracks);
        playAllOffsetRef.current += rawTracks.length;

        if (newTracks.length > 0) {
          setQueueState(prev => ({
            ...prev,
            tracks: [...prev.tracks, ...newTracks],
          }));
        }
      } catch {
        // Silently fail - user can continue with the current queue.
      } finally {
        isFetchingMoreRef.current = false;
      }
    };

    void fetchMorePlayAllTracks();

    return () => {
      isCancelled = true;
    };
  }, [
    queueState.isPlayAllMode,
    currentTrackIndex,
    queueState.tracks.length,
    populateTrackMetadata,
  ]);

  useEffect(() => {
    const trackId = queueState.currentTrack?.id ?? null;

    if (trackId !== currentTrackIdRef.current) {
      currentTrackIdRef.current = trackId;
      hasRecordedPlayRef.current = false;
      playStartTimeRef.current = isPlaying && trackId ? Date.now() : null;
    }

    if (isPlaying && trackId && !playStartTimeRef.current) {
      playStartTimeRef.current = Date.now();
    } else if (!isPlaying) {
      playStartTimeRef.current = null;
    }

    if (!isPlaying || !trackId || hasRecordedPlayRef.current) {
      return;
    }

    const interval = setInterval(() => {
      const audio = audioRef.current;
      const startTime = playStartTimeRef.current;

      if (!audio || !trackId || !startTime || hasRecordedPlayRef.current) {
        return;
      }

      const elapsedSeconds = (Date.now() - startTime) / 1000;
      const progress =
        audio.duration > 0 ? audio.currentTime / audio.duration : 0;
      const isCompleted = progress >= COMPLETION_THRESHOLD;

      if (elapsedSeconds >= MINIMUM_PLAY_SECONDS || isCompleted) {
        hasRecordedPlayRef.current = true;

        const sendPlayEvent = async () => {
          try {
            await recordPlayEvent(
              trackId,
              Math.floor(elapsedSeconds),
              isCompleted,
            );
          } catch {
            // Silently fail - don't interrupt playback for stats.
          }
        };

        void sendPlayEvent();
      }
    }, PLAY_CHECK_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [isPlaying, queueState.currentTrack?.id]);

  const playTrack: AudioPlayerActions["playTrack"] = useCallback(
    (track, playlist, albumInfo) => {
      clearMetadataRefs();
      setQueueState({
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
    },
    [clearMetadataRefs],
  );

  const playAlbum: AudioPlayerActions["playAlbum"] = useCallback(
    (tracks, albumInfo) => {
      if (tracks.length === 0) return;

      clearMetadataRefs();
      setQueueState({
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
    },
    [clearMetadataRefs],
  );

  const shuffleAlbum: AudioPlayerActions["shuffleAlbum"] = useCallback(
    (tracks, albumInfo) => {
      if (tracks.length === 0) return;

      const shuffled = shuffleArray(tracks);

      clearMetadataRefs();
      setQueueState({
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
    },
    [clearMetadataRefs],
  );

  const startShufflePlayback: AudioPlayerActions["startShufflePlayback"] =
    useCallback(async () => {
      const response = await getShuffleTracks(50);
      if (response.error || response.data.tracks.length === 0) {
        return;
      }

      const rawTracks = response.data.tracks;
      const tracks = rawTracks.map(convertToAudioTrack);

      clearMetadataRefs();
      populateTrackMetadata(rawTracks);

      const firstTrack = rawTracks[0];
      const { cover, musician } = extractTrackMetadata(firstTrack);

      setQueueState({
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
    }, [clearMetadataRefs, populateTrackMetadata]);

  const startPlayAllPlayback: AudioPlayerActions["startPlayAllPlayback"] =
    useCallback(async () => {
      const response = await getTracksPaginated(50, 0);
      if (response.error || response.data.tracks.length === 0) {
        return;
      }

      const rawTracks = response.data.tracks;
      const tracks = rawTracks.map(convertToAudioTrack);

      clearMetadataRefs();
      populateTrackMetadata(rawTracks);

      playAllOffsetRef.current = rawTracks.length;
      playAllTotalRef.current = response.data.total;

      const firstTrack = rawTracks[0];
      const { cover, musician } = extractTrackMetadata(firstTrack);

      setQueueState({
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
    }, [clearMetadataRefs, populateTrackMetadata]);

  const setTrack: AudioPlayerActions["setTrack"] = useCallback(track => {
    setQueueState(prev => {
      const newPlayedIds =
        prev.isShuffleMode && prev.currentTrack
          ? new Set(prev.shufflePlayedIds).add(prev.currentTrack.id)
          : prev.shufflePlayedIds;

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
  }, []);

  const stop: AudioPlayerActions["stop"] = useCallback(() => {
    clearMetadataRefs();
    setQueueState(createInitialQueueState());
    setIsPlaying(false);
    setIsExpanded(false);
  }, [clearMetadataRefs]);

  const pause = useCallback(() => {
    audioRef.current?.pause();
  }, []);

  const togglePlay = useCallback(() => {
    const audio = audioRef.current;
    if (!audio) return;

    if (audio.paused) {
      void playAudio();
      return;
    }

    audio.pause();
  }, [playAudio]);

  const suspendKeyboard = useCallback(() => {
    setKeyboardSuspendCount(count => count + 1);
  }, []);

  const resumeKeyboard = useCallback(() => {
    setKeyboardSuspendCount(count => Math.max(0, count - 1));
  }, []);

  const expand = useCallback(() => {
    setIsExpanded(true);
  }, []);

  const minimize = useCallback(() => {
    setIsExpanded(false);
  }, []);

  const handlePlayStateChange = useCallback((playing: boolean) => {
    setIsPlaying(playing);
  }, []);

  const isKeyboardSuspended = keyboardSuspendCount > 0;

  const stateValue = useMemo(
    () => ({
      ...queueState,
      isPlaying,
      isExpanded,
      isKeyboardSuspended,
    }),
    [queueState, isPlaying, isExpanded, isKeyboardSuspended],
  );

  const actionsValue = useMemo(
    () => ({
      playTrack,
      playAlbum,
      shuffleAlbum,
      startShufflePlayback,
      startPlayAllPlayback,
      setTrack,
      stop,
      pause,
      togglePlay,
      expand,
      minimize,
      suspendKeyboard,
      resumeKeyboard,
    }),
    [
      playTrack,
      playAlbum,
      shuffleAlbum,
      startShufflePlayback,
      startPlayAllPlayback,
      setTrack,
      stop,
      pause,
      togglePlay,
      expand,
      minimize,
      suspendKeyboard,
      resumeKeyboard,
    ],
  );

  return (
    <AudioPlayerStateContext.Provider value={stateValue}>
      <AudioPlayerActionsContext.Provider value={actionsValue}>
        {children}
        <AudioPlayer
          track={queueState.currentTrack}
          tracks={queueState.tracks}
          albumCover={queueState.albumCover}
          albumTitle={queueState.albumTitle}
          musicianName={queueState.musicianName}
          onTrackChange={setTrack}
          onClose={stop}
          audioRef={audioRef}
          isPlaying={isPlaying}
          onPlayStateChange={handlePlayStateChange}
          isExpanded={isExpanded}
          onMinimize={minimize}
          onExpand={expand}
          isKeyboardSuspended={isKeyboardSuspended}
        />
      </AudioPlayerActionsContext.Provider>
    </AudioPlayerStateContext.Provider>
  );
}

export { AudioPlayerStateContext, AudioPlayerActionsContext };
