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
import { useEndlessQueueRefill } from "@/hooks/useEndlessQueueRefill";
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
  SHUFFLE_EXCLUDE_LIMIT,
  SHUFFLE_TRACKS_LIMIT,
  TRACKS_INFINITE_PAGE_SIZE,
} from "@/lib/constants";
import { showInfo } from "@/lib/toast-helpers";

const MINIMUM_PLAY_SECONDS = 30;
const COMPLETION_THRESHOLD = 0.8;
const PLAY_CHECK_INTERVAL_MS = 5000;
// Both endless modes (shuffle, play all) top up at the same runway: start
// fetching once this few tracks remain ahead of the current one.
const ENDLESS_QUEUE_LOAD_AHEAD_TRACKS = 10;
// Endless queues (shuffle, play all) are trimmed when a new batch is appended;
// this many played tracks stay reachable via previous-track navigation.
const MAX_TRACKS_BEHIND = 50;

function createInitialQueueState(): AudioPlayerQueueState {
  return {
    queueId: 0,
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
  //
  // queueId is the queue the batch was fetched for. A batch that lands after
  // the user started something else is dropped here rather than in an effect
  // cleanup: this is the only place that can see which queue is actually live,
  // so a batch cancelled merely by a track advance still gets kept.
  const appendToQueue = (appended: TrackType[], queueId: number) => {
    setQueueState(prev => {
      if (prev.queueId !== queueId) {
        return prev;
      }

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

  // Resolve one track's display metadata: the per-track values when the queue
  // carries them (mixed lists - playlists, shuffle, play all, search results),
  // otherwise the queue-wide fallback, which is what album and musician queues
  // rely on. A track can legitimately map to null (no cover, no musician), so
  // "unmapped" and "mapped to null" are distinguished via has().
  const resolveTrackDisplay = (
    trackId: number,
    fallback: { cover: string | null; title: string; musician: string | null },
  ) => {
    const covers = trackCoversRef.current;
    const musicians = trackMusiciansRef.current;
    const albumTitles = trackAlbumTitlesRef.current;

    return {
      albumCover: covers?.has(trackId)
        ? (covers.get(trackId) ?? null)
        : fallback.cover,
      musicianName: musicians?.has(trackId)
        ? (musicians.get(trackId) ?? null)
        : fallback.musician,
      albumTitle: albumTitles?.has(trackId)
        ? (albumTitles.get(trackId) ?? "")
        : fallback.title,
    };
  };

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

  // Runway left ahead of the current track; negative when the current track is
  // not in the queue (which the refill hook treats as "do nothing").
  const tracksAhead = currentTrackIndex >= 0
    ? queueState.tracks.length - currentTrackIndex
    : -1;

  // Tell the server which tracks the queue already holds so it samples around
  // them. It used to re-sample the whole library blind, which on a library
  // smaller than the queue returned nothing but duplicates and burned a full
  // table scan per retry.
  const fetchShuffleBatch = async () => {
    const knownIds = [...new Set(queueState.tracks.map(track => track.id))];

    const response = await getShuffleTracks(
      SHUFFLE_TRACKS_LIMIT,
      // The most recent ids matter most if the queue ever outgrows the cap.
      knownIds.slice(-SHUFFLE_EXCLUDE_LIMIT),
    );
    if (response.error) {
      return [];
    }

    // An empty batch now means the exclusions covered everything left, so say
    // so instead of letting the queue quietly run dry.
    if (response.data.tracks.length === 0) {
      showInfo(
        "That's the whole library",
        "Shuffle has played everything it hasn't already queued.",
      );
      return [];
    }

    // Defensive: the server honours only the first SHUFFLE_EXCLUDE_LIMIT ids,
    // and a repeated id would break the player's findIndex navigation.
    const excluded = new Set(knownIds);
    const rawTracks = response.data.tracks.filter(
      track => !excluded.has(track.id),
    );

    populateTrackMetadata(rawTracks);

    return rawTracks.map(convertToAudioTrack);
  };

  const fetchPlayAllBatch = async () => {
    // Play-all walks the library in order, so it stops once the last page has
    // been fetched. Checked here rather than in the `enabled` flag so the refs
    // are not read during render.
    if (playAllOffsetRef.current >= playAllTotalRef.current) {
      return [];
    }

    const response = await getTracksPaginated(
      TRACKS_INFINITE_PAGE_SIZE,
      playAllOffsetRef.current,
    );
    if (response.error || response.data.tracks.length === 0) {
      return [];
    }

    const rawTracks = response.data.tracks;
    populateTrackMetadata(rawTracks);
    playAllOffsetRef.current += rawTracks.length;

    return rawTracks.map(convertToAudioTrack);
  };

  useEndlessQueueRefill({
    enabled: queueState.isShuffleMode,
    queueId: queueState.queueId,
    tracksAhead,
    lookahead: ENDLESS_QUEUE_LOAD_AHEAD_TRACKS,
    fetchBatch: fetchShuffleBatch,
    onAppend: appendToQueue,
  });

  useEndlessQueueRefill({
    enabled: queueState.isPlayAllMode,
    queueId: queueState.queueId,
    tracksAhead,
    lookahead: ENDLESS_QUEUE_LOAD_AHEAD_TRACKS,
    fetchBatch: fetchPlayAllBatch,
    onAppend: appendToQueue,
  });

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

    // Resolve the opening track the same way navigation does, or a mixed queue
    // would show the queue-wide albumInfo for track 1 and each track's own
    // details only from track 2 onward.
    const display = resolveTrackDisplay(currentTrack.id, albumInfo);

    setQueueState(prev => ({
      // A fresh identity so any endless-queue batch still in flight for the
      // previous queue is dropped by appendToQueue instead of landing here.
      queueId: prev.queueId + 1,
      currentTrack,
      tracks,
      albumCover: display.albumCover,
      albumTitle: display.albumTitle,
      musicianName: display.musicianName,
      isShuffleMode,
      isPlayAllMode,
      trimmedCount: 0,
    }));
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

  const playQueue: AudioPlayerActions["playQueue"] = (
    tracks,
    albumInfo,
    rawTracks,
  ) => {
    if (tracks.length === 0) return;

    startQueue({ currentTrack: tracks[0], tracks, albumInfo, rawTracks });
  };

  const shuffleQueue: AudioPlayerActions["shuffleQueue"] = (
    tracks,
    albumInfo,
    rawTracks,
  ) => {
    if (tracks.length === 0) return;

    // rawTracks only seeds the id-keyed metadata maps, so it stays in its
    // original order while the queue itself is shuffled.
    const shuffled = shuffleArray(tracks);

    startQueue({
      currentTrack: shuffled[0],
      tracks: shuffled,
      albumInfo,
      rawTracks,
    });
  };

  const startShufflePlayback: AudioPlayerActions["startShufflePlayback"] =
    async () => {
      const response = await getShuffleTracks(SHUFFLE_TRACKS_LIMIT);
      // apiRequest resolves an error envelope rather than rejecting, so throw
      // here or the caller's catch (and its toast) never runs.
      if (response.error) {
        throw new Error(response.message || "Failed to fetch shuffle tracks");
      }
      if (response.data.tracks.length === 0) {
        return;
      }

      // The append effect above already dedupes subsequent batches; this one
      // covers repeats within the first batch.
      const rawTracks = dedupeById(response.data.tracks);
      const tracks = rawTracks.map(convertToAudioTrack);
      // The first track's own album, not a "Shuffle All" placeholder: setTrack
      // resolves the real title from track 2 onward, so a literal here would
      // show a bogus album for exactly one track.
      const { cover, musician, albumTitle } = extractTrackMetadata(rawTracks[0]);

      startQueue({
        currentTrack: tracks[0],
        tracks,
        albumInfo: { cover, title: albumTitle, musician },
        rawTracks,
        isShuffleMode: true,
      });
    };

  const startPlayAllPlayback: AudioPlayerActions["startPlayAllPlayback"] =
    async () => {
      const response = await getTracksPaginated(TRACKS_INFINITE_PAGE_SIZE, 0);
      // Same error-envelope caveat as startShufflePlayback above.
      if (response.error) {
        throw new Error(response.message || "Failed to fetch tracks");
      }
      if (response.data.tracks.length === 0) {
        return;
      }

      const rawTracks = response.data.tracks;
      const tracks = rawTracks.map(convertToAudioTrack);
      const { cover, musician, albumTitle } = extractTrackMetadata(rawTracks[0]);

      startQueue({
        currentTrack: tracks[0],
        tracks,
        albumInfo: { cover, title: albumTitle, musician },
        rawTracks,
        isPlayAllMode: true,
      });

      // After startQueue: clearMetadataRefs inside it zeroes these counters.
      playAllOffsetRef.current = rawTracks.length;
      playAllTotalRef.current = response.data.total;
    };

  const setTrack: AudioPlayerActions["setTrack"] = track => {
    setQueueState(prev => ({
      ...prev,
      currentTrack: track,
      // Album and musician queues leave the maps empty, so the values already
      // on the queue are the fallback.
      ...resolveTrackDisplay(track.id, {
        cover: prev.albumCover,
        title: prev.albumTitle,
        musician: prev.musicianName,
      }),
    }));
  };

  const stop: AudioPlayerActions["stop"] = () => {
    clearMetadataRefs();
    // Keep advancing the identity so a batch still in flight is not appended
    // into the cleared queue.
    setQueueState(prev => ({
      ...createInitialQueueState(),
      queueId: prev.queueId + 1,
    }));
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
    // The React Compiler memoizes both provider values - this component is kept
    // compilable on purpose (see the try-block comments above). Splitting actions
    // from now-playing is already the structural fix for consumer re-renders.
    // react-doctor-disable-next-line react-doctor/context-provider-value-from-unmemoized-local-literal
    <AudioPlayerActionsContext.Provider value={actionsValue}>
      {/* Compiler-memoized, as above. */}
      {/* react-doctor-disable-next-line react-doctor/context-provider-value-from-unmemoized-local-literal */}
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
