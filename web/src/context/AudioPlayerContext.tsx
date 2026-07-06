import {
  createContext,
  useState,
  useRef,
  useEffect,
} from "react";
import type {
  AudioPlayerActions,
  AudioPlayerState,
  PlayableTrackData,
  TrackType,
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
} from "@/lib/audio-utils";

const MINIMUM_PLAY_SECONDS = 30;
const COMPLETION_THRESHOLD = 0.8;
const PLAY_CHECK_INTERVAL_MS = 5000;
const MAX_SHUFFLE_FETCH_ATTEMPTS = 3;

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

  const trackCoversRef = useRef<Map<number, string | null> | null>(null);
  const trackMusiciansRef = useRef<Map<number, string | null> | null>(null);
  const trackAlbumTitlesRef = useRef<Map<number, string> | null>(null);

  const playAllOffsetRef = useRef(0);
  const playAllTotalRef = useRef(0);

  const populateTrackMetadata = (tracks: PlayableTrackData[]) => {
    if (trackCoversRef.current === null) {
      trackCoversRef.current = new Map();
    }
    if (trackMusiciansRef.current === null) {
      trackMusiciansRef.current = new Map();
    }
    if (trackAlbumTitlesRef.current === null) {
      trackAlbumTitlesRef.current = new Map();
    }

    for (const track of tracks) {
      const { cover, musician } = extractTrackMetadata(track);
      trackCoversRef.current.set(track.id, cover);
      trackMusiciansRef.current.set(track.id, musician);
      if (track.album_title?.Valid) {
        trackAlbumTitlesRef.current.set(track.id, track.album_title.String);
      }
    }
  };

  const clearMetadataRefs = () => {
    trackCoversRef.current?.clear();
    trackMusiciansRef.current?.clear();
    trackAlbumTitlesRef.current?.clear();
    playAllOffsetRef.current = 0;
    playAllTotalRef.current = 0;
  };

  const playAudio = async () => {
    const audio = audioRef.current;
    if (!audio) return;

    try {
      await audio.play();
    } catch {
      // Playback can still be blocked by the browser in some cases.
    }
  };

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

      // The shuffle endpoint returns purely random tracks with no awareness of
      // what we've already queued, so dedupe against the existing queue. Retry a
      // few times when a batch is all duplicates so playback doesn't stall.
      const knownIds = new Set(queueState.tracks.map(track => track.id));
      const collected: TrackType[] = [];
      let attempts = 0;

      while (
        attempts < MAX_SHUFFLE_FETCH_ATTEMPTS &&
        collected.length === 0 &&
        !isCancelled
      ) {
        attempts++;

        try {
          const response = await getShuffleTracks(50);

          if (response.error || response.data.tracks.length === 0) {
            break;
          }

          const rawTracks = response.data.tracks;
          populateTrackMetadata(rawTracks);

          for (const track of rawTracks) {
            if (!knownIds.has(track.id)) {
              knownIds.add(track.id);
              collected.push(convertToAudioTrack(track));
            }
          }
        } catch {
          // Silently fail - user can continue with the current queue.
          break;
        }
      }

      if (!isCancelled && collected.length > 0) {
        setQueueState(prev => ({
          ...prev,
          tracks: [...prev.tracks, ...collected],
        }));
      }

      isFetchingMoreRef.current = false;
    };

    void fetchMoreShuffleTracks();

    return () => {
      isCancelled = true;
    };
  }, [
    queueState.isShuffleMode,
    currentTrackIndex,
    queueState.tracks,
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

        if (!isCancelled && !response.error && response.data.tracks.length > 0) {
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
        }
      } catch {
        // Silently fail - user can continue with the current queue.
      }

      isFetchingMoreRef.current = false;
    };

    void fetchMorePlayAllTracks();

    return () => {
      isCancelled = true;
    };
  }, [
    queueState.isPlayAllMode,
    currentTrackIndex,
    queueState.tracks.length,
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

  const playTrack: AudioPlayerActions["playTrack"] = (track, playlist, albumInfo) => {
    clearMetadataRefs();
    setQueueState({
      currentTrack: track,
      tracks: playlist,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
      isShuffleMode: false,
      isPlayAllMode: false,
    });
    setIsExpanded(true);
  };

  const playTrackFromList: AudioPlayerActions["playTrackFromList"] = (
    rawTracks,
    startTrackId,
  ) => {
    // Mixed lists (search results, library tracks tab) can repeat an id;
    // dedupe so findIndex-based prev/next navigation stays coherent.
    const seenIds = new Set<number>();
    const uniqueRawTracks = rawTracks.filter(track => {
      if (seenIds.has(track.id)) {
        return false;
      }
      seenIds.add(track.id);
      return true;
    });

    const startRawTrack = uniqueRawTracks.find(
      track => track.id === startTrackId,
    );
    if (!startRawTrack) return;

    clearMetadataRefs();
    populateTrackMetadata(uniqueRawTracks);

    const tracks = uniqueRawTracks.map(convertToAudioTrack);
    const { cover, musician } = extractTrackMetadata(startRawTrack);

    setQueueState({
      currentTrack: tracks[uniqueRawTracks.indexOf(startRawTrack)],
      tracks,
      albumCover: cover,
      albumTitle: startRawTrack.album_title?.Valid
        ? startRawTrack.album_title.String
        : "",
      musicianName: musician,
      isShuffleMode: false,
      isPlayAllMode: false,
    });
    setIsExpanded(true);
  };

  const playAlbum: AudioPlayerActions["playAlbum"] = (tracks, albumInfo) => {
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
    });
    setIsExpanded(true);
  };

  const shuffleAlbum: AudioPlayerActions["shuffleAlbum"] = (tracks, albumInfo) => {
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
    });
    setIsExpanded(true);
  };

  const startShufflePlayback: AudioPlayerActions["startShufflePlayback"] =
    async () => {
      const response = await getShuffleTracks(50);
      if (response.error || response.data.tracks.length === 0) {
        return;
      }

      // The shuffle endpoint can repeat an id within one batch; dedupe so
      // findIndex-based prev/next navigation stays coherent (the append
      // effect above already dedupes subsequent batches).
      const seenIds = new Set<number>();
      const rawTracks = response.data.tracks.filter(track => {
        if (seenIds.has(track.id)) {
          return false;
        }
        seenIds.add(track.id);
        return true;
      });
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
      });
      setIsExpanded(true);
    };

  const startPlayAllPlayback: AudioPlayerActions["startPlayAllPlayback"] =
    async () => {
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
      });
      setIsExpanded(true);
    };

  const setTrack: AudioPlayerActions["setTrack"] = track => {
    setQueueState(prev => {
      // Album/playlist flows clear the metadata maps, so their lookups miss
      // and the queue-wide values carry over; mixed queues (shuffle, play
      // all, search/library lists) resolve per-track metadata here. A track
      // can legitimately map to null (no cover/musician), so distinguish
      // "unmapped" from "mapped to null" via has().
      const covers = trackCoversRef.current;
      const musicians = trackMusiciansRef.current;

      return {
        ...prev,
        currentTrack: track,
        albumCover: covers?.has(track.id)
          ? (covers.get(track.id) ?? null)
          : prev.albumCover,
        musicianName: musicians?.has(track.id)
          ? (musicians.get(track.id) ?? null)
          : prev.musicianName,
        albumTitle:
          trackAlbumTitlesRef.current?.get(track.id) ?? prev.albumTitle,
      };
    });
  };

  const stop: AudioPlayerActions["stop"] = () => {
    clearMetadataRefs();
    setQueueState(createInitialQueueState());
    setIsPlaying(false);
    setIsExpanded(false);
  };

  const pause = () => {
    audioRef.current?.pause();
  };

  const togglePlay = () => {
    const audio = audioRef.current;
    if (!audio) return;

    if (audio.paused) {
      void playAudio();
      return;
    }

    audio.pause();
  };

  const suspendKeyboard = () => {
    setKeyboardSuspendCount(count => count + 1);
  };

  const resumeKeyboard = () => {
    setKeyboardSuspendCount(count => Math.max(0, count - 1));
  };

  const expand = () => {
    setIsExpanded(true);
  };

  const minimize = () => {
    setIsExpanded(false);
  };

  const handlePlayStateChange = (playing: boolean) => {
    setIsPlaying(playing);
  };

  const isKeyboardSuspended = keyboardSuspendCount > 0;

  const stateValue = {
    ...queueState,
    isPlaying,
    isExpanded,
    isKeyboardSuspended,
  };

  const actionsValue = {
    playTrack,
    playTrackFromList,
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
  };

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
