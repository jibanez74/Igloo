import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import userEvent from "@testing-library/user-event";
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

const defaultMatchMedia = window.matchMedia;

function createSearchQueryClient() {
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

function mockSearchFetch() {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.toString()
          : input.url;

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Search User",
            email: "search@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    if (url === "/api/search?q=Casino") {
      return jsonResponse({
        error: false,
        data: {
          query: "Casino",
          movies: {
            results: [
              {
                id: 7,
                title: "Casino Royale",
                poster_path: { String: "", Valid: false },
                year: { Int64: 2006, Valid: true },
                certification: { String: "PG-13", Valid: true },
              },
            ],
            total: 1,
          },
          albums: {
            results: [],
            total: 0,
          },
          musicians: {
            results: [],
            total: 0,
          },
          tracks: {
            results: [],
            total: 0,
          },
        },
      });
    }

    if (url.startsWith("/api/search/movies?")) {
      return jsonResponse({
        error: false,
        data: {
          query: "Casino",
          results: [
            {
              id: 7,
              title: "Casino Royale",
              poster_path: { String: "", Valid: false },
              year: { Int64: 2006, Valid: true },
              certification: { String: "PG-13", Valid: true },
            },
          ],
          total: 50,
          page: 3,
          per_page: 24,
          total_pages: 3,
        },
      });
    }

    if (url === "/api/search/albums?q=Casino&page=1&per_page=24") {
      return jsonResponse({
        error: false,
        data: {
          query: "Casino",
          results: [
            {
              id: 12,
              title: "Casino Original Soundtrack",
              cover: { String: "", Valid: false },
              musician: { String: "Various Artists", Valid: true },
              year: { Int64: 1995, Valid: true },
            },
          ],
          total: 1,
          page: 1,
          per_page: 24,
          total_pages: 1,
        },
      });
    }

    return jsonResponse({
      error: true,
      message: `Unexpected request: ${url}`,
    });
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

async function renderSearchRoute(initialEntry: string) {
  const fetchMock = mockSearchFetch();
  const queryClient = createSearchQueryClient();
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

  return { fetchMock, router, ...view };
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

describe("search route", () => {
  it("applies the base section entrance contract to the empty-search header", async () => {
    await renderSearchRoute("/search/");

    const heading = await screen.findByRole("heading", { name: "Search" });

    expect(heading.closest("header")?.className).toContain(
      MOTION_SECTION_ENTER_CLASS,
    );
  });

  it("applies section entrance contracts to the results header and stable tabs root", async () => {
    await renderSearchRoute("/search/?q=Casino&tab=all");

    const heading = await screen.findByRole("heading", {
      name: /Search results for/i,
    });
    const tabsRoot = screen.getByRole("tablist").closest('[data-slot="tabs"]');

    expect(heading.closest("header")?.className).toContain(
      MOTION_SECTION_ENTER_CLASS,
    );
    expect(tabsRoot?.className).toContain(MOTION_SECTION_ENTER_DELAYED_CLASS);
    expect(
      screen.getByRole("tabpanel", { name: "All" }).firstElementChild
        ?.className,
    ).toContain(MOTION_SECTION_ENTER_CLASS);
  });

  it("shows the server-clamped page for overlarge requested pages", async () => {
    window.scrollTo = vi.fn();
    const { fetchMock, router } = await renderSearchRoute(
      "/search/?q=Casino&tab=movies&page=999",
    );

    expect(await screen.findByText("Casino Royale")).toBeInTheDocument();
    expect(screen.getByText("50 movies")).toBeInTheDocument();
    expect(screen.getByText("Page 3 of 3")).toBeInTheDocument();

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/search/movies?q=Casino&page=999&per_page=24",
        expect.objectContaining({
          credentials: "include",
          method: "GET",
        }),
      );
    });

    await waitFor(() => {
      expect(router.state.location.search).toMatchObject({
        q: "Casino",
        tab: "movies",
        page: 3,
      });
    });
  });

  it("delays swapping from all results to albums until the fade-out completes", async () => {
    window.scrollTo = vi.fn();
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderSearchRoute("/search/?q=Casino&tab=all");

    expect(screen.getByText("Casino Royale")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Albums" }));

    expect(screen.getByText("Casino Royale")).toBeInTheDocument();
    expect(
      screen.queryByText("Casino Original Soundtrack"),
    ).not.toBeInTheDocument();

    const transitionCallIndex = setTimeoutSpy.mock.calls.findIndex(
      ([, delay]) => delay === CONTENT_FADE_TRANSITION_MS,
    );

    expect(transitionCallIndex).toBeGreaterThanOrEqual(0);
    expect(screen.getByText("Casino Royale")).toBeInTheDocument();
    expect(
      screen.queryByText("Casino Original Soundtrack"),
    ).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Casino Original Soundtrack")).toBeInTheDocument();
    });
  });

  it("switches tabs without waiting when reduced motion is enabled", async () => {
    setReducedMotionPreference(true);
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderSearchRoute("/search/?q=Casino&tab=all");

    await user.click(screen.getByRole("tab", { name: "Albums" }));

    await waitFor(() => {
      expect(screen.getByText("Casino Original Soundtrack")).toBeInTheDocument();
    });
    expect(
      setTimeoutSpy.mock.calls.some(
        ([, delay]) => delay === CONTENT_FADE_TRANSITION_MS,
      ),
    ).toBe(false);
  });
});
