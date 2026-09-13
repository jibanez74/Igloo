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
        playbackPreferencesReady: false,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({ kind: "loading", message: "Loading episode..." });
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
        playbackPreferencesReady: true,
        modeUnavailable: false,
        playbackError: null,
      }),
    ).toEqual({ kind: "ready" });
  });
});
