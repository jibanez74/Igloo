import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DETAIL_PAGE_CONTENT_ENTER_CLASS } from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";
import type {
  AuthUser,
  LibraryMovieDetailsResponse,
  MovieTechnicalDetailsResponse,
  PlaybackSettingsType,
} from "@/types";

const DETAIL_PAGE_ANIMATION_MARKER =
  "animate-in fade-in slide-in-from-bottom-2";

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function requestURL(input: RequestInfo | URL) {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.toString();
  return input.url;
}

function nullableString(value = "") {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

function nullableInt64(value: number | null = null) {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function nullableFloat64(value: number | null = null) {
  return {
    Float64: value ?? 0,
    Valid: value != null,
  };
}

function authUser(): AuthUser {
  return {
    id: 1,
    name: "Movie User",
    email: "movies@example.com",
    is_admin: false,
    avatar: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function playbackSettings(): PlaybackSettingsType {
  return {
    profiles: [],
    preferred_profile: null,
    download_mbps: null,
    server_upload_mbps: null,
    is_admin: false,
    preferred_audio_language: null,
    preferred_subtitle_language: null,
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
        frame_rate: nullableString("24/1"),
        aspect_ratio: nullableString("16:9"),
        pixel_format: nullableString("yuv420p"),
        color_space: nullableString("bt709"),
        color_transfer: nullableString("bt709"),
        color_primaries: nullableString("bt709"),
        hdr_format: nullableString(""),
      },
    ],
    audio_streams: [
      {
        id,
        movie_id: id,
        stream_index: 1,
        codec: "aac",
        bit_rate: 192000,
        channels: 2,
        channel_layout: nullableString("stereo"),
        sample_rate: 48000,
        language: nullableString("en"),
        title: nullableString("English"),
        is_default: true,
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

function createMovieDetailsQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: {
        retry: false,
      },
      queries: {
        retry: false,
      },
    },
  });
}

async function renderMovieDetailsRoute(initialEntry: string) {
  vi.stubGlobal("scrollTo", vi.fn());
  mockMovieDetailsFetch();

  const queryClient = createMovieDetailsQueryClient();
  const history = createMemoryHistory({
    initialEntries: [initialEntry],
  });
  const router = createRouter({
    routeTree,
    context: {
      queryClient,
    },
    history,
  });

  await act(async () => {
    await router.load();
  });

  const view = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} context={{ queryClient }} />
    </QueryClientProvider>,
  );

  return {
    router,
    queryClient,
    ...view,
  };
}

function getDetailMotionWrappers(container: HTMLElement) {
  return Array.from(container.querySelectorAll("div")).filter((element) =>
    element.className.includes(DETAIL_PAGE_ANIMATION_MARKER),
  );
}

function getHeroMotionWrapper(container: HTMLElement) {
  return getDetailMotionWrappers(container).find((element) =>
    element.className.includes("delay-75"),
  );
}

function getLowerMotionWrapper(container: HTMLElement) {
  return getDetailMotionWrappers(container).find((element) =>
    element.className.includes("delay-150"),
  );
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("movie details route motion", () => {
  it("renders the library movie detail page with the three-stage stagger contract", async () => {
    const { container } = await renderMovieDetailsRoute("/movies/57/");

    expect(
      await screen.findByRole("heading", { name: /Arrival/i }),
    ).toBeInTheDocument();

    const wrappers = getDetailMotionWrappers(container);
    const heroWrapper = getHeroMotionWrapper(container);
    const lowerWrapper = getLowerMotionWrapper(container);
    const backdropWrapper = wrappers.find(
      (element) =>
        element !== heroWrapper &&
        element !== lowerWrapper &&
        element.className === DETAIL_PAGE_CONTENT_ENTER_CLASS,
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
      await screen.findByRole("heading", { name: /Arrival/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Arrival overview for motion verification."),
    ).toBeInTheDocument();

    const firstHeroWrapper = getHeroMotionWrapper(container);
    expect(firstHeroWrapper).toBeDefined();

    await act(async () => {
      await router.navigate({
        to: "/movies/$id/",
        params: { id: "58" },
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /Heat/i }),
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
