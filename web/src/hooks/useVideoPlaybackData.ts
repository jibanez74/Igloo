import { useEffect, useEffectEvent, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiErrorMessage } from "@/lib/is-api-failure";
import { mediaKey } from "@/lib/media-ref";
import {
  getAvailableModes,
  getDefaultPlaybackSettings,
  getPrimaryVideoStream,
  hlsVideoCodecsFor,
  playbackDefaultsInput,
  playbackModeLabel,
  resolveModeForAudioTrack,
  resolvePlaybackSettings,
} from "@/lib/playback";
import {
  buildStreamUrl,
  buildSubtitleTrackInfo,
  clampPlaybackTime,
  hlsPlaybackOffsetSec,
  hlsStartTimeSec,
} from "@/lib/video-playback";
import {
  authUserFrom,
  authUserQueryOpts,
  mediaTechnicalDetailsQueryOpts,
  mediaWatchProgressQueryOpts,
  playbackSettingsFromResponse,
  playbackSettingsQueryOpts,
  playbackTechnicalFile,
} from "@/lib/query-opts";
import { useDevicePlaybackPreferences } from "@/hooks/useDevicePlaybackPreferences";
import { useLastSuccess } from "@/hooks/useLastSuccess";
import { unwrapFloatOrUndefined } from "@/lib/nullable";
import {
  playbackSettingsToPlaySearch,
  subtitleTrackFromPlaySearch,
  type PlaySearchParams,
} from "@/lib/route-search";
import type { PlaybackMediaRef, PlaybackSettings, StreamModeId } from "@/types";

type UseVideoPlaybackDataArgs = {
  media: PlaybackMediaRef;
  search: PlaySearchParams;
  streamReloadKey: number;
  playbackSessionId: string;
  /**
   * Duration known before the technical details resolve (a movie details
   * row carries one); the file's own duration wins once it is loaded.
   */
  fallbackDurationSec?: number;
  onSyncSearch: (target: PlaybackSettings) => void;
};

/**
 * Everything the player derives from the URL and the media's technical
 * details: the resolved mode and tracks, the stream and subtitle URLs, and
 * the absolute-time bookkeeping a rebased HLS session needs. The caller owns
 * the media's own details query (title, artwork), which differs per kind.
 */
