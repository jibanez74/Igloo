import {
  createContext,
  useState,
  useRef,
  useEffect,
} from "react";
import type {
  AudioPlayerActions,
  AudioPlayerNowPlaying,
  AudioPlayerQueueState,
  PlayableTrackData,
  TrackType,
} from "@/types";
import AudioPlayer from "@/components/playback/AudioPlayer";
import {
  getShuffleTracks,
  getTracksPaginated,
  recordPlayEvent,
} from "@/lib/api";
import {
  convertToAudioTrack,
  dedupeById,
  extractTrackMetadata,
  playMediaElement,
  shuffleArray,
  toggleMediaPlayback,
  trimQueueHistory,
} from "@/lib/audio-utils";
import {
  SHUFFLE_TRACKS_LIMIT,
  TRACKS_INFINITE_PAGE_SIZE,
} from "@/lib/constants";

const MINIMUM_PLAY_SECONDS = 30;
const COMPLETION_THRESHOLD = 0.8;
const PLAY_CHECK_INTERVAL_MS = 5000;
const MAX_SHUFFLE_FETCH_ATTEMPTS = 3;
const ENDLESS_QUEUE_LOAD_AHEAD_TRACKS = 10;
// Endless queues (shuffle, play all) are trimmed when a new batch is appended;
// this many played tracks stay reachable via previous-track navigation.
const MAX_TRACKS_BEHIND = 50;

function createInitialQueueState(): AudioPlayerQueueState {
  return {
    currentTrack: null,
    tracks: [],
    albumCover: null,
    albumTitle: "",
    musicianName: null,
    isShuffleMode: false,
    isPlayAllMode: false,
    trimmedCount: 0,
  };
}

const AudioPlayerActionsContext = createContext<AudioPlayerActions | null>(null);
// The queue itself is passed to AudioPlayer as props rather than published as
// a context, so appends to an endless queue re-render only the player. Every
// other subscriber (track rows, the app shell) reads this primitive-only
// slice instead.
const AudioPlayerNowPlayingContext =
  createContext<AudioPlayerNowPlaying | null>(null);

