import { describe, expect, it } from "vitest";
import { episodeMediaRef, mediaApiBasePath, mediaKey, movieMediaRef } from "@/lib/media-ref";
import { buildStreamUrl } from "@/lib/video-playback";

const session = "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4";

describe("media refs", () => {
  it("routes movies and episodes to their own API families", () => {
    expect(mediaApiBasePath(movieMediaRef(12))).toBe("/api/movies/12");
    expect(mediaApiBasePath(episodeMediaRef(12))).toBe("/api/shows/episodes/12");
  });

  it("keys a movie and an episode with the same id apart", () => {
    expect(mediaKey(movieMediaRef(12))).toBe("movie:12");
    expect(mediaKey(episodeMediaRef(12))).toBe("episode:12");
  });
});

describe("buildStreamUrl", () => {
  it("direct-plays an episode from the episode stream route", () => {
    expect(buildStreamUrl(episodeMediaRef(9), "direct", 0, 0, 0, session)).toBe(
      "/api/shows/episodes/9/stream",
    );
  });

  it("builds episode HLS manifests with the same query contract as movies", () => {
    expect(
      buildStreamUrl(episodeMediaRef(9), "720p_3mbps", 1, 110.6, 2, session),
    ).toBe(
      `/api/shows/episodes/9/hls/720p_3mbps/playlist.m3u8?playback_session=${session}&start=110&audio_track=1&reload=2`,
    );
    expect(
      buildStreamUrl(movieMediaRef(9), "720p_3mbps", null, 0, 0, session),
    ).toBe(
      `/api/movies/9/hls/720p_3mbps/playlist.m3u8?playback_session=${session}&start=0`,
    );
  });
});
