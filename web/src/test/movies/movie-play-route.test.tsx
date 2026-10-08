import { focusManager, onlineManager } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { MEDIA_ERR_SRC_NOT_SUPPORTED } from "@/lib/constants";
import type {
  LibraryMovieDetailsResponse,
  MovieTechnicalDetailsResponse,
  PrerollTrailer,
} from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { stubMediaElement } from "../helpers/dom";
import {
  authUser,
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";
import { audioStream, videoStream } from "../helpers/tech-details";

// The YouTube IFrame API is never loaded in tests; the pre-roll only needs
// the hook's surface to render its chrome. The clock is mutable so a test can
// place the last trailer inside the movie's warm-up window.
const youtubeClock = vi.hoisted(() => ({ currentTime: 0, duration: 120 }));

vi.mock("@/hooks/useYouTubePlayer", () => ({
  useYouTubePlayer: () => ({
    containerRef: () => {},
    isReady: true,
    isPlaying: true,
    currentTime: youtubeClock.currentTime,
    duration: youtubeClock.duration,
    volume: 100,
    isMuted: false,
    error: null,
    play: vi.fn(),
    pause: vi.fn(),
    togglePlay: vi.fn(),
    seekTo: vi.fn(),
    seekForward: vi.fn(),
    seekBackward: vi.fn(),
    setVolume: vi.fn(),
    mute: vi.fn(),
    unmute: vi.fn(),
    toggleMute: vi.fn(),
    retry: vi.fn(),
  }),
}));

// Decided once at module load from the browser; jsdom has no MediaSource, so
// a test that reaches HLS opts into the native path the player uses on Safari.
const hlsSupport = vi.hoisted(() => ({ native: false }));

vi.mock("@/lib/playback", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/playback")>();
  return {
    ...actual,
    get prefersNativeHLS() {
      return hlsSupport.native;
    },
  };
});

const MOVIE_ID = 7;
const PLAY_PATH = `/movies/${MOVIE_ID}/play`;
const FRESH_SEARCH = "mode=direct&audio_track=0&subtitle_track=off&start=0";

function renderMovieRoute(search: string) {
  return renderRoute(`${PLAY_PATH}?${search}`, {
    wrapper: (children) => (
      <AudioPlayerProvider>{children}</AudioPlayerProvider>
    ),
  });
}

const movieDetails: LibraryMovieDetailsResponse = {
  movie: {
    id: MOVIE_ID,
    title: "Signal Fire",
    adult: false,
    tmdb_id: nullableInt64(1007),
    imdb_id: nullableString("tt1007"),
    poster_path: nullableString("/poster.jpg"),
    backdrop_path: nullableString("/backdrop.jpg"),
    language: nullableString("en"),
    year: nullableInt64(2024),
    release_date: nullableString("2024-05-01"),
    overview: nullableString("A lighthouse keeper."),
    tag_line: nullableString(""),
    certification: nullableString("PG-13"),
    critic_rating: nullableFloat64(80),
    audience_rating: nullableFloat64(75),
    revenue: nullableFloat64(0),
    budget: nullableFloat64(0),
    run_time: nullableInt64(100),
    duration: nullableFloat64(6000),
  },
  cast: [],
  crew: [],
  genres: [],
  production_companies: [],
  extra_videos: [],
};

const technicalDetails: MovieTechnicalDetailsResponse = {
  movie: {
    file_name: "signal-fire.mp4",
    size: 1024,
    container: "mp4",
    mime_type: "video/mp4",
    run_time: nullableInt64(100),
    duration: nullableFloat64(6000),
  },
  video_streams: [videoStream({ movie_id: MOVIE_ID })],
  audio_streams: [audioStream({ movie_id: MOVIE_ID, is_default: true })],
  subtitles: [],
  chapters: [],
};

function trailer(key: string, title: string): PrerollTrailer {
  return {
    title,
    youtube_key: key,
    source: "library",
    movie_id: 9,
    tmdb_id: null,
  };
}

const twoTrailers = [
  trailer("one", "Glacier Run"),
  trailer("two", "North Light"),
];

type MovieApiOptions = {
  /** The queue the preroll endpoint answers with; "fail" answers a 500. */
  preroll?: PrerollTrailer[] | "fail";
  progress?: { progress_sec: number; duration_sec: number };
};

