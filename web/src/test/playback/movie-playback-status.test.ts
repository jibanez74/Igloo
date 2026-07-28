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
          techPending: false,
          playbackPreferencesReady: false,
          modeUnavailable: false,
          playbackError: null,
        }),
      ).toEqual({ kind: "loading", message: "Preparing playback..." });
    },
  );

  // Every mode waits for technical details — direct included, or a cold deep
  // link would request /stream before eligibility is known (audit D16).
  it.each<StreamModeId>(["direct", "remux", "720p_3mbps"])(
    "keeps a %s deep link preparing while technical details are pending",
    requestedMode => {
      expect(
        deriveMoviePlaybackStatus({
          movieNotFound: false,
          movieIsPending: false,
          hasMovie: true,
          requestedMode,
          techPending: true,
          playbackPreferencesReady: true,
          modeUnavailable: false,
          playbackError: null,
        }),
      ).toEqual({ kind: "loading", message: "Preparing playback..." });
    },
  );

  it("reports ready once preferences and technical details resolve", () => {
    expect(
      deriveMoviePlaybackStatus({
        movieNotFound: false,
        movieIsPending: false,
        hasMovie: true,
        requestedMode: "direct",
        techPending: false,
        playbackPreferencesReady: true,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({ kind: "ready" });
  });
});
