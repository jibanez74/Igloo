import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import {
  AUTH_USER_KEY,
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  PLAYBACK_SETTINGS_KEY,
} from "@/lib/constants";
import type {
  ApiResponseType,
  AuthUser,
  LibraryMovieDetailsResponse,
  MovieTechnicalDetailsResponse,
  PlaybackSettingsType,
} from "@/types";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  resetDevicePlaybackPreferencesCache,
  setDevicePlaybackPreferences,
} from "@/lib/playback-preferences";
import { jsonResponse, requestURL } from "../helpers/api";
import { nullableFloat64, nullableInt64, nullableString } from "../helpers/fixtures";
import { createTestQueryClient } from "../helpers/render";
import { renderRoute } from "../helpers/render-route";
import {
  getDetailMotionWrappers,
  getHeroMotionWrapper,
  getLowerMotionWrapper,
} from "../helpers/motion";

beforeEach(() => {
  localStorage.clear();
  resetDevicePlaybackPreferencesCache();
});

// The playback settings dialog renders native selects on coarse pointers,
// which jsdom can drive with fireEvent.change (Radix selects need real
// pointer events). Only the dialog consumes this hook.
const prefersCoarse = vi.hoisted(() => ({ value: false }));
vi.mock("@/hooks/use-coarse-pointer", () => ({
  usePrefersCoarsePointer: () => prefersCoarse.value,
}));

function success<T extends Record<string, unknown>>(data: T): ApiResponseType<T> {
  return {
    error: false,
    data,
  };
}