export function AudioPlayerProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [queueState, setQueueState] = useState<AudioPlayerQueueState>(() =>
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
      const { cover, musician, albumTitle } = extractTrackMetadata(track);
      trackCoversRef.current.set(track.id, cover);
      trackMusiciansRef.current.set(track.id, musician);
      trackAlbumTitlesRef.current.set(track.id, albumTitle);
    }
  };

  // Append a fetched batch to an endless queue, trimming played tracks beyond
  // MAX_TRACKS_BEHIND so multi-hour shuffle or play-all sessions stay bounded.
  // The updater stays pure: their metadata is pruned by the effect below.
  const appendToQueue = (appended: TrackType[]) => {
    setQueueState(prev => {
      const { tracks: kept, dropped } = trimQueueHistory(
        prev.tracks,
        prev.currentTrack?.id ?? null,
        MAX_TRACKS_BEHIND,
      );

      return {
        ...prev,
        tracks: [...kept, ...appended],
        trimmedCount: prev.trimmedCount + dropped.length,
      };
    });
  };

  // Drop metadata for tracks that have left the queue, keyed on the committed
  // queue rather than done inside the updater above: React may run an updater
  // for a render it discards, and these deletions are permanent, which would
  // strand a still-queued track without its cover or artist (see setTrack's
  // has() lookups). Reconciling against the commit is idempotent instead.
  useEffect(() => {
    const liveIds = new Set(queueState.tracks.map(track => track.id));
    if (queueState.currentTrack) {
      liveIds.add(queueState.currentTrack.id);
    }

    const maps = [
      trackCoversRef.current,
      trackMusiciansRef.current,
      trackAlbumTitlesRef.current,
    ];

    for (const map of maps) {
      if (!map) continue;

      for (const id of [...map.keys()]) {
        if (!liveIds.has(id)) {
          map.delete(id);
        }
      }
    }
  }, [queueState.tracks, queueState.currentTrack]);

  const clearMetadataRefs = () => {
    trackCoversRef.current?.clear();
    trackMusiciansRef.current?.clear();
    trackAlbumTitlesRef.current?.clear();
    playAllOffsetRef.current = 0;
    playAllTotalRef.current = 0;
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

        // The React Compiler cannot compile components containing
        // conditional/logical expressions inside try blocks, so the try wraps
        // only the fetch itself.
        let response;
        try {
          response = await getShuffleTracks(SHUFFLE_TRACKS_LIMIT);
        } catch {
          // Silently fail - user can continue with the current queue.
          break;
        }

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
      }

      if (!isCancelled && collected.length > 0) {
        appendToQueue(collected);
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
      queueState.tracks.length - currentTrackIndex <
        ENDLESS_QUEUE_LOAD_AHEAD_TRACKS &&
      !isFetchingMoreRef.current &&
      playAllOffsetRef.current < playAllTotalRef.current;

    if (!shouldFetchMore) {
      return;
    }

    let isCancelled = false;

    const fetchMorePlayAllTracks = async () => {
      isFetchingMoreRef.current = true;

      // Try wraps only the fetch: the React Compiler cannot compile
      // components containing conditional/logical expressions in try blocks.
      let response = null;
      try {
        response = await getTracksPaginated(
          TRACKS_INFINITE_PAGE_SIZE,
          playAllOffsetRef.current,
        );
      } catch {
        // Silently fail - user can continue with the current queue.
      }

      if (
        response !== null &&
        !isCancelled &&
        !response.error &&
        response.data.tracks.length > 0
      ) {
        const rawTracks = response.data.tracks;
        const newTracks = rawTracks.map(convertToAudioTrack);

        populateTrackMetadata(rawTracks);
        playAllOffsetRef.current += rawTracks.length;

        if (newTracks.length > 0) {
          appendToQueue(newTracks);
        }
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

  // Every playback entry point resets the metadata maps, seeds the queue, and
  // expands the player. Mixed-list flows also pass rawTracks so setTrack can
  // resolve per-track metadata on navigation; album flows leave the maps empty
  // so the queue-wide albumInfo carries over.
  const startQueue = ({
    currentTrack,
    tracks,
    albumInfo,
    rawTracks,
    isShuffleMode = false,
    isPlayAllMode = false,
  }: {
    currentTrack: TrackType;
    tracks: TrackType[];
    albumInfo: { cover: string | null; title: string; musician: string | null };
    rawTracks?: PlayableTrackData[];
    isShuffleMode?: boolean;
    isPlayAllMode?: boolean;
  }) => {
    clearMetadataRefs();
    if (rawTracks) {
      populateTrackMetadata(rawTracks);
    }
    // Restarting a queue whose first track is already the current one leaves
    // the stream URL unchanged, so AudioPlayer's load effect does not re-fire
    // — rewind here or "Play all" would silently resume mid-track.
    const isSameTrack = currentTrack.id === queueState.currentTrack?.id;

    setQueueState({
      currentTrack,
      tracks,
      albumCover: albumInfo.cover,
      albumTitle: albumInfo.title,
      musicianName: albumInfo.musician,
      isShuffleMode,
      isPlayAllMode,
      trimmedCount: 0,
    });
    setIsExpanded(true);

    if (isSameTrack && audioRef.current) {
      audioRef.current.currentTime = 0;
      void playMediaElement(audioRef.current);
    }
  };

  const playTrack: AudioPlayerActions["playTrack"] = (track, playlist, albumInfo) => {
    // Track rows are labeled "Pause X"/"Play X" for the current track, so a
    // repeat click toggles playback instead of rebuilding the queue and
    // re-opening the fullscreen player — even when clicked from a different
    // list than the one the queue came from.
    if (track.id === queueState.currentTrack?.id) {
      togglePlay();
      return;
    }

    startQueue({ currentTrack: track, tracks: playlist, albumInfo });
  };

  const playTrackFromList: AudioPlayerActions["playTrackFromList"] = (
    rawTracks,
    startTrackId,
  ) => {
    // Same toggle-instead-of-rebuild contract as playTrack above.
    if (startTrackId === queueState.currentTrack?.id) {
      togglePlay();
      return;
    }

    const uniqueRawTracks = dedupeById(rawTracks);

    const startRawTrack = uniqueRawTracks.find(
      track => track.id === startTrackId,
    );
    if (!startRawTrack) return;

    const tracks = uniqueRawTracks.map(convertToAudioTrack);
    const { cover, musician, albumTitle } = extractTrackMetadata(startRawTrack);

    startQueue({
      currentTrack: tracks[uniqueRawTracks.indexOf(startRawTrack)],
      tracks,
      albumInfo: { cover, title: albumTitle, musician },
      rawTracks: uniqueRawTracks,
    });
  };

  const playQueue: AudioPlayerActions["playQueue"] = (tracks, albumInfo) => {
    if (tracks.length === 0) return;

    startQueue({ currentTrack: tracks[0], tracks, albumInfo });
  };

  const shuffleQueue: AudioPlayerActions["shuffleQueue"] = (tracks, albumInfo) => {
    if (tracks.length === 0) return;

    const shuffled = shuffleArray(tracks);

    startQueue({ currentTrack: shuffled[0], tracks: shuffled, albumInfo });
  };

  const startShufflePlayback: AudioPlayerActions["startShufflePlayback"] =
    async () => {
      const response = await getShuffleTracks(SHUFFLE_TRACKS_LIMIT);
      if (response.error || response.data.tracks.length === 0) {
        return;
      }

      // The append effect above already dedupes subsequent batches; this one
      // covers repeats within the first batch.
      const rawTracks = dedupeById(response.data.tracks);
      const tracks = rawTracks.map(convertToAudioTrack);
      const { cover, musician } = extractTrackMetadata(rawTracks[0]);

      startQueue({
        currentTrack: tracks[0],
        tracks,
        albumInfo: { cover, title: "Shuffle All", musician },
        rawTracks,
        isShuffleMode: true,
      });
    };

  const startPlayAllPlayback: AudioPlayerActions["startPlayAllPlayback"] =
    async () => {
      const response = await getTracksPaginated(TRACKS_INFINITE_PAGE_SIZE, 0);
      if (response.error || response.data.tracks.length === 0) {
        return;
      }

      const rawTracks = response.data.tracks;
      const tracks = rawTracks.map(convertToAudioTrack);
      const { cover, musician } = extractTrackMetadata(rawTracks[0]);

      startQueue({
        currentTrack: tracks[0],
        tracks,
        albumInfo: { cover, title: "All Tracks", musician },
        rawTracks,
        isPlayAllMode: true,
      });

      // After startQueue: clearMetadataRefs inside it zeroes these counters.
      playAllOffsetRef.current = rawTracks.length;
      playAllTotalRef.current = response.data.total;
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
        albumTitle: trackAlbumTitlesRef.current?.has(track.id)
          ? (trackAlbumTitlesRef.current.get(track.id) ?? "")
          : prev.albumTitle,
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
    toggleMediaPlayback(audioRef.current);
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

  const nowPlayingValue = {
    currentTrackId: queueState.currentTrack?.id ?? null,
    isPlaying,
    isExpanded,
  };

  const actionsValue = {
    playTrack,
    playTrackFromList,
    playQueue,
    shuffleQueue,
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
    <AudioPlayerActionsContext.Provider value={actionsValue}>
      <AudioPlayerNowPlayingContext.Provider value={nowPlayingValue}>
        {children}
        <AudioPlayer
          track={queueState.currentTrack}
          tracks={queueState.tracks}
          trimmedCount={queueState.trimmedCount}
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
      </AudioPlayerNowPlayingContext.Provider>
    </AudioPlayerActionsContext.Provider>
  );
}

export { AudioPlayerActionsContext, AudioPlayerNowPlayingContext };
