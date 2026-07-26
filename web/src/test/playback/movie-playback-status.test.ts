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
});
