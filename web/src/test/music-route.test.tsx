import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CONTENT_FADE_TRANSITION_MS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
} from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";

const { audioPlayerActionsMock } = vi.hoisted(() => ({
  audioPlayerActionsMock: {
    playAlbum: vi.fn(),
    playTrack: vi.fn(),
    startPlayAllPlayback: vi.fn(),
    startShufflePlayback: vi.fn(),
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

const defaultMatchMedia = window.matchMedia;

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

function track(id: number, title: string) {
  return {
    id,
    title,
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    file_path: `/music/${title}.flac`,
    album_id: nullableInt64(10),
    album_title: nullableString("Blue Record"),
    album_cover: nullableString(),
    musician_id: nullableInt64(20),
    musician_name: nullableString("The Band"),
  };
}

function mockMusicFetch() {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Music User",
            email: "music@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    if (url === "/api/music/stats") {
      return jsonResponse({
        error: false,
        data: {
          total_albums: 1,
          total_tracks: 5,
          total_musicians: 1,
        },
      });
    }

    if (url === "/api/music/albums?page=1&per_page=24") {
      return jsonResponse({
        error: false,
        data: {
          albums: [
            {
              id: 1,
              title: "Blue Record",
              cover: nullableString(),
              musician: nullableString("The Band"),
              year: nullableInt64(2026),
            },
          ],
          total: 1,
          page: 1,
          per_page: 24,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/music/musicians?page=1&per_page=24") {
      return jsonResponse({
        error: false,
        data: {
          musicians: [
            {
              id: 2,
              name: "Nina Simone",
              sort_name: "Simone, Nina",
              thumb: nullableString(),
              album_count: 1,
              track_count: 5,
            },
          ],
          total: 1,
          page: 1,
          per_page: 24,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/music/tracks?limit=50&offset=0") {
      return jsonResponse({
        error: false,
        data: {
          tracks: [track(1, "Alabaster"), track(2, "Borrowed Light")],
          total: 2,
          offset: 0,
          limit: 50,
          has_more: false,
        },
      });
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

function createMusicQueryClient() {
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

async function renderMusicRoute(initialEntry: string) {
  vi.stubGlobal("scrollTo", vi.fn());
  mockMusicFetch();

  const queryClient = createMusicQueryClient();
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
    queryClient,
    router,
    ...view,
  };
}

function setReducedMotionPreference(prefersReducedMotion: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches:
        query === "(prefers-reduced-motion: reduce)" && prefersReducedMotion,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: defaultMatchMedia,
  });
});

describe("music route tab transitions", () => {
  it("delays swapping from albums to musicians until the fade-out completes", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMusicRoute("/music/");

    expect(screen.getByText("Blue Record")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Musicians" }));

    expect(screen.getByText("Blue Record")).toBeInTheDocument();
    expect(screen.queryByText("Nina Simone")).not.toBeInTheDocument();

    const transitionCallIndex = setTimeoutSpy.mock.calls.findIndex(
      ([, delay]) => delay === CONTENT_FADE_TRANSITION_MS,
    );

    expect(transitionCallIndex).toBeGreaterThanOrEqual(0);
    expect(screen.getByText("Blue Record")).toBeInTheDocument();
    expect(screen.queryByText("Nina Simone")).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Nina Simone")).toBeInTheDocument();
    });
  });

  it("switches tabs without waiting when reduced motion is enabled", async () => {
    setReducedMotionPreference(true);
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMusicRoute("/music/");

    await user.click(screen.getByRole("tab", { name: "Musicians" }));

    await waitFor(() => {
      expect(screen.getByText("Nina Simone")).toBeInTheDocument();
    });
    expect(
      setTimeoutSpy.mock.calls.some(
        ([, delay]) => delay === CONTENT_FADE_TRANSITION_MS,
      ),
    ).toBe(false);
  });
});

describe("music route section motion", () => {
  it("applies section entrance contracts without changing tab panel fade behavior", async () => {
    await renderMusicRoute("/music/");

    const heading = await screen.findByRole("heading", {
      name: "Music Library",
    });
    const stats = screen.getByRole("region", {
      name: "Library statistics: 1 albums, 5 tracks, 1 musicians",
    });
    const tabsRoot = screen.getByRole("tablist").closest('[data-slot="tabs"]');

    expect(heading.closest("header")?.className).toContain(
      MOTION_SECTION_ENTER_CLASS,
    );
    expect(stats.className).toContain(MOTION_SECTION_ENTER_DELAYED_CLASS);
    expect(tabsRoot?.className).toContain(MOTION_SECTION_ENTER_DELAYED_CLASS);
    expect(
      screen.getByRole("tabpanel", { name: "Albums" }).firstElementChild
        ?.className,
    ).toContain(MOTION_SECTION_ENTER_CLASS);
  });
});

describe("music route tracks tab", () => {
  it("labels the virtualized track list and track action menus", async () => {
    await renderMusicRoute("/music/?tab=tracks");

    const tracksList = await screen.findByRole("list", { name: "Tracks" });

    expect(tracksList).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Tracks starting with A" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Tracks starting with B" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "More actions for Alabaster" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "More actions for Borrowed Light" }),
    ).toBeInTheDocument();

    const trackRows = within(tracksList).getAllByRole("listitem");

    expect(trackRows).toHaveLength(2);
    expect(trackRows[0]).toHaveAttribute("aria-posinset", "1");
    expect(trackRows[0]).toHaveAttribute("aria-setsize", "2");
    expect(trackRows[1]).toHaveAttribute("aria-posinset", "2");
    expect(trackRows[1]).toHaveAttribute("aria-setsize", "2");
  });
});
