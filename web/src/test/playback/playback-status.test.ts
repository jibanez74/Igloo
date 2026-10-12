import { describe, expect, it } from "vitest";
import { derivePlaybackStatus } from "@/lib/video-playback";
import type { StreamModeId } from "@/types";

describe("derivePlaybackStatus", () => {
  it.each<StreamModeId>(["direct", "720p_3mbps"])(
    "keeps a %s deep link preparing while playback preferences are pending",
    requestedMode => {
      expect(
        derivePlaybackStatus({
          mediaNoun: "movie",
          notFound: false,
          detailsPending: false,
          hasDetails: true,
          requestedMode,
          techPending: false,
          techError: null,
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
        derivePlaybackStatus({
          mediaNoun: "movie",
          notFound: false,
          detailsPending: false,
          hasDetails: true,
          requestedMode,
          techPending: true,
          techError: null,
          playbackPreferencesReady: true,
          modeUnavailable: false,
          playbackError: null,
        }),
      ).toEqual({ kind: "loading", message: "Preparing playback..." });
    },
  );

  it("names the media kind while its header loads", () => {
    expect(
      derivePlaybackStatus({
        mediaNoun: "episode",
        notFound: false,
        detailsPending: true,
        hasDetails: false,
        requestedMode: "direct",
        techPending: true,
        techError: null,
        playbackPreferencesReady: false,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({ kind: "loading", message: "Loading episode..." });
  });

  // Playback never requests a stream before technical details resolve
  // (docs/ffmpeg.md), so their failure is the player's error, not a guess.
  it("reports a failure to load technical details instead of starting", () => {
    expect(
      derivePlaybackStatus({
        mediaNoun: "movie",
        notFound: false,
        detailsPending: false,
        hasDetails: true,
        requestedMode: "direct",
        techPending: false,
        techError: "The movie's technical details could not be loaded.",
        playbackPreferencesReady: true,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({
      kind: "error",
      message: "The movie's technical details could not be loaded.",
    });
  });

  it("reports ready once preferences and technical details resolve", () => {
    expect(
      derivePlaybackStatus({
        mediaNoun: "movie",
        notFound: false,
        detailsPending: false,
        hasDetails: true,
        requestedMode: "direct",
        techPending: false,
        techError: null,
        playbackPreferencesReady: true,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({ kind: "ready" });
  });
});
