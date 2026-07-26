import { useEffect, useEffectEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { STREAM_MODES, TMDB_POSTER_SIZE } from "@/lib/constants";
import {
  getAvailableModes,
  getPrimaryVideoStream,
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
import type { PlaySearchParams } from "@/lib/route-search";
import type { StreamModeId } from "@/types";

type MoviePlaybackSyncTarget = {
  mode: StreamModeId;
  audioTrack: number;
  subtitleTrack: number | null;
};

type UseMoviePlaybackDataArgs = {
  movieId: number;
  search: PlaySearchParams;
  streamReloadKey: number;
  playbackSessionId: string;
  onSyncSearch: (target: MoviePlaybackSyncTarget) => void;
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
    subtitle_track: subtitleTrack,
    start,
  } = search;
  const mode: StreamModeId = search.mode ?? "direct";

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
            subtitleTrack: subtitleTrack ?? null,
          },
          availableModes,
          audioStreams,
          subtitleStreams,
          userPlaybackPrefs,
        )
      : {
          mode,
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
  const streamAudioTrack = audioStreams.length > 0 ? resolvedAudioTrack : null;
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

  const syncSearch = useEffectEvent((target: MoviePlaybackSyncTarget) => {
    onSyncSearch(target);
  });

  useEffect(() => {
    if (!playbackPreferencesReady || availableModes === null) return;

    const resolvedSubtitleSearch = resolvedSubtitleTrack ?? undefined;
    if (
      mode === resolvedMode &&
      audioTrack === resolvedAudioTrack &&
      subtitleTrack === resolvedSubtitleSearch
    ) {
      return;
    }

    syncSearch({
      mode: resolvedMode,
      audioTrack: resolvedAudioTrack,
      subtitleTrack: resolvedSubtitleTrack,
    });
  }, [
    audioTrack,
    availableModes,
    mode,
    playbackPreferencesReady,
    resolvedAudioTrack,
    resolvedMode,
    resolvedSubtitleTrack,
    subtitleTrack,
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
