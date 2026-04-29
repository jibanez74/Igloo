import { useEffect, useEffectEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import {
  STREAM_MODES,
  getAvailableModes,
  getPrimaryVideoStream,
  resolvePlaybackSettings,
  type StreamModeId,
} from "@/lib/playback";
import {
  buildMovieStreamUrl,
  buildMovieSubtitleTrackInfo,
  hlsPlaybackOffsetSec,
  hlsStartTimeSec,
} from "@/lib/movie-playback";
import {
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
  movieWatchProgressQueryOpts,
} from "@/lib/query-opts";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { unwrapFloatOrUndefined, unwrapString } from "@/lib/nullable";
import type { PlaySearchParams } from "@/types";

type SyncTarget = {
  mode: StreamModeId;
  audioTrack: number;
  subtitleTrack: number | null;
};

type UseMoviePlaybackDataArgs = {
  movieId: number;
  search: PlaySearchParams;
  streamReloadKey: number;
  onSyncSearch: (target: SyncTarget) => void;
};

export function useMoviePlaybackData({
  movieId,
  search,
  streamReloadKey,
  onSyncSearch,
}: UseMoviePlaybackDataArgs) {
  const { mode, audio_track: audioTrack, subtitle_track: subtitleTrack, start } = search;

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
        techData.data!.movie?.mime_type,
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
  const hlsStartSec = hlsStartTimeSec(isHlsPlayback, start);
  const hlsPlaybackOffset = hlsPlaybackOffsetSec(
    isHlsPlayback,
    start,
    hlsStartSec,
  );
  const streamUrl = buildMovieStreamUrl(
    movieId,
    resolvedMode,
    resolvedAudioTrack,
    hlsStartSec,
    streamReloadKey,
  );
  const qualityLabel =
    STREAM_MODES.find((m) => m.id === resolvedMode)?.label ?? resolvedMode;
  const modeUnavailable =
    availableModes !== null && availableModes.length === 0;
  const movieDurationSec =
    unwrapFloatOrUndefined(techData?.data?.movie?.duration) ??
    unwrapFloatOrUndefined(movie?.duration);
  const playbackTiming = { isHlsPlayback, hlsStartSec, movieDurationSec };
  const subtitleInfo = buildMovieSubtitleTrackInfo({
    movieId,
    resolvedSubtitleTrack,
    techLoaded,
    subtitleStreams,
  });
  const sessionWindowKey = `${movieId}:${resolvedMode}:${resolvedAudioTrack}:${Math.floor(hlsStartSec)}`;

  const syncSearch = useEffectEvent((target: SyncTarget) => {
    onSyncSearch(target);
  });

  useEffect(() => {
    if (availableModes === null) return;

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
    resolvedAudioTrack,
    resolvedMode,
    resolvedSubtitleTrack,
    subtitleTrack,
  ]);

  return {
    movie,
    movieIsPending,
    movieNotFound,
    techData,
    techPending,
    watchProgressData,
    watchProgressPending,
    title,
    posterUrl,
    qualityLabel,
    chapters,
    modeUnavailable,
    resolvedMode,
    resolvedAudioTrack,
    resolvedSubtitleTrack,
    isHlsPlayback,
    hlsStartSec,
    hlsPlaybackOffset,
    streamUrl,
    subtitleInfo,
    playbackTiming,
    movieDurationSec,
    sessionWindowKey,
  };
}
