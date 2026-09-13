import type { QueryClient } from "@tanstack/react-query";
import {
  getAvailableModes,
  getDefaultPlaybackSettings,
  getPrimaryVideoStream,
  playbackDefaultsInput,
} from "@/lib/playback";
import { getDevicePlaybackPreferences } from "@/lib/playback-preferences";
import {
  authUserQueryOpts,
  mediaTechnicalDetailsQueryOpts,
  playbackSettingsQueryOpts,
  playbackTechnicalFile,
} from "@/lib/query-opts";
import {
  playbackSettingsToPlaySearch,
  type PlaySearchParams,
} from "@/lib/route-search";
import type { PlaybackMediaRef } from "@/types/playback";

type PlayRouteLoaderArgs = {
  queryClient: QueryClient;
  media: PlaybackMediaRef;
  /** Warms the media's own header query alongside the playback ones. */
  ensureDetails: () => Promise<unknown>;
  deps: Pick<PlaySearchParams, "mode" | "start" | "autoplay">;
};

/**
 * The play routes' shared loader. A URL that already carries a mode warms the
 * caches without blocking navigation and resolves to nothing. A URL without
 * one resolves the player's default settings from the file, the device and
 * the server catalog, and returns the canonical search the route should
 * redirect to; null means the defaults could not be resolved and the page
 * should render as is (the component reports the failure).
 */
export async function loadPlayRoute({
  queryClient,
  media,
  ensureDetails,
  deps,
}: PlayRouteLoaderArgs): Promise<PlaySearchParams | null> {
  if (deps.mode !== undefined) {
    // The player waits for technical details before requesting media
    // (audit D16), so warm the caches for URLs that already carry a mode —
    // but without blocking navigation, or the player skeleton would not
    // render until every query resolves. The component's own queries join
    // these in-flight fetches; errors surface through them.
    void (async () => {
      const authRes = await queryClient.ensureQueryData(authUserQueryOpts());
      if (authRes.error) return;
      await Promise.all([
        ensureDetails(),
        queryClient.ensureQueryData(mediaTechnicalDetailsQueryOpts(media)),
        queryClient.ensureQueryData(playbackSettingsQueryOpts()),
      ]);
    })().catch(() => {});
    return null;
  }

  const authRes = await queryClient.ensureQueryData(authUserQueryOpts());
  if (authRes.error) return null;

  const [, techRes, playbackRes] = await Promise.all([
    ensureDetails(),
    queryClient.ensureQueryData(mediaTechnicalDetailsQueryOpts(media)),
    queryClient.ensureQueryData(playbackSettingsQueryOpts()),
  ]);

  const techData = techRes.error === false ? techRes.data : null;
  if (!techData) return null;

  const videoStreams = techData.video_streams ?? [];
  const audioStreams = techData.audio_streams ?? [];
  const subtitleStreams = techData.subtitles ?? [];
  const primaryVideo = getPrimaryVideoStream(videoStreams);
  const availableModes = getAvailableModes({
    video: primaryVideo,
    videoStreamsLoaded: true,
    audioStreams,
    mimeType: playbackTechnicalFile(techData).mime_type,
  });
  if (availableModes.length === 0) return null;

  const serverSettings =
    playbackRes.error === false ? (playbackRes.data?.settings ?? null) : null;
  const resolved = getDefaultPlaybackSettings(
    availableModes,
    playbackDefaultsInput(
      getDevicePlaybackPreferences(authRes.data.user.id),
      serverSettings,
    ),
    audioStreams,
    subtitleStreams,
  );

  return {
    ...playbackSettingsToPlaySearch(resolved),
    start: deps.start ?? 0,
    autoplay: deps.autoplay,
  };
}
