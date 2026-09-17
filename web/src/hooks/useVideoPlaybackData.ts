import { useEffect, useEffectEvent, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { STREAM_MODES } from "@/lib/constants";
import { mediaKey } from "@/lib/media-ref";
import {
  directPlayModeLabel,
  getAvailableModes,
  getPrimaryVideoStream,
  playbackDefaultsInput,
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
  authUserQueryOpts,
  mediaTechnicalDetailsQueryOpts,
  mediaWatchProgressQueryOpts,
  playbackSettingsQueryOpts,
  playbackTechnicalFile,
} from "@/lib/query-opts";
import { useDevicePlaybackPreferences } from "@/hooks/useDevicePlaybackPreferences";
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

  const { data: techData, isPending: techPending } = useQuery(
    mediaTechnicalDetailsQueryOpts(media),
  );
  const { data: watchProgressData, isPending: watchProgressPending } = useQuery(
    mediaWatchProgressQueryOpts(media),
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
  const techPayload = techData?.error === false ? techData.data : null;
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
  const sessionWindowKey = `${mediaKey(media)}:${resolvedMode}:${streamAudioTrack ?? "none"}:${playbackSessionId}:${Math.floor(requestedHlsStartSec)}`;
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
  const streamUrl = buildStreamUrl(
    media,
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
  };
}