function mockMovieApi(options: MovieApiOptions = {}) {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";

    if (url === "/api/auth/user") return jsonResponse(authUser());
    if (url === "/api/notifications/unread-count") {
      return jsonResponse({ error: false, data: { unread_count: 0 } });
    }
    if (url === "/api/settings/playback") {
      return jsonResponse({
        error: false,
        data: { settings: { profiles: [], server_upload_mbps: null } },
      });
    }
    if (url === `/api/movies/details/${MOVIE_ID}`) {
      return jsonResponse({ error: false, data: movieDetails });
    }
    if (url === `/api/movies/${MOVIE_ID}/technical-details`) {
      return jsonResponse({ error: false, data: technicalDetails });
    }
    if (url === `/api/movies/${MOVIE_ID}/preroll`) {
      if (options.preroll === "fail") {
        return jsonResponse(
          { error: true, message: "failed to build the trailer queue" },
          500,
        );
      }
      return jsonResponse({
        error: false,
        data: { trailers: options.preroll ?? [] },
      });
    }
    if (url === `/api/movies/${MOVIE_ID}/watch-progress`) {
      if (method === "DELETE") {
        return jsonResponse({ error: false, data: { cleared: true } });
      }
      if (method === "PUT") {
        return jsonResponse({ error: false, data: { watched: false } });
      }
      return jsonResponse({
        error: false,
        data: options.progress
          ? {
              ...options.progress,
              watched: false,
              updated_at: "2026-09-01T00:00:00Z",
            }
          : {
              progress_sec: null,
              duration_sec: null,
              watched: false,
              updated_at: null,
            },
      });
    }

    return jsonResponse(
      { error: true, message: `Unexpected request: ${url}` },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function requestsTo(
  fetchMock: ReturnType<typeof mockMovieApi>,
  path: string,
  method = "GET",
) {
  return fetchMock.mock.calls.filter(
    ([input, init]) =>
      requestURL(input) === path && (init?.method ?? "GET") === method,
  );
}

const prerollRegion = () =>
  screen.queryByRole("region", { name: "Trailers before the movie" });

const playSpy = () => vi.mocked(window.HTMLMediaElement.prototype.play);

/** The movie's video element once it is mounted. */
async function movieVideo() {
  await waitFor(() => expect(document.querySelector("video")).not.toBeNull());
  return document.querySelector("video") as HTMLVideoElement;
}

/** jsdom never loads media, so a test says when the stream could play. */
function reportCanPlay(video: HTMLVideoElement) {
  act(() => {
    video.dispatchEvent(new Event("canplay"));
  });
}

describe("movie play route trailer pre-roll", () => {
  beforeEach(() => {
    stubMediaElement();
    youtubeClock.currentTime = 0;
    youtubeClock.duration = 120;
    hlsSupport.native = false;
  });

  it("starts the movie straight away, paused, on an empty queue", async () => {
    const fetchMock = mockMovieApi({ preroll: [] });

    await renderMovieRoute(FRESH_SEARCH);

    await screen.findByRole("region", { name: "Video player for Signal Fire" });
    const video = await movieVideo();
    expect(prerollRegion()).toBeNull();
    // The backdrop fills the frame instead of a black box until play starts.
    expect(video).toHaveAttribute("poster", "/api/tmdb/images/w1280/backdrop.jpg");
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/preroll`),
    ).toHaveLength(1);

    // No trailer played, so nothing tells the movie to play on its own.
    reportCanPlay(video);
    expect(playSpy()).not.toHaveBeenCalled();
  });

  it("starts the movie, paused, when the queue request fails", async () => {
    mockMovieApi({ preroll: "fail" });

    await renderMovieRoute(FRESH_SEARCH);

    await screen.findByRole("region", { name: "Video player for Signal Fire" });
    const video = await movieVideo();
    expect(prerollRegion()).toBeNull();

    reportCanPlay(video);
    expect(playSpy()).not.toHaveBeenCalled();
  });

  it("plays the movie on its own once the trailers finish", async () => {
    mockMovieApi({ preroll: twoTrailers });

    await renderMovieRoute(FRESH_SEARCH);

    await screen.findByRole("region", { name: "Trailers before the movie" });
    fireEvent.click(screen.getByRole("button", { name: /^Start movie/ }));

    const video = await movieVideo();
    reportCanPlay(video);
    await waitFor(() => expect(playSpy()).toHaveBeenCalledTimes(1));
  });

  it("holds a direct-play fallback's autoplay until the last trailer ends", async () => {
    // The only trailer is already inside its final seconds, so the movie
    // warms up underneath it straight away.
    youtubeClock.currentTime = 110;
    mockMovieApi({ preroll: [trailer("one", "Glacier Run")] });
    // jsdom has no MediaSource; native HLS keeps the remux stream on the
    // video element itself.
    hlsSupport.native = true;

    const { router } = await renderMovieRoute(FRESH_SEARCH);

    await screen.findByRole("region", { name: "Trailers before the movie" });
    const warmed = await movieVideo();

    // The browser cannot play the file directly: the player switches to
    // remux and arms autoplay for the replacement stream. jsdom has no
    // MediaError, which the player's error handler reads.
    vi.stubGlobal("MediaError", {
      MEDIA_ERR_ABORTED: 1,
      MEDIA_ERR_NETWORK: 2,
      MEDIA_ERR_DECODE: 3,
      MEDIA_ERR_SRC_NOT_SUPPORTED,
    });
    Object.defineProperty(warmed, "error", {
      configurable: true,
      value: { code: MEDIA_ERR_SRC_NOT_SUPPORTED },
    });
    act(() => {
      warmed.dispatchEvent(new Event("error"));
    });
    await waitFor(() => {
      const search = router.state.location.search as Record<string, unknown>;
      expect(search.mode).toBe("remux");
    });

    reportCanPlay(await movieVideo());
    expect(prerollRegion()).not.toBeNull();
    expect(playSpy()).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /^Start movie/ }));
    await waitFor(() => expect(prerollRegion()).toBeNull());
    reportCanPlay(await movieVideo());
    await waitFor(() => expect(playSpy()).toHaveBeenCalledTimes(1));
  });

  it("plays the trailers before mounting the movie, saving no progress meanwhile", async () => {
    const fetchMock = mockMovieApi({ preroll: twoTrailers });

    await renderMovieRoute(FRESH_SEARCH);

    const region = await screen.findByRole("region", {
      name: "Trailers before the movie",
    });
    expect(region).toHaveTextContent("Trailer 1 of 2");
    expect(region).toHaveTextContent("Glacier Run");
    expect(document.querySelector("video")).toBeNull();
    // The movie's transport chrome yields to the trailer's.
    expect(
      screen.queryByRole("group", { name: "Playback controls" }),
    ).toBeNull();

    vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/watch-progress`, "PUT"),
    ).toHaveLength(0);

    fireEvent.click(screen.getByRole("button", { name: /^Start movie/ }));

    await waitFor(() => expect(document.querySelector("video")).not.toBeNull());
    expect(prerollRegion()).toBeNull();
    expect(
      screen.getByRole("group", { name: "Playback controls" }),
    ).toBeInTheDocument();
  });

  it("keeps the queue it started with when the window regains focus", async () => {
    const fetchMock = mockMovieApi({ preroll: twoTrailers });

    await renderMovieRoute(FRESH_SEARCH);

    const region = await screen.findByRole("region", {
      name: "Trailers before the movie",
    });
    act(() => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
      onlineManager.setOnline(false);
      onlineManager.setOnline(true);
    });
    focusManager.setFocused(undefined);

    // A refetch would draw a new random queue under the trailer playing.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/preroll`),
    ).toHaveLength(1);
    expect(region).toHaveTextContent("Glacier Run");
  });

  it("never asks for trailers when the play URL starts mid-movie", async () => {
    const fetchMock = mockMovieApi({ preroll: twoTrailers });

    await renderMovieRoute(
      "mode=direct&audio_track=0&subtitle_track=off&start=900",
    );

    await screen.findByRole("region", { name: "Video player for Signal Fire" });
    await waitFor(() => expect(document.querySelector("video")).not.toBeNull());
    expect(prerollRegion()).toBeNull();
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/preroll`),
    ).toHaveLength(0);
  });

  // Eligibility used to follow the live URL: seeking back to 0:00 rebases the
  // stream to start=0, which put a movie opened mid-way into the pre-roll and
  // unmounted the player in the middle of the film.
  it("never starts the trailers when a mid-movie page seeks back to the start", async () => {
    const fetchMock = mockMovieApi({ preroll: twoTrailers });
    hlsSupport.native = true;
    const { router } = await renderMovieRoute(
      "mode=remux&audio_track=0&subtitle_track=off&start=900",
    );
    const video = await movieVideo();
    Object.defineProperty(video, "readyState", {
      configurable: true,
      get: () => 4,
    });

    fireEvent.keyDown(document.body, { key: "Home" });
    await waitFor(() => {
      const search = router.state.location.search as Record<string, unknown>;
      expect(search.start).toBe(0);
    });

    expect(prerollRegion()).toBeNull();
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/preroll`),
    ).toHaveLength(0);
    expect(
      screen.getByRole("region", { name: "Video player for Signal Fire" }),
    ).toBeInTheDocument();
  });

  // A route opened with autoplay=true never runs the pre-roll, but the flag is
  // dropped from the URL once it has played, which made the movie eligible.
  it("never starts the trailers once a spent autoplay flag leaves the URL", async () => {
    const fetchMock = mockMovieApi({ preroll: twoTrailers });
    const { router } = await renderMovieRoute(
      "mode=direct&audio_track=0&subtitle_track=off&start=0&autoplay=true",
    );

    reportCanPlay(await movieVideo());
    await waitFor(() => {
      const search = router.state.location.search as Record<string, unknown>;
      expect(search.autoplay).toBeUndefined();
    });

    expect(prerollRegion()).toBeNull();
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/preroll`),
    ).toHaveLength(0);
    expect(document.querySelector("video")).not.toBeNull();
  });

  it("skips the trailers on Resume", async () => {
    mockMovieApi({
      preroll: twoTrailers,
      progress: { progress_sec: 900, duration_sec: 6000 },
    });

    const { router } = await renderMovieRoute(FRESH_SEARCH);

    const dialog = await screen.findByRole("dialog");
    expect(document.querySelector("video")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Resume" }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    await waitFor(() => expect(document.querySelector("video")).not.toBeNull());
    expect(prerollRegion()).toBeNull();
    const search = router.state.location.search as Record<string, unknown>;
    expect(search.start).toBe(900);
  });

  it("runs the trailers on Start from beginning", async () => {
    const fetchMock = mockMovieApi({
      preroll: twoTrailers,
      progress: { progress_sec: 900, duration_sec: 6000 },
    });

    await renderMovieRoute(FRESH_SEARCH);

    await screen.findByRole("dialog");
    expect(prerollRegion()).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "Start from beginning" }),
    );

    const region = await screen.findByRole("region", {
      name: "Trailers before the movie",
    });
    expect(region).toHaveTextContent("Trailer 1 of 2");
    expect(document.querySelector("video")).toBeNull();
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/watch-progress`, "DELETE"),
    ).toHaveLength(1);
  });
});

