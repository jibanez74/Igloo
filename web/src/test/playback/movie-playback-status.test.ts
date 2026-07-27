import { describe, expect, it } from "vitest";
import { deriveMoviePlaybackStatus } from "@/lib/movie-playback";
import type { StreamModeId } from "@/types";

describe("deriveMoviePlaybackStatus", () => {
  it.each<StreamModeId>(["direct", "720p_3mbps"])(
    "keeps a %s deep link preparing while playback preferences are pending",
    requestedMode => {
      expect(
        deriveMoviePlaybackStatus({
          movieNotFound: false,
          movieIsPending: false,
          hasMovie: true,
          requestedMode,
          effectiveMode: requestedMode,
          techPending: false,
          playbackPreferencesReady: false,
          modeUnavailable: false,
          playbackError: null,
        }),
      ).toEqual({ kind: "loading", message: "Preparing playback..." });
    },
  );

  it("keeps provisional remux preparing while technical details are pending", () => {
    expect(
      deriveMoviePlaybackStatus({
        movieNotFound: false,
        movieIsPending: false,
        hasMovie: true,
        requestedMode: "direct",
        effectiveMode: "remux",
        techPending: true,
        playbackPreferencesReady: true,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({ kind: "loading", message: "Preparing playback..." });
  });

  it("keeps cold first-audio direct playback ready", () => {
    expect(
      deriveMoviePlaybackStatus({
        movieNotFound: false,
        movieIsPending: false,
        hasMovie: true,
        requestedMode: "direct",
        effectiveMode: "direct",
        techPending: true,
        playbackPreferencesReady: true,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({ kind: "ready" });
  });
});
