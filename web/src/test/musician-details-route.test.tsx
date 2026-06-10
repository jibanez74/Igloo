import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DETAIL_PAGE_CONTENT_ENTER_CLASS } from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";
import type { MusicianDetailsResponseType, MusicianTrackType } from "@/types";

const DETAIL_PAGE_ANIMATION_MARKER =
  "animate-in fade-in slide-in-from-bottom-2";

const { audioPlayerActionsMock } = vi.hoisted(() => ({
  audioPlayerActionsMock: {
    playAlbum: vi.fn(),
    playTrack: vi.fn(),
    shuffleAlbum: vi.fn(),
    togglePlay: vi.fn(),
  },
}));

vi.mock("@/hooks/useAudioPlayerActions", () => ({
  useAudioPlayerActions: () => audioPlayerActionsMock,
}));

vi.mock("@/hooks/useAudioPlayerState", () => ({
  useAudioPlayerState: () => ({
    currentTrack: null,
    isPlaying: false,
  }),
}));

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

function musicianTrack(
  id: number,
  title: string,
  albumId = 7,
  albumTitle = "Blue Record",
): MusicianTrackType {
  return {
    id,
    title,
    sort_title: title,
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    file_path: `/music/${title.toLowerCase().replaceAll(" ", "-")}.flac`,
    track_index: id,
    disc: 1,
    album_id: nullableInt64(albumId),
    album_title: nullableString(albumTitle),
    album_cover: nullableString(""),
  };
}

type MusicianDetailsFixtureOptions = {
  id?: number;
  name?: string;
  sortName?: string;
  summary?: string;
  albumId?: number;
  albumTitle?: string;
  trackIds?: number[];
  trackTitles?: string[];
  genres?: string[];
};

function musicianDetailsResponse({
  id = 20,
  name = "The Band",
  sortName = "Band, The",
  summary = "A focused test artist with two tracks.",
  albumId = 7,
  albumTitle = "Blue Record",
  trackIds = [1, 2],
  trackTitles = ["Alabaster", "Borrowed Light"],
  genres = ["Alternative"],
}: MusicianDetailsFixtureOptions = {}): MusicianDetailsResponseType {
  const tracks = trackTitles.map((title, index) =>
    musicianTrack(
      trackIds[index] ?? id * 100 + index,
      title,
      albumId,
      albumTitle,
    ),
  );

  return {
    musician: {
      id,
      name,
      sort_name: sortName,
      summary: nullableString(summary),
      spotify_popularity: nullableFloat64(74),
      spotify_followers: nullableInt64(12000),
      spotify_id: nullableString(`spotify-${id}`),
      thumb: nullableString(""),
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    albums: [
      {
        id: albumId,
        title: albumTitle,
        cover: nullableString(""),
        year: nullableInt64(2026),
        release_date: nullableString("2026-01-01"),
        spotify_popularity: nullableFloat64(70),
        track_count: tracks.length,
      },
    ],
    tracks,
    genres,
    total_duration: tracks.reduce(
      (total, track) => total + track.duration * 1000,
      0,
    ),
  };
}

function mockMusicianDetailsFetch() {
  const detailsById = new Map<number, MusicianDetailsResponseType>([
    [20, musicianDetailsResponse()],
    [
      21,
      musicianDetailsResponse({
        id: 21,
        name: "The Soloist",
        sortName: "Soloist, The",
        summary: "A second test artist for route motion.",
        albumId: 8,
        albumTitle: "Silver Record",
        trackIds: [3],
        trackTitles: ["Silver Path"],
        genres: ["Ambient"],
      }),
    ],
  ]);

  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Musician User",
            email: "musicians@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    const detailsMatch = url.match(/^\/api\/music\/musicians\/(\d+)$/);
    if (detailsMatch) {
      const musicianId = Number.parseInt(detailsMatch[1], 10);
      const payload = detailsById.get(musicianId);
      if (payload) {
        return jsonResponse({
          error: false,
          data: payload,
        });
      }
    }

    if (url === "/api/music/tracks/liked-ids") {
      return jsonResponse({
        error: false,
        data: {
          liked_track_ids: [2],
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
}

function createMusicianDetailsQueryClient() {
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

async function renderMusicianDetailsRoute(initialEntry = "/music/musician/20") {
  vi.stubGlobal("scrollTo", vi.fn());
  mockMusicianDetailsFetch();

  const queryClient = createMusicianDetailsQueryClient();
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

describe("musician details route accessibility", () => {
  it("describes the artist article with a non-focusable summary and keeps Play All in the tab order", async () => {
    const user = userEvent.setup();
    const { container } = await renderMusicianDetailsRoute();

    expect(
      await screen.findByRole("heading", { name: "The Band" }),
    ).toBeInTheDocument();

    const article = container.querySelector("article");
    expect(article).toHaveAttribute("aria-describedby", "musician-20-summary");

    const summary = document.getElementById("musician-20-summary");
    expect(summary).toHaveTextContent(
      "The Band. 1 album, 2 tracks. Total duration: 6m 0s. Genres: Alternative.",
    );
    expect(summary).not.toHaveAttribute("tabindex");

    const playAll = screen.getByRole("button", {
      name: "Play all 2 tracks by The Band",
    });

    for (
      let index = 0;
      index < 20 && document.activeElement !== playAll;
      index += 1
    ) {
      await user.tab();
      expect(document.activeElement).not.toBe(summary);
    }

    expect(playAll).toHaveFocus();
  });
});

describe("musician details route motion", () => {
  it("renders the musician detail page with the three-stage stagger contract", async () => {
    const { container } = await renderMusicianDetailsRoute();

    expect(
      await screen.findByRole("heading", { name: "The Band" }),
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

  it("replays the detail-page stagger when navigating between musician ids on the same route", async () => {
    const { container, router } = await renderMusicianDetailsRoute();

    expect(
      await screen.findByRole("heading", { name: "The Band" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Alabaster")).toBeInTheDocument();

    const firstHeroWrapper = getHeroMotionWrapper(container);
    expect(firstHeroWrapper).toBeDefined();

    await act(async () => {
      await router.navigate({
        to: "/music/musician/$id",
        params: { id: "21" },
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "The Soloist" }),
      ).toBeInTheDocument();
    });

    const secondHeroWrapper = getHeroMotionWrapper(container);

    expect(secondHeroWrapper).toBeDefined();
    expect(secondHeroWrapper).not.toBe(firstHeroWrapper);
    expect(screen.getByText("Silver Path")).toBeInTheDocument();
    expect(screen.queryByText("Alabaster")).not.toBeInTheDocument();
  });
});
