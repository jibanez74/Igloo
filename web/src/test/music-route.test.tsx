import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CONTENT_FADE_TRANSITION_MS } from "@/lib/constants";
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

  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} context={{ queryClient }} />
    </QueryClientProvider>,
  );
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
