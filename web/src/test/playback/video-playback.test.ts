import { describe, expect, it } from "vitest";
import {
  EPISODE_TECHNICAL_DETAILS_KEY,
  EPISODE_WATCH_PROGRESS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
  MOVIE_WATCH_PROGRESS_KEY,
} from "@/lib/constants";
import { episodeMediaRef, mediaApiBasePath, mediaKey, movieMediaRef } from "@/lib/media-ref";
import {
  mediaTechnicalDetailsQueryOpts,
  mediaWatchProgressQueryKey,
  movieTechnicalDetailsQueryOpts,
} from "@/lib/query-opts";
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

describe("media query keys", () => {
  it("caches a movie and an episode with the same id separately", () => {
    expect(mediaWatchProgressQueryKey(movieMediaRef(12))).toEqual([
      MOVIE_WATCH_PROGRESS_KEY,
      12,
    ]);
    expect(mediaWatchProgressQueryKey(episodeMediaRef(12))).toEqual([
      EPISODE_WATCH_PROGRESS_KEY,
      12,
    ]);
    expect(mediaTechnicalDetailsQueryOpts(episodeMediaRef(12)).queryKey).toEqual([
      EPISODE_TECHNICAL_DETAILS_KEY,
      12,
    ]);
  });

  it("shares the movie technical-details entry with the details page", () => {
    expect(mediaTechnicalDetailsQueryOpts(movieMediaRef(12)).queryKey).toEqual(
      movieTechnicalDetailsQueryOpts(12).queryKey,
    );
    expect(mediaTechnicalDetailsQueryOpts(movieMediaRef(12)).queryKey).toEqual([
      MOVIE_TECHNICAL_DETAILS_KEY,
      12,
    ]);
  });
});