export function useVideoPlaybackData({
  media,
  search,
  streamReloadKey,
  playbackSessionId,
  fallbackDurationSec,
  onSyncSearch,
}: UseVideoPlaybackDataArgs) {
  const {
    audio_track: audioTrack,
    start,
  } = search;
  const subtitleTrack = subtitleTrackFromPlaySearch(search.subtitle_track);
  const mode: StreamModeId = search.mode ?? "direct";
  const provisionalMode = resolveModeForAudioTrack(mode, audioTrack);

  // Each input to the stream keeps its last success through a failed
  // background refetch: losing the technical details mid-film raised the error
  // screen, and losing the user or the server settings could re-resolve the
  // mode and restart the stream.
  const techQuery = useQuery(mediaTechnicalDetailsQueryOpts(media));
  const { isPending: techPending, refetch: refetchTech } = techQuery;
  const techData = useLastSuccess(techQuery.data, mediaKey(media));
  const { data: watchProgressData, isPending: watchProgressPending } = useQuery(
    mediaWatchProgressQueryOpts(media),
  );
  const authUserQuery = useQuery(authUserQueryOpts());
  const authUserPending = authUserQuery.isPending;
  const user = authUserFrom(useLastSuccess(authUserQuery.data, "auth"));
  const playbackSettingsQuery = useQuery(playbackSettingsQueryOpts());
  const playbackSettingsPending = playbackSettingsQuery.isPending;
  const playbackSettingsData = useLastSuccess(playbackSettingsQuery.data, "settings");
  const serverPlaybackSettings =
    playbackSettingsFromResponse(playbackSettingsData);
  const devicePrefs = useDevicePlaybackPreferences(user?.id ?? 0);
  const userPlaybackPrefs = playbackDefaultsInput(
    devicePrefs,
    serverPlaybackSettings,
  );
  const techPayload = techData?.error === false ? techData.data : null;
  const techError = techData?.error
    ? apiErrorMessage(
        techData,
        `The ${media.kind}'s technical details could not be loaded.`,
      )
    : null;
  const techLoaded = !techPending && techPayload != null;
  const techFile = techPayload ? playbackTechnicalFile(techPayload) : null;
  const videoStreams = techPayload?.video_streams ?? [];
  const audioStreams = techPayload?.audio_streams ?? [];
  const subtitleStreams = techPayload?.subtitles ?? [];
  const chapters = techPayload?.chapters ?? [];
  const primaryVideo = techLoaded
    ? getPrimaryVideoStream(videoStreams)
    : undefined;
  const availableModes = techLoaded
    ? getAvailableModes({
        video: primaryVideo,
        videoStreamsLoaded: true,
        audioStreams,
        mimeType: techFile?.mime_type,
        maxTranscodeHeight: serverPlaybackSettings?.max_transcode_height,
      })
    : null;
  // Device preferences are synchronous, but the server's playback settings
  // decide which transcode modes this file offers (max_transcode_height) as
  // well as the download-speed recommendation, so a mode resolved before they
  // arrive could start a stream their arrival restarts. The route loader
  // fetches them alongside the technical details playback already waits for.
  const playbackPreferencesReady =
    !authUserPending && !playbackSettingsPending;
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
  const mediaDurationSec =
    unwrapFloatOrUndefined(techFile?.duration) ?? fallbackDurationSec;
  const playbackStartSec = clampPlaybackTime(
    start,
    0,
    mediaDurationSec ?? 0,
  );
  const streamAudioTrack =
    techLoaded && audioStreams.length === 0 ? null : resolvedAudioTrack;
  const requestedHlsStartSec = hlsStartTimeSec(
    isHlsPlayback,
    playbackStartSec,
  );
  // The codec declaration is part of the server's session key, so a stream
  // that starts or stops declaring HEVC (the file was replaced) is a new
  // window: nothing measured on the old session carries over.
  const hlsVideoCodecs = hlsVideoCodecsFor(resolvedMode, primaryVideo);
  const sessionWindowKey = `${mediaKey(media)}:${resolvedMode}:${streamAudioTrack ?? "none"}:${playbackSessionId}:${Math.floor(requestedHlsStartSec)}${hlsVideoCodecs ? `:${hlsVideoCodecs}` : ""}`;
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
  // Where a copied HEVC stream the browser fails to decode goes: the
  // transcode the viewer's defaults would pick among this file's.
  const transcodeModes =
    availableModes?.filter((m) => m.type === "transcode") ?? [];
  const hevcFallbackMode =
    transcodeModes.length > 0
      ? getDefaultPlaybackSettings(transcodeModes, userPlaybackPrefs).mode
      : undefined;
  const streamUrl = buildStreamUrl(
    media,
    resolvedMode,
    streamAudioTrack,
    requestedHlsStartSec,
    streamReloadKey,
    playbackSessionId,
    hlsVideoCodecs,
  );
  const modeLabel = playbackModeLabel(
    resolvedMode,
    techLoaded ? audioStreams : undefined,
  );
  const modeUnavailable =
    availableModes !== null && availableModes.length === 0;
  const directPlayAvailable =
    availableModes?.some((m) => m.id === "direct") ?? false;
  const playbackTiming = {
    isHlsPlayback,
    actualHlsStartSec,
    mediaDurationSec,
  };
  const subtitleInfo = buildSubtitleTrackInfo({
    media,
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
    techPending,
    techLoaded,
    techError,
    refetchTech,
    directPlayAvailable,
    playbackPreferencesReady,
    watchProgressData,
    watchProgressPending,
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
    mediaDurationSec,
    sessionWindowKey,
    handleActualHlsStart,
    hevcCopyDeclared: hlsVideoCodecs === "hevc",
    hevcFallbackMode,
  };
}