describe("movie play route resume offer", () => {
  beforeEach(() => {
    stubMediaElement();
    hlsSupport.native = false;
  });

  // A restored tab reopens the play URL, whose start is the last seek rather
  // than the last position. It used to start there without any offer.
  it("offers saved progress past a mid-media start and can keep the start", async () => {
    const fetchMock = mockMovieApi({
      progress: { progress_sec: 1500, duration_sec: 6000 },
    });
    const { router } = await renderMovieRoute(
      "mode=direct&audio_track=0&subtitle_track=off&start=900",
    );

    const dialog = await screen.findByRole("dialog", {
      name: "Resume playback?",
    });
    expect(
      within(dialog).getByText("Resume from 25:00 or play from 15:00."),
    ).toBeInTheDocument();

    fireEvent.click(
      within(dialog).getByRole("button", { name: "Play from 15:00" }),
    );

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const search = router.state.location.search as Record<string, unknown>;
    expect(search.start).toBe(900);
    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/watch-progress`, "DELETE"),
    ).toHaveLength(0);
  });

  // The page sits at its URL start while the offer stands. Saving that on a
  // tab switch or on leaving overwrote the progress the dialog was offering.
  it("saves no progress while the offer is undecided", async () => {
    const fetchMock = mockMovieApi({
      progress: { progress_sec: 1500, duration_sec: 6000 },
    });
    await renderMovieRoute(
      "mode=direct&audio_track=0&subtitle_track=off&start=900",
    );
    await screen.findByRole("dialog", { name: "Resume playback?" });
    const video = await movieVideo();
    Object.defineProperty(video, "currentTime", {
      configurable: true,
      get: () => 900,
    });

    act(() => {
      video.dispatchEvent(new Event("timeupdate"));
      window.dispatchEvent(new Event("pagehide"));
    });

    expect(
      requestsTo(fetchMock, `/api/movies/${MOVIE_ID}/watch-progress`, "PUT"),
    ).toHaveLength(0);
  });

  it("resumes from the saved progress", async () => {
    mockMovieApi({ progress: { progress_sec: 1500, duration_sec: 6000 } });
    const { router } = await renderMovieRoute(
      "mode=direct&audio_track=0&subtitle_track=off&start=900",
    );

    const dialog = await screen.findByRole("dialog", {
      name: "Resume playback?",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Resume" }));

    await waitFor(() => {
      const search = router.state.location.search as Record<string, unknown>;
      expect(search.start).toBe(1500);
    });
  });
});

describe("movie play route HLS session", () => {
  beforeEach(() => {
    stubMediaElement();
    // jsdom has no MediaSource; native HLS fetches the manifest itself first.
    hlsSupport.native = true;
  });

  function manifestSessionIds(fetchMock: ReturnType<typeof mockMovieApi>) {
    return fetchMock.mock.calls
      .map(([input]) => requestURL(input))
      .filter((url) => url.includes("/hls/remux/playlist.m3u8"))
      .map((url) => new URL(url, "http://igloo.test").searchParams.get("playback_session"));
  }

  // A reloaded, restored, or duplicated tab used to reuse the id from
  // sessionStorage, so the closing tab's late stop request could tear down
  // the session the new page had just created.
  it("gives every page its own playback session", async () => {
    const fetchMock = mockMovieApi();
    const search = "mode=remux&audio_track=0&subtitle_track=off&start=900";

    const first = await renderMovieRoute(search);
    await waitFor(() => expect(manifestSessionIds(fetchMock)).toHaveLength(1));
    first.unmount();

    await renderMovieRoute(search);
    await waitFor(() => expect(manifestSessionIds(fetchMock)).toHaveLength(2));

    const [firstId, secondId] = manifestSessionIds(fetchMock);
    expect(firstId).toBeTruthy();
    expect(secondId).toBeTruthy();
    expect(secondId).not.toBe(firstId);
  });
});

describe("movie play route pause during a source swap", () => {
  beforeEach(() => {
    stubMediaElement();
    // Native HLS: the element sits without a source while the replacement's
    // manifest is fetched, which is the window this is about.
    hlsSupport.native = true;
  });

  // Replacing a source pauses the element without a pause event, so the
  // controls kept showing Pause; pressing it played instead of pausing, and
  // the carried play started the new stream anyway.
  it("withdraws the play carried to the next source", async () => {
    const baseFetch = mockMovieApi();
    let releaseManifest: () => void = () => {};
    const heldManifest = new Promise<Response>((resolve) => {
      releaseManifest = () => resolve(new Response("#EXTM3U"));
    });
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = requestURL(input);
        const replacementManifest =
          url.includes("/hls/remux/playlist.m3u8") && !url.includes("start=890");
        if (replacementManifest) return heldManifest;
        return baseFetch(input, init);
      }),
    );

    await renderMovieRoute("mode=remux&audio_track=0&subtitle_track=off&start=900");
    const video = await movieVideo();
    await waitFor(() => expect(video.getAttribute("src")).toContain("start=890"));
    // jsdom never plays: a running play() is what turns `paused` false, and a
    // new load turns it back without firing `pause`.
    let paused = true;
    let readyState = 4;
    Object.defineProperty(video, "duration", {
      configurable: true,
      get: () => 5110,
    });
    act(() => {
      video.dispatchEvent(new Event("durationchange"));
    });
    Object.defineProperty(video, "paused", {
      configurable: true,
      get: () => paused,
    });
    Object.defineProperty(video, "readyState", {
      configurable: true,
      get: () => readyState,
    });
    playSpy().mockImplementation(async () => {
      paused = false;
      video.dispatchEvent(new Event("play"));
    });
    vi.mocked(window.HTMLMediaElement.prototype.load).mockImplementation(() => {
      paused = true;
      readyState = 0;
    });
    // The chrome re-renders across the swap, so each step looks it up again.
    const controls = () =>
      screen.getByRole("group", { name: "Playback controls" });
    await screen.findByRole("group", { name: "Playback controls" });

    await act(async () => {
      fireEvent.click(within(controls()).getByRole("button", { name: /^Play/ }));
    });
    expect(playSpy()).toHaveBeenCalledOnce();

    // A far seek rebases the session, which replaces the source.
    fireEvent.keyDown(screen.getByRole("slider", { name: /seek/i }), {
      key: "End",
    });
    await waitFor(() => expect(video.hasAttribute("src")).toBe(false));
    expect(
      within(controls()).getByRole("button", { name: /^Pause/ }),
    ).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(within(controls()).getByRole("button", { name: /^Pause/ }));
    });
    await waitFor(() =>
      expect(
        within(controls()).getByRole("button", { name: /^Play/ }),
      ).toBeInTheDocument(),
    );

    await act(async () => {
      releaseManifest();
    });
    await waitFor(() => expect(video.hasAttribute("src")).toBe(true));
    reportCanPlay(video);

    expect(playSpy()).toHaveBeenCalledOnce();
  });
});

describe("movie play route play button", () => {
  beforeEach(() => {
    stubMediaElement();
    hlsSupport.native = false;
  });

  async function pressPlay(rejection: DOMException) {
    mockMovieApi();
    playSpy().mockRejectedValue(rejection);
    await renderMovieRoute(
      "mode=direct&audio_track=0&subtitle_track=off&start=900",
    );
    await movieVideo();
    const controls = await screen.findByRole("group", {
      name: "Playback controls",
    });
    await act(async () => {
      fireEvent.click(within(controls).getByRole("button", { name: /^Play/ }));
    });
  }

  // A restored tab's first press used to land here: the fresh session's
  // start correction replaced the source under the pending play.
  it("does not report a play the player's own source swap cut short", async () => {
    await pressPlay(
      new DOMException(
        "The play() request was interrupted by a new load request.",
        "AbortError",
      ),
    );

    expect(playSpy()).toHaveBeenCalledOnce();
    expect(screen.queryByText("Playback failed")).toBeNull();
    expect(
      screen.getByRole("region", { name: "Video player for Signal Fire" }),
    ).toBeInTheDocument();
  });

  it("still reports a play the browser refused", async () => {
    await pressPlay(
      new DOMException("play() is not allowed here.", "NotAllowedError"),
    );

    expect(await screen.findByText("Playback failed")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Playback failed — the browser could not play this stream.",
      ),
    ).toBeInTheDocument();
  });

  // The browser rejects a pending play with NotSupportedError right after the
  // element's error event, which the direct-play fallback has just consumed.
  // Reporting the rejection raised the error screen over the remux stream the
  // fallback was switching to.
  it("lets the direct-play fallback take a pending play", async () => {
    mockMovieApi();
    hlsSupport.native = true;
    let rejectPlay: (reason: unknown) => void = () => {};
    playSpy().mockImplementation(
      () =>
        new Promise<void>((_resolve, reject) => {
          rejectPlay = reject;
        }),
    );
    const { router } = await renderMovieRoute(
      "mode=direct&audio_track=0&subtitle_track=off&start=900",
    );
    const video = await movieVideo();
    const controls = await screen.findByRole("group", {
      name: "Playback controls",
    });
    await act(async () => {
      fireEvent.click(within(controls).getByRole("button", { name: /^Play/ }));
    });

    vi.stubGlobal("MediaError", {
      MEDIA_ERR_ABORTED: 1,
      MEDIA_ERR_NETWORK: 2,
      MEDIA_ERR_DECODE: 3,
      MEDIA_ERR_SRC_NOT_SUPPORTED,
    });
    Object.defineProperty(video, "error", {
      configurable: true,
      value: { code: MEDIA_ERR_SRC_NOT_SUPPORTED },
    });
    await act(async () => {
      video.dispatchEvent(new Event("error"));
      rejectPlay(
        new DOMException("The element has no supported sources.", "NotSupportedError"),
      );
    });

    await waitFor(() => {
      const search = router.state.location.search as Record<string, unknown>;
      expect(search.mode).toBe("remux");
    });
    expect(screen.queryByText("Playback failed")).toBeNull();
    expect(
      screen.getByRole("region", { name: "Video player for Signal Fire" }),
    ).toBeInTheDocument();
  });
});