function authUser(): AuthUser {
  return {
    id: 1,
    name: "Movie User",
    email: "movies@example.com",
    is_admin: false,
    avatar: null,
    has_pin: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function playbackSettings(): PlaybackSettingsType {
  return {
    profiles: [],
    server_upload_mbps: null,
    hardware_acceleration_device: "cpu",
  };
}

function movieDetailsResponse(
  id: number,
  title: string,
  overview: string,
  releaseDate: string,
  language: string,
): LibraryMovieDetailsResponse {
  return {
    movie: {
      id,
      title,
      file_path: `/movies/${title.toLowerCase().replaceAll(" ", "-")}.mkv`,
      file_name: `${title.toLowerCase().replaceAll(" ", "-")}.mkv`,
      size: 1024,
      container: "mkv",
      mime_type: "video/x-matroska",
      adult: false,
      tmdb_id: nullableInt64(1000 + id),
      imdb_id: nullableString(`tt${1000 + id}`),
      poster_path: nullableString(`/poster-${id}.jpg`),
      backdrop_path: nullableString(`/backdrop-${id}.jpg`),
      language: nullableString(language),
      year: nullableInt64(Number.parseInt(releaseDate.slice(0, 4), 10)),
      release_date: nullableString(releaseDate),
      overview: nullableString(overview),
      tag_line: nullableString(""),
      certification: nullableString("PG-13"),
      critic_rating: nullableFloat64(92),
      audience_rating: nullableFloat64(86),
      revenue: nullableFloat64(1000000),
      budget: nullableFloat64(500000),
      run_time: nullableInt64(116),
      duration: nullableFloat64(6960),
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    cast: [
      {
        id: id * 10,
        movie_id: id,
        artist_id: id * 10,
        character: "Lead",
        cast_order: 0,
        artist_name: `${title} Lead`,
        artist_profile: nullableString(""),
      },
    ],
    crew: [
      {
        id: id * 20,
        movie_id: id,
        artist_id: id * 20,
        job: "Director",
        department: "Directing",
        artist_name: `${title} Director`,
        artist_profile: nullableString(""),
      },
    ],
    genres: [
      {
        id,
        tag: "Drama",
      },
    ],
    production_companies: [
      {
        id,
        name: `${title} Pictures`,
        tmdb_id: id,
        logo: nullableString(""),
        country: nullableString("US"),
      },
    ],
    extra_videos: [],
  };
}

function technicalDetailsResponse(id: number): MovieTechnicalDetailsResponse {
  return {
    movie: {
      file_name: `movie-${id}.mkv`,
      file_path: `/movies/movie-${id}.mkv`,
      size: 1024,
      container: "mkv",
      mime_type: "video/x-matroska",
      run_time: nullableInt64(116),
      duration: nullableFloat64(6960),
    },
    video_streams: [
      {
        id,
        movie_id: id,
        stream_index: 0,
        codec: "h264",
        codec_profile: nullableString("High"),
        codec_level: nullableInt64(41),
        bit_rate: 4000000,
        width: 1920,
        height: 1080,
        coded_width: nullableInt64(1920),
        coded_height: nullableInt64(1080),
        aspect_ratio: nullableString("16:9"),
        frame_rate: 24,
        avg_frame_rate: nullableString("24/1"),
        bit_depth: nullableInt64(8),
        pixel_format: nullableString("yuv420p"),
        color_range: nullableString("tv"),
        color_space: nullableString("bt709"),
        color_primaries: nullableString("bt709"),
        color_transfer: nullableString("bt709"),
        field_order: nullableString("progressive"),
        rotation: nullableInt64(),
        language: nullableString("en"),
        title: nullableString("Main"),
      },
    ],
    audio_streams: [
      {
        id,
        movie_id: id,
        stream_index: 1,
        codec: "aac",
        codec_profile: nullableString("LC"),
        bit_rate: 192000,
        sample_rate: nullableInt64(48000),
        channels: 2,
        channel_layout: nullableString("stereo"),
        language: nullableString("en"),
        title: nullableString("English"),
        is_default: false,
      },
    ],
    subtitles: [],
    chapters: [],
  };
}

function mockMovieDetailsFetch() {
  const detailsById = new Map<number, LibraryMovieDetailsResponse>([
    [
      57,
      movieDetailsResponse(
        57,
        "Arrival",
        "Arrival overview for motion verification.",
        "2016-11-11",
        "en",
      ),
    ],
    [
      58,
      movieDetailsResponse(
        58,
        "Heat",
        "Heat overview after navigating to a different movie.",
        "1995-12-15",
        "fr",
      ),
    ],
  ]);
  const technicalById = new Map<number, MovieTechnicalDetailsResponse>([
    [57, technicalDetailsResponse(57)],
    [58, technicalDetailsResponse(58)],
  ]);

  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: authUser(),
        },
      });
    }

    if (url === "/api/settings/playback") {
      return jsonResponse({
        error: false,
        data: {
          settings: playbackSettings(),
        },
      });
    }

    const detailsMatch = url.match(/^\/api\/movies\/details\/(\d+)$/);
    if (detailsMatch) {
      const movieId = Number.parseInt(detailsMatch[1], 10);
      const payload = detailsById.get(movieId);
      if (payload) {
        return jsonResponse({
          error: false,
          data: payload,
        });
      }
    }

    const technicalMatch = url.match(
      /^\/api\/movies\/(\d+)\/technical-details$/,
    );
    if (technicalMatch) {
      const movieId = Number.parseInt(technicalMatch[1], 10);
      const payload = technicalById.get(movieId);
      if (payload) {
        return jsonResponse({
          error: false,
          data: payload,
        });
      }
    }

    const likeStatusMatch = url.match(/^\/api\/movies\/(\d+)\/like-status$/);
    if (likeStatusMatch) {
      return jsonResponse({
        error: false,
        data: {
          is_liked: false,
        },
      });
    }

    const watchProgressMatch = url.match(
      /^\/api\/movies\/(\d+)\/watch-progress$/,
    );
    if (watchProgressMatch) {
      return jsonResponse({
        error: false,
        data: {
          progress_sec: null,
          duration_sec: null,
          watched: false,
          updated_at: null,
        },
      });
    }

    return jsonResponse(
      {
        error: true,
        message: `Unexpected request: ${url}`,
      },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

async function renderMovieDetailsRoute(initialEntry: string) {
  mockMovieDetailsFetch();

  return renderRoute(initialEntry);
}

async function renderMovieDetailsRouteWithQueryClient(
  initialEntry: string,
  queryClient: QueryClient,
) {
  return renderRoute(initialEntry, { queryClient });
}

function getPlayLink() {
  return screen.getByRole("link", { name: /^Play$/i });
}

function getPlayLinkMode() {
  const href = getPlayLink().getAttribute("href");
  expect(href).not.toBeNull();

  return new URL(href ?? "", "http://localhost").searchParams.get("mode");
}

describe("movie details route motion", () => {
  it("renders the library movie detail page with the three-stage stagger contract", async () => {
    const { container } = await renderMovieDetailsRoute("/movies/57/");

    expect(
      await screen.findByRole("heading", { name: /Arrival/i, level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByText("1 hr 56 min")).toBeInTheDocument();

    const runtime = screen
      .getByText("Runtime: 1 hour 56 minutes")
      .closest("time");
    expect(runtime).toHaveAttribute("datetime", "PT116M");

    const wrappers = getDetailMotionWrappers(container);
    const heroWrapper = getHeroMotionWrapper(container);
    const lowerWrapper = getLowerMotionWrapper(container);
    const backdropWrapper = wrappers.find(
      (element) =>
        element !== heroWrapper &&
        element !== lowerWrapper &&
        element.className.startsWith(DETAIL_PAGE_CONTENT_ENTER_CLASS) &&
        !element.className.includes("delay-"),
    );

    expect(wrappers).toHaveLength(3);
    expect(backdropWrapper).toBeDefined();
    expect(heroWrapper?.className).toContain("delay-75 motion-reduce:delay-0");
    expect(lowerWrapper?.className).toContain(
      "delay-150 motion-reduce:delay-0",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:opacity-100",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:translate-y-0",
    );
  });

  it("replays the detail-page stagger when navigating between movie ids on the same route", async () => {
    const { container, router } = await renderMovieDetailsRoute("/movies/57/");

    expect(
      await screen.findByRole("heading", { name: /Arrival/i, level: 1 }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Arrival overview for motion verification."),
    ).toBeInTheDocument();

    const firstHeroWrapper = getHeroMotionWrapper(container);
    expect(firstHeroWrapper).toBeDefined();

    await act(async () => {
      await router.navigate({
        to: "/movies/$id",
        params: { id: "58" },
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /Heat/i, level: 1 }),
      ).toBeInTheDocument();
    });

    const secondHeroWrapper = getHeroMotionWrapper(container);

    expect(secondHeroWrapper).toBeDefined();
    expect(secondHeroWrapper).not.toBe(firstHeroWrapper);
    expect(
      screen.getByText("Heat overview after navigating to a different movie."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Arrival overview for motion verification."),
    ).not.toBeInTheDocument();
  });
});

describe("movie details route playback settings sync", () => {
  it("uses the seeded smart default mode on the initial render", async () => {
    mockMovieDetailsFetch();

    const queryClient = createTestQueryClient();
    queryClient.setQueryData([AUTH_USER_KEY], success({ user: authUser() }));
    queryClient.setQueryData(
      [PLAYBACK_SETTINGS_KEY],
      success({ settings: playbackSettings() }),
    );
    setDevicePlaybackPreferences(1, { preferredProfile: "1080p_8mbps" });

    await renderMovieDetailsRouteWithQueryClient("/movies/57/", queryClient);

    expect(
      await screen.findByRole("heading", { name: /Arrival/i, level: 1 }),
    ).toBeInTheDocument();
    expect(getPlayLinkMode()).toBe("1080p_8mbps");
  });

  it("updates the play link when the device preference changes", async () => {
    await renderMovieDetailsRoute("/movies/57/");

    expect(
      await screen.findByRole("heading", { name: /Arrival/i, level: 1 }),
    ).toBeInTheDocument();
    expect(getPlayLinkMode()).toBe("remux");

    await act(async () => {
      setDevicePlaybackPreferences(1, { preferredProfile: "1080p_8mbps" });
    });

    await waitFor(() => {
      expect(getPlayLinkMode()).toBe("1080p_8mbps");
    });
  });

  it("keeps the user's saved dialog selection when the device preference changes", async () => {
    prefersCoarse.value = true;
    try {
      await renderMovieDetailsRoute("/movies/57/");

      expect(
        await screen.findByRole("heading", { name: /Arrival/i, level: 1 }),
      ).toBeInTheDocument();
      expect(getPlayLinkMode()).toBe("remux");

      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: "More options" }));
      await user.click(
        await screen.findByRole("menuitem", { name: "Playback Settings" }),
      );

      const modeSelect = await screen.findByLabelText("Playback");
      fireEvent.change(modeSelect, { target: { value: "720p_3mbps" } });
      await user.click(screen.getByRole("button", { name: "Done" }));

      await waitFor(() => {
        expect(getPlayLinkMode()).toBe("720p_3mbps");
      });

      await act(async () => {
        setDevicePlaybackPreferences(1, { preferredProfile: "1080p_8mbps" });
      });

      expect(getPlayLinkMode()).toBe("720p_3mbps");
    } finally {
      prefersCoarse.value = false;
    }
  });
});
