import type { ReactNode } from "react";
import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { useMoviePlaybackData } from "@/hooks/useMoviePlaybackData";
import {
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
  MOVIE_WATCH_PROGRESS_KEY,
} from "@/lib/constants";
import { toAbsolutePlaybackTime } from "@/lib/movie-playback";

const playbackSessionId = "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4";

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
    expect(result.current.hlsStartSec).toBe(110);
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
});
