import { useEffect, useEffectEvent, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { STREAM_MODES, TMDB_POSTER_SIZE } from "@/lib/constants";
import {
  directPlayModeLabel,
  getAvailableModes,
  getPrimaryVideoStream,
  playbackDefaultsInput,
  resolveModeForAudioTrack,
  resolvePlaybackSettings,
} from "@/lib/playback";
import {
  buildMovieStreamUrl,
  buildMovieSubtitleTrackInfo,
  clampMoviePlaybackTime,
  hlsPlaybackOffsetSec,
  hlsStartTimeSec,
} from "@/lib/movie-playback";
import {
  authUserQueryOpts,
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
  movieWatchProgressQueryOpts,
  playbackSettingsQueryOpts,
} from "@/lib/query-opts";
import { useDevicePlaybackPreferences } from "@/hooks/useDevicePlaybackPreferences";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { unwrapFloatOrUndefined, unwrapString } from "@/lib/nullable";
import {
  playbackSettingsToPlaySearch,
  subtitleTrackFromPlaySearch,
  type PlaySearchParams,
} from "@/lib/route-search";
import type { PlaybackSettings, StreamModeId } from "@/types";

type UseMoviePlaybackDataArgs = {
  movieId: number;
  search: PlaySearchParams;
  streamReloadKey: number;
  playbackSessionId: string;
  onSyncSearch: (target: PlaybackSettings) => void;
};

export function useMoviePlaybackData({
  movieId,
  search,
  streamReloadKey,
  playbackSessionId,
  onSyncSearch,
}: UseMoviePlaybackDataArgs) {
  const {
    audio_track: audioTrack,
    start,
  } = search;
  const subtitleTrack = subtitleTrackFromPlaySearch(search.subtitle_track);
  const mode: StreamModeId = search.mode ?? "direct";
  const provisionalMode = resolveModeForAudioTrack(mode, audioTrack);

  const {
    data,
    isPending: movieIsPending,
    isError,
  } = useQuery(libraryMovieDetailsQueryOpts(movieId));
  const movie = data && !data.error ? data.data?.movie : null;
  const title = movie?.title ?? "Movie";
  const posterUrl = movie
    ? buildTmdbImageUrl(unwrapString(movie.poster_path), TMDB_POSTER_SIZE)
    : null;
  const movieNotFound = Boolean(isError || (data && data.error));

  const { data: techData, isPending: techPending } = useQuery(
    movieTechnicalDetailsQueryOpts(movieId),
  );
  const { data: watchProgressData, isPending: watchProgressPending } = useQuery(
    movieWatchProgressQueryOpts(movieId),
  );
  const { data: userData, isPending: authUserPending } = useQuery(
    authUserQueryOpts(),
  );
  const user = userData?.error === false ? (userData.data?.user ?? null) : null;
  const {
    data: playbackSettingsData,
    isPending: playbackSettingsPending,
  } = useQuery(playbackSettingsQueryOpts());
  const serverPlaybackSettings =
    playbackSettingsData?.error === false && playbackSettingsData.data?.settings
      ? playbackSettingsData.data.settings
      : null;
  const devicePrefs = useDevicePlaybackPreferences(user?.id ?? 0);
  const userPlaybackPrefs = playbackDefaultsInput(
    devicePrefs,
    serverPlaybackSettings,
  );
  const techLoaded = !techPending && techData?.data != null;
  const videoStreams = techData?.data?.video_streams ?? [];
  const audioStreams = techData?.data?.audio_streams ?? [];
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const chapters = techData?.data?.chapters ?? [];
  const primaryVideo = techLoaded
    ? getPrimaryVideoStream(videoStreams)
    : undefined;
  const availableModes = techLoaded
    ? getAvailableModes({
        video: primaryVideo,
        videoStreamsLoaded: true,
        audioStreams,
        mimeType: techData.data.movie?.mime_type,
      })
    : null;
  // Device preferences are synchronous, so the only thing still worth waiting
  // for is the server catalog -- and getDefaultPlaybackSettings consults it on
  // exactly one path: the mode is not settled by a stored profile, but there is
  // a download speed to size one against. A stored profile this file cannot
  // serve falls through to that same path, so it leaves the mode unsettled too.
  // Everything else (audio/subtitle language) resolves immediately.
  const storedProfileApplies =
    devicePrefs.preferredProfile !== null &&
    (availableModes?.some((m) => m.id === devicePrefs.preferredProfile) ??
      false);
  const needsServerCatalog =
    !storedProfileApplies && devicePrefs.downloadMbps !== null;
  const playbackPreferencesReady =
    !authUserPending && (!needsServerCatalog || !playbackSettingsPending);
  const resolvedPlaybackSettings =
    availableModes !== null
      ? resolvePlaybackSettings(
          {
            mode,
            audioTrack,
            subtitleTrack,
          },
          availableModes,
          audioStreams,
          subtitleStreams,
          userPlaybackPrefs,
        )
      : {
          mode: provisionalMode,
          audioTrack,
          subtitleTrack: subtitleTrack ?? null,
        };
  const resolvedMode = resolvedPlaybackSettings.mode;
  const resolvedAudioTrack = resolvedPlaybackSettings.audioTrack;
  const resolvedSubtitleTrack = resolvedPlaybackSettings.subtitleTrack;
  const isHlsPlayback = resolvedMode !== "direct";
  const movieDurationSec =
    unwrapFloatOrUndefined(techData?.data?.movie?.duration) ??
    unwrapFloatOrUndefined(movie?.duration);
  const playbackStartSec = clampMoviePlaybackTime(
    start,
    0,
    movieDurationSec ?? 0,
  );
  const streamAudioTrack =
    techLoaded && audioStreams.length === 0 ? null : resolvedAudioTrack;
  const requestedHlsStartSec = hlsStartTimeSec(
    isHlsPlayback,
    playbackStartSec,
  );
  const sessionWindowKey = `${movieId}:${resolvedMode}:${streamAudioTrack ?? "none"}:${playbackSessionId}:${Math.floor(requestedHlsStartSec)}`;
  const [reportedActualStart, setReportedActualStart] = useState<{
    sessionWindowKey: string;
    startSec: number;
  } | null>(null);
  const actualHlsStartSec =
    reportedActualStart?.sessionWindowKey === sessionWindowKey
      ? reportedActualStart.startSec
      : requestedHlsStartSec;
  const hlsPlaybackOffset = hlsPlaybackOffsetSec(
    isHlsPlayback,
    playbackStartSec,
    actualHlsStartSec,
  );
  const streamUrl = buildMovieStreamUrl(
    movieId,
    resolvedMode,
    streamAudioTrack,
    requestedHlsStartSec,
    streamReloadKey,
    playbackSessionId,
  );
  const modeLabel =
    resolvedMode === "direct"
      ? directPlayModeLabel(techLoaded ? audioStreams : undefined)
      : (STREAM_MODES.find((m) => m.id === resolvedMode)?.label ??
        resolvedMode);
  const modeUnavailable =
    availableModes !== null && availableModes.length === 0;
  const directPlayAvailable =
    availableModes?.some((m) => m.id === "direct") ?? false;
  const playbackTiming = {
    isHlsPlayback,
    actualHlsStartSec,
    movieDurationSec,
  };
  const subtitleInfo = buildMovieSubtitleTrackInfo({
    movieId,
    resolvedSubtitleTrack,
    techLoaded,
    subtitleStreams,
    actualHlsStartSec,
  });

  const handleActualHlsStart = (startSec: number) => {
    const validStart =
      Number.isFinite(startSec) &&
      startSec >= 0 &&
      startSec <= requestedHlsStartSec;
    if (!validStart) return;

    setReportedActualStart((previous) => {
      const unchanged =
        previous?.sessionWindowKey === sessionWindowKey &&
        previous.startSec === startSec;
      if (unchanged) return previous;

      return { sessionWindowKey, startSec };
    });
  };

  const syncSearch = useEffectEvent((target: PlaybackSettings) => {
    onSyncSearch(target);
  });

  useEffect(() => {
    if (!playbackPreferencesReady || !techLoaded) return;

    const resolvedSettings = {
      mode: resolvedMode,
      audioTrack: resolvedAudioTrack,
      subtitleTrack: resolvedSubtitleTrack,
    };
    const resolvedSearch = playbackSettingsToPlaySearch(resolvedSettings);
    if (
      search.mode === resolvedSearch.mode &&
      audioTrack === resolvedSearch.audio_track &&
      search.subtitle_track === resolvedSearch.subtitle_track
    ) {
      return;
    }

    // The 'parent' is the router's URL search params, not a component holding
    // duplicate state. The URL is also the upstream input, so this is a one-way
    // reconciliation guarded above against a navigate loop.
    // react-doctor-disable-next-line react-doctor/no-pass-data-to-parent, react-doctor/no-pass-live-state-to-parent
    syncSearch(resolvedSettings);
  }, [
    audioTrack,
    playbackPreferencesReady,
    resolvedAudioTrack,
    resolvedMode,
    resolvedSubtitleTrack,
    search.mode,
    search.subtitle_track,
    techLoaded,
  ]);

  return {
    movie,
    movieIsPending,
    movieNotFound,
    techPending,
    techLoaded,
    directPlayAvailable,
    playbackPreferencesReady,
    watchProgressData,
    watchProgressPending,
    title,
    posterUrl,
    modeLabel,
    chapters,
    modeUnavailable,
    resolvedMode,
    resolvedAudioTrack,
    resolvedSubtitleTrack,
    isHlsPlayback,
    playbackStartSec,
    requestedHlsStartSec,
    actualHlsStartSec,
    hlsPlaybackOffset,
    streamUrl,
    subtitleInfo,
    playbackTiming,
    movieDurationSec,
    sessionWindowKey,
    handleActualHlsStart,
  };
}
