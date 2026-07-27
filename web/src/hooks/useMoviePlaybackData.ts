import { useEffect, useEffectEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { STREAM_MODES, TMDB_POSTER_SIZE } from "@/lib/constants";
import {
  getAvailableModes,
  getPrimaryVideoStream,
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
  } = useQuery({
    ...playbackSettingsQueryOpts(user?.id ?? 0),
    enabled: user !== null,
  });
  const userPlaybackPrefs =
    playbackSettingsData?.error === false && playbackSettingsData.data?.settings
      ? playbackSettingsData.data.settings
      : null;
  const playbackPreferencesReady =
    !authUserPending && (user === null || !playbackSettingsPending);

  const techLoaded = !techPending && techData?.data != null;
  const videoStreams = techData?.data?.video_streams ?? [];
  const audioStreams = techData?.data?.audio_streams ?? [];
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const chapters = techData?.data?.chapters ?? [];
  const primaryVideo = techLoaded
    ? getPrimaryVideoStream(videoStreams)
    : undefined;
  const availableModes = techLoaded
    ? getAvailableModes(
        primaryVideo?.height ?? 0,
        primaryVideo?.codec,
        audioStreams[0]?.codec,
        techData.data.movie?.mime_type,
      )
    : null;
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
  const hlsStartSec = hlsStartTimeSec(isHlsPlayback, playbackStartSec);
  const hlsPlaybackOffset = hlsPlaybackOffsetSec(
    isHlsPlayback,
    playbackStartSec,
    hlsStartSec,
  );
  const streamAudioTrack =
    techLoaded && audioStreams.length === 0 ? null : resolvedAudioTrack;
  const streamUrl = buildMovieStreamUrl(
    movieId,
    resolvedMode,
    streamAudioTrack,
    hlsStartSec,
    streamReloadKey,
    playbackSessionId,
  );
  const modeLabel =
    STREAM_MODES.find((m) => m.id === resolvedMode)?.label ?? resolvedMode;
  const modeUnavailable =
    availableModes !== null && availableModes.length === 0;
  const playbackTiming = { isHlsPlayback, hlsStartSec, movieDurationSec };
  const subtitleInfo = buildMovieSubtitleTrackInfo({
    movieId,
    resolvedSubtitleTrack,
    techLoaded,
    subtitleStreams,
  });
  const sessionWindowKey = `${movieId}:${resolvedMode}:${streamAudioTrack ?? "none"}:${playbackSessionId}:${Math.floor(hlsStartSec)}`;

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
    hlsStartSec,
    hlsPlaybackOffset,
    streamUrl,
    subtitleInfo,
    playbackTiming,
    movieDurationSec,
    sessionWindowKey,
  };
}
