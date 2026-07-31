import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useMoviePlaybackData } from "@/hooks/useMoviePlaybackData";
import {
  AUTH_USER_KEY,
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
  MOVIE_WATCH_PROGRESS_KEY,
  PLAYBACK_SETTINGS_KEY,
} from "@/lib/constants";
import {
  deriveMoviePlaybackStatus,
  shouldRebaseHlsMovieSession,
  toAbsoluteDuration,
  toAbsolutePlaybackTime,
  toMediaPlaybackTime,
} from "@/lib/movie-playback";
import type {
  AuthUser,
  PlaybackSettingsType,
  StreamModeId,
} from "@/types";

const playbackSessionId = "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4";
const authenticatedUserId = 1;

const playbackProfiles = [
  {
    id: "1080p_8mbps",
    label: "1080p · 8 Mbps",
    height: 1080,
    video_mbps: 8,
  },
  {
    id: "720p_3mbps",
    label: "720p · 3 Mbps",
    height: 720,
    video_mbps: 3,
  },
];

function nullableFloat64(value: number) {
  return { Float64: value, Valid: true };
}

function wrapperFor(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function authenticatedUser(): AuthUser {
  return {
    id: authenticatedUserId,
    name: "Playback User",
    email: "playback@example.com",
    is_admin: false,
    avatar: null,
    has_pin: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function playbackSettings(
  overrides: Partial<PlaybackSettingsType> = {},
): PlaybackSettingsType {
  return {
    profiles: playbackProfiles,
    preferred_profile: null,
    download_mbps: null,
    server_upload_mbps: null,
    hardware_acceleration_device: "cpu",
    is_admin: false,
    preferred_audio_language: null,
    preferred_subtitle_language: null,
    ...overrides,
  };
}

function seedAuthenticatedUser(queryClient: QueryClient) {
  queryClient.setQueryData([AUTH_USER_KEY], {
    error: false,
    data: { user: authenticatedUser() },
  });
}

function seedSettledPlaybackPreferences(
  queryClient: QueryClient,
  settings = playbackSettings(),
) {
  seedAuthenticatedUser(queryClient);
  queryClient.setQueryData([PLAYBACK_SETTINGS_KEY, authenticatedUserId], {
    error: false,
    data: { settings },
  });
}

function seedPreferenceResolutionMovie(
  queryClient: QueryClient,
  movieId: number,
) {
  queryClient.setQueryData([LIBRARY_MOVIE_DETAILS_KEY, movieId], {
    error: false,
    data: {
      movie: {
        title: "Preference Race",
        poster_path: { String: "", Valid: false },
        duration: nullableFloat64(600),
      },
    },
  });
  queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, movieId], {
    error: false,
    data: {
      movie: {
        mime_type: "video/mp4",
        duration: nullableFloat64(600),
      },
      video_streams: [{ codec: "h264", height: 1080 }],
      audio_streams: [
        {
          codec: "aac",
          language: { String: "eng", Valid: true },
        },
        {
          codec: "aac",
          language: { String: "spa", Valid: true },
        },
      ],
      subtitles: [
        {
          codec: "subrip",
          language: { String: "eng", Valid: true },
          title: { String: "", Valid: false },
        },
        {
          codec: "subrip",
          language: { String: "spa", Valid: true },
          title: { String: "", Valid: false },
        },
      ],
      chapters: [],
    },
  });
  queryClient.setQueryData([MOVIE_WATCH_PROGRESS_KEY, movieId], {
    error: false,
    data: null,
  });
}

function seedMovieWithoutTechnicalDetails(
  queryClient: QueryClient,
  movieId: number,
) {
  queryClient.setQueryData([LIBRARY_MOVIE_DETAILS_KEY, movieId], {
    error: false,
    data: {
      movie: {
        title: "Cold Playback",
        poster_path: { String: "", Valid: false },
        duration: nullableFloat64(600),
      },
    },
  });
  queryClient.setQueryData([MOVIE_WATCH_PROGRESS_KEY, movieId], {
    error: false,
    data: null,
  });
  seedSettledPlaybackPreferences(queryClient);
}

function createDeferredResponse() {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((resolvePromise) => {
    resolve = resolvePromise;
  });

  return { promise, resolve };
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function playbackStatus(
  data: ReturnType<typeof useMoviePlaybackData>,
  requestedMode: StreamModeId,
) {
  return deriveMoviePlaybackStatus({
    movieNotFound: data.movieNotFound,
    movieIsPending: data.movieIsPending,
    hasMovie: !!data.movie,
    requestedMode,
    techPending: data.techPending,
    playbackPreferencesReady: data.playbackPreferencesReady,
    modeUnavailable: data.modeUnavailable,
    playbackError: null,
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useMoviePlaybackData", () => {
  it("clamps a stale start before deriving every HLS timing value", () => {
    const movieId = 7;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData([LIBRARY_MOVIE_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          title: "Stale Resume",
          poster_path: { String: "", Valid: false },
          duration: nullableFloat64(120),
        },
      },
    });
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          mime_type: "video/x-matroska",
          duration: nullableFloat64(120),
        },
        video_streams: [{ codec: "h264", height: 1080 }],
        audio_streams: [{ codec: "aac" }],
        subtitles: [],
        chapters: [],
      },
    });
    queryClient.setQueryData([MOVIE_WATCH_PROGRESS_KEY, movieId], {
      error: false,
      data: null,
    });
    seedSettledPlaybackPreferences(queryClient);

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "720p_3mbps",
            audio_track: 0,
            subtitle_track: undefined,
            start: 1000,
          },
          streamReloadKey: 3,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.playbackStartSec).toBe(120);
    expect(result.current.requestedHlsStartSec).toBe(110);
    expect(result.current.actualHlsStartSec).toBe(110);
    expect(result.current.hlsPlaybackOffset).toBe(10);
    expect(result.current.streamUrl).toBe(
      `/api/movies/7/hls/720p_3mbps/playlist.m3u8?playback_session=${playbackSessionId}&start=110&audio_track=0&reload=3`,
    );
    expect(
      toAbsolutePlaybackTime(10, result.current.playbackTiming),
    ).toBe(120);
    expect(result.current.sessionWindowKey).toBe(
      `7:720p_3mbps:0:${playbackSessionId}:110`,
    );
  });

  it("keeps requested session identity while mapping playback through the measured start", () => {
    const movieId = 74;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedPreferenceResolutionMovie(queryClient, movieId);
    seedSettledPlaybackPreferences(queryClient);
    const search = {
      mode: "remux" as const,
      audio_track: 0,
      subtitle_track: 0,
      start: 600,
    };

    const { result, rerender } = renderHook(
      (props: { search: typeof search }) =>
        useMoviePlaybackData({
          movieId,
          search: props.search,
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      {
        initialProps: { search },
        wrapper: wrapperFor(queryClient),
      },
    );

    expect(result.current.requestedHlsStartSec).toBe(590);
    expect(result.current.actualHlsStartSec).toBe(590);
    expect(result.current.hlsPlaybackOffset).toBe(10);
    expect(result.current.subtitleInfo?.url).toContain("start=590");

    act(() => {
      result.current.handleActualHlsStart(581);
    });

    expect(result.current.requestedHlsStartSec).toBe(590);
    expect(result.current.actualHlsStartSec).toBe(581);
    expect(result.current.hlsPlaybackOffset).toBe(19);
    expect(result.current.streamUrl).toContain("start=590");
    expect(result.current.streamUrl).not.toContain("start=581");
    expect(result.current.sessionWindowKey).toBe(
      `74:remux:0:${playbackSessionId}:590`,
    );
    expect(result.current.subtitleInfo?.url).toContain("start=581");
    expect(
      toAbsolutePlaybackTime(19, result.current.playbackTiming),
    ).toBe(600);
    expect(toMediaPlaybackTime(600, result.current.playbackTiming)).toBe(19);
    expect(toAbsoluteDuration(19, result.current.playbackTiming)).toBe(600);
    expect(
      shouldRebaseHlsMovieSession({
        isHlsPlayback: true,
        targetTimeSec: 580,
        actualHlsStartSec: result.current.actualHlsStartSec,
        currentVideoTimeSec: 600,
      }),
    ).toBe(true);
    expect(
      shouldRebaseHlsMovieSession({
        isHlsPlayback: true,
        targetTimeSec: 581,
        actualHlsStartSec: result.current.actualHlsStartSec,
        currentVideoTimeSec: 600,
      }),
    ).toBe(false);

    const timingAfterMeasurement = result.current.playbackTiming;
    act(() => {
      result.current.handleActualHlsStart(581);
    });
    expect(result.current.playbackTiming).toBe(timingAfterMeasurement);

    rerender({
      search: {
        ...search,
        start: 500,
      },
    });
    expect(result.current.requestedHlsStartSec).toBe(490);
    expect(result.current.actualHlsStartSec).toBe(490);
    expect(result.current.hlsPlaybackOffset).toBe(10);
    expect(result.current.subtitleInfo?.url).toContain("start=490");
  });

  // Audit D9: while direct play is active the audible stream is always
  // ordinal 0, so the badge names its language instead of the generic
  // "plays as-is" claim.
  it("names the playing audio language in the direct mode label", () => {
    const movieId = 22;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedPreferenceResolutionMovie(queryClient, movieId);
    seedSettledPlaybackPreferences(queryClient);

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 0,
            subtitle_track: undefined,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.resolvedMode).toBe("direct");
    expect(result.current.modeLabel).toBe("Original file — English audio");
  });

  it("keeps the generic mode label for non-direct playback", () => {
    const movieId = 23;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedPreferenceResolutionMovie(queryClient, movieId);
    seedSettledPlaybackPreferences(queryClient);

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "remux",
            audio_track: 1,
            subtitle_track: undefined,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.resolvedMode).toBe("remux");
    expect(result.current.modeLabel).toBe("Original video, adjusted audio");
  });

  // Audit matrix row 18b (D17): loaded metadata with zero video streams must
  // not leave direct play on offer — the player never mounts and no /stream
  // request can be issued.
  it("marks every mode unavailable when metadata has no video stream", () => {
    const movieId = 21;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData([LIBRARY_MOVIE_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          title: "No Video Streams",
          poster_path: { String: "", Valid: false },
          duration: nullableFloat64(600),
        },
      },
    });
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          mime_type: "video/mp4",
          duration: nullableFloat64(600),
        },
        video_streams: [],
        audio_streams: [{ codec: "aac" }],
        subtitles: [],
        chapters: [],
      },
    });
    queryClient.setQueryData([MOVIE_WATCH_PROGRESS_KEY, movieId], {
      error: false,
      data: null,
    });
    seedSettledPlaybackPreferences(queryClient);

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 0,
            subtitle_track: undefined,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.modeUnavailable).toBe(true);
    expect(result.current.directPlayAvailable).toBe(false);
    expect(playbackStatus(result.current, "direct").kind).toBe(
      "modeUnavailable",
    );
  });

  it("drops a subtitle_track URL param that points at a bitmap subtitle", () => {
    const movieId = 8;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData([LIBRARY_MOVIE_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          title: "Bitmap Subs",
          poster_path: { String: "", Valid: false },
          duration: nullableFloat64(120),
        },
      },
    });
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          mime_type: "video/mp4",
          duration: nullableFloat64(120),
        },
        video_streams: [{ codec: "h264", height: 1080 }],
        audio_streams: [{ codec: "aac" }],
        subtitles: [
          {
            codec: "hdmv_pgs_subtitle",
            language: { String: "eng", Valid: true },
            title: { String: "", Valid: false },
          },
        ],
        chapters: [],
      },
    });
    queryClient.setQueryData([MOVIE_WATCH_PROGRESS_KEY, movieId], {
      error: false,
      data: null,
    });
    seedSettledPlaybackPreferences(queryClient);

    const onSyncSearch = vi.fn();
    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 0,
            subtitle_track: 0,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch,
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.resolvedSubtitleTrack).toBeNull();
    expect(result.current.subtitleInfo).toBeNull();
    expect(onSyncSearch).toHaveBeenCalledWith({
      mode: "direct",
      audioTrack: 0,
      subtitleTrack: null,
    });
  });

  it("applies subtitle preferences only when the route selection is omitted", async () => {
    const movieId = 81;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedPreferenceResolutionMovie(queryClient, movieId);
    seedSettledPlaybackPreferences(
      queryClient,
      playbackSettings({ preferred_subtitle_language: "es" }),
    );
    const onSyncSearch = vi.fn();

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 0,
            subtitle_track: undefined,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch,
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.resolvedSubtitleTrack).toBe(1);
    await waitFor(() => {
      expect(onSyncSearch).toHaveBeenCalledWith({
        mode: "direct",
        audioTrack: 0,
        subtitleTrack: 1,
      });
    });
  });

  it("keeps an explicit subtitle-off route selection over preferences", () => {
    const movieId = 82;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedPreferenceResolutionMovie(queryClient, movieId);
    seedSettledPlaybackPreferences(
      queryClient,
      playbackSettings({ preferred_subtitle_language: "es" }),
    );
    const onSyncSearch = vi.fn();

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 0,
            subtitle_track: "off",
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch,
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.resolvedSubtitleTrack).toBeNull();
    expect(onSyncSearch).not.toHaveBeenCalled();
  });

  it("streams a direct-play deep link through remux when it asks for a non-first audio track", () => {
    const movieId = 9;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData([LIBRARY_MOVIE_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          title: "Multi Audio",
          poster_path: { String: "", Valid: false },
          duration: nullableFloat64(600),
        },
      },
    });
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, movieId], {
      error: false,
      data: {
        movie: {
          mime_type: "video/mp4",
          duration: nullableFloat64(600),
        },
        video_streams: [{ codec: "h264", height: 1080 }],
        audio_streams: [{ codec: "aac" }, { codec: "aac" }],
        subtitles: [],
        chapters: [],
      },
    });
    queryClient.setQueryData([MOVIE_WATCH_PROGRESS_KEY, movieId], {
      error: false,
      data: null,
    });
    seedSettledPlaybackPreferences(queryClient);
    const onSyncSearch = vi.fn();

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 1,
            subtitle_track: undefined,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch,
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.resolvedMode).toBe("remux");
    expect(result.current.resolvedAudioTrack).toBe(1);
    expect(result.current.streamUrl).toBe(
      `/api/movies/9/hls/remux/playlist.m3u8?playback_session=${playbackSessionId}&start=0&audio_track=1`,
    );
    expect(onSyncSearch).toHaveBeenCalledWith({
      mode: "remux",
      audioTrack: 1,
      subtitleTrack: null,
    });
  });

  it("uses provisional remux while valid non-first-audio metadata is pending", async () => {
    const movieId = 91;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedMovieWithoutTechnicalDetails(queryClient, movieId);
    const technicalDetailsRequest = createDeferredResponse();
    vi.stubGlobal("fetch", vi.fn(() => technicalDetailsRequest.promise));

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 1,
            subtitle_track: "off",
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.techPending).toBe(true);
    expect(result.current.resolvedMode).toBe("remux");
    expect(result.current.resolvedAudioTrack).toBe(1);
    expect(result.current.streamUrl).toBe(
      `/api/movies/91/hls/remux/playlist.m3u8?playback_session=${playbackSessionId}&start=0&audio_track=1`,
    );
    expect(result.current.sessionWindowKey).toBe(
      `91:remux:1:${playbackSessionId}:0`,
    );
    expect(playbackStatus(result.current, "direct")).toEqual({
      kind: "loading",
      message: "Preparing playback...",
    });

    technicalDetailsRequest.resolve(
      jsonResponse({
        error: false,
        data: {
          movie: {
            mime_type: "video/mp4",
            duration: nullableFloat64(600),
          },
          video_streams: [{ codec: "h264", height: 1080 }],
          audio_streams: [{ codec: "aac" }, { codec: "aac" }],
          subtitles: [],
          chapters: [],
        },
      }),
    );

    await waitFor(() => {
      expect(result.current.techPending).toBe(false);
      expect(result.current.resolvedMode).toBe("remux");
      expect(result.current.resolvedAudioTrack).toBe(1);
    });
    expect(result.current.streamUrl).toBe(
      `/api/movies/91/hls/remux/playlist.m3u8?playback_session=${playbackSessionId}&start=0&audio_track=1`,
    );
  });

  it("never falls back to the raw stream after technical-details failure", () => {
    const movieId = 92;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedMovieWithoutTechnicalDetails(queryClient, movieId);
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, movieId], {
      error: true,
      message: "Technical details unavailable",
    });

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 1,
            subtitle_track: "off",
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.techPending).toBe(false);
    expect(result.current.resolvedMode).toBe("remux");
    expect(result.current.resolvedAudioTrack).toBe(1);
    expect(result.current.streamUrl).toContain(
      "/api/movies/92/hls/remux/playlist.m3u8",
    );
    expect(result.current.streamUrl).toContain("audio_track=1");
    expect(result.current.streamUrl).not.toContain("/stream");
  });

  it("keeps cold direct playback preparing until technical details resolve", async () => {
    const movieId = 93;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedMovieWithoutTechnicalDetails(queryClient, movieId);
    const technicalDetailsRequest = createDeferredResponse();
    vi.stubGlobal("fetch", vi.fn(() => technicalDetailsRequest.promise));

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "direct",
            audio_track: 0,
            subtitle_track: "off",
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch: vi.fn(),
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.techPending).toBe(true);
    expect(result.current.resolvedMode).toBe("direct");
    // The player must not mount (and no /stream request may fire) before
    // eligibility is known — audit D16.
    expect(playbackStatus(result.current, "direct")).toEqual({
      kind: "loading",
      message: "Preparing playback...",
    });

    technicalDetailsRequest.resolve(
      jsonResponse({
        error: false,
        data: {
          movie: {
            mime_type: "video/mp4",
            duration: nullableFloat64(600),
          },
          video_streams: [{ codec: "h264", height: 1080 }],
          audio_streams: [{ codec: "aac" }],
          subtitles: [],
          chapters: [],
        },
      }),
    );
    await waitFor(() => {
      expect(result.current.techPending).toBe(false);
    });
    expect(result.current.resolvedMode).toBe("direct");
    expect(result.current.streamUrl).toBe("/api/movies/93/stream");
    expect(playbackStatus(result.current, "direct")).toEqual({ kind: "ready" });
  });

  it("waits for auth and playback preferences before normalizing stale deep-link values", async () => {
    const movieId = 10;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedPreferenceResolutionMovie(queryClient, movieId);

    const authRequest = createDeferredResponse();
    const playbackSettingsRequest = createDeferredResponse();
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      if (String(input) === "/api/auth/user") {
        return authRequest.promise;
      }
      if (String(input) === "/api/settings/playback") {
        return playbackSettingsRequest.promise;
      }
      throw new Error(`Unexpected request: ${String(input)}`);
    });
    vi.stubGlobal("fetch", fetchMock);
    const onSyncSearch = vi.fn();

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "2160p_16mbps",
            audio_track: 99,
            subtitle_track: 99,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch,
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    expect(result.current.resolvedMode).toBe("direct");
    expect(result.current.resolvedAudioTrack).toBe(0);
    expect(result.current.playbackPreferencesReady).toBe(false);
    expect(playbackStatus(result.current, "2160p_16mbps")).toEqual({
      kind: "loading",
      message: "Preparing playback...",
    });
    expect(onSyncSearch).not.toHaveBeenCalled();

    authRequest.resolve(
      jsonResponse({
        error: false,
        data: { user: authenticatedUser() },
      }),
    );

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/settings/playback",
        expect.objectContaining({
          method: "GET",
          credentials: "include",
        }),
      );
    });
    expect(result.current.playbackPreferencesReady).toBe(false);
    expect(playbackStatus(result.current, "2160p_16mbps")).toEqual({
      kind: "loading",
      message: "Preparing playback...",
    });
    expect(onSyncSearch).not.toHaveBeenCalled();

    playbackSettingsRequest.resolve(
      jsonResponse({
        error: false,
        data: {
          settings: playbackSettings({
            preferred_profile: "720p_3mbps",
            preferred_audio_language: "es",
            preferred_subtitle_language: "es",
          }),
        },
      }),
    );

    await waitFor(() => {
      expect(result.current.resolvedMode).toBe("720p_3mbps");
      expect(result.current.resolvedAudioTrack).toBe(1);
      expect(result.current.resolvedSubtitleTrack).toBe(1);
      expect(result.current.playbackPreferencesReady).toBe(true);
      expect(playbackStatus(result.current, "2160p_16mbps")).toEqual({
        kind: "ready",
      });
      expect(onSyncSearch).toHaveBeenCalledTimes(1);
    });
    expect(onSyncSearch).toHaveBeenCalledWith({
      mode: "720p_3mbps",
      audioTrack: 1,
      subtitleTrack: 1,
    });
  });

  it("normalizes through existing defaults after playback settings fail", async () => {
    const movieId = 11;
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    seedPreferenceResolutionMovie(queryClient, movieId);
    seedAuthenticatedUser(queryClient);

    const playbackSettingsRequest = createDeferredResponse();
    const fetchMock = vi.fn(() => playbackSettingsRequest.promise);
    vi.stubGlobal("fetch", fetchMock);
    const onSyncSearch = vi.fn();

    const { result } = renderHook(
      () =>
        useMoviePlaybackData({
          movieId,
          search: {
            mode: "2160p_16mbps",
            audio_track: 99,
            subtitle_track: 99,
            start: 0,
          },
          streamReloadKey: 0,
          playbackSessionId,
          onSyncSearch,
        }),
      { wrapper: wrapperFor(queryClient) },
    );

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
    expect(result.current.playbackPreferencesReady).toBe(false);
    expect(playbackStatus(result.current, "2160p_16mbps")).toEqual({
      kind: "loading",
      message: "Preparing playback...",
    });
    expect(onSyncSearch).not.toHaveBeenCalled();

    playbackSettingsRequest.resolve(
      jsonResponse(
        {
          error: true,
          message: "Playback settings unavailable",
        },
        500,
      ),
    );

    await waitFor(() => {
      expect(result.current.playbackPreferencesReady).toBe(true);
      expect(result.current.resolvedMode).toBe("direct");
      expect(result.current.resolvedAudioTrack).toBe(0);
      expect(result.current.resolvedSubtitleTrack).toBeNull();
      expect(playbackStatus(result.current, "2160p_16mbps")).toEqual({
        kind: "ready",
      });
      expect(onSyncSearch).toHaveBeenCalledTimes(1);
    });
    expect(onSyncSearch).toHaveBeenCalledWith({
      mode: "direct",
      audioTrack: 0,
      subtitleTrack: null,
    });
  });
});
