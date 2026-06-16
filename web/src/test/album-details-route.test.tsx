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
import type { AlbumDetailsResponseType, TrackType } from "@/types";

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

const toastMocks = vi.hoisted(() => ({
  showActionFailed: vi.fn(),
  showDeleted: vi.fn(),
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

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: toastMocks.showActionFailed,
  showDeleted: toastMocks.showDeleted,
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

function albumTrack(
  id: number,
  albumId: number,
  title: string,
  trackIndex: number,
): TrackType {
  return {
    id,
    title,
    sort_title: title,
    file_path: `/music/${albumId}/${title.toLowerCase().replaceAll(" ", "-")}.flac`,
    file_name: `${title.toLowerCase().replaceAll(" ", "-")}.flac`,
    container: "flac",
    mime_type: "audio/flac",
    codec: "flac",
    size: 1024,
    track_index: trackIndex,
    duration: 180,
    disc: 1,
    channels: "2",
    channel_layout: "stereo",
    bit_rate: 900000,
    profile: "",
    release_date: nullableString("2026-01-01"),
    year: nullableInt64(2026),
    composer: nullableString(""),
    copyright: nullableString(""),
    language: nullableString("en"),
    album_id: nullableInt64(albumId),
    musician_id: nullableInt64(albumId + 100),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function albumDetailsResponse(
  id: number,
  title: string,
  artistName: string,
  trackTitle: string,
): AlbumDetailsResponseType {
  return {
    album: {
      id,
      title,
      sort_title: title,
      musician: nullableString(artistName),
      spotify_id: nullableString(`spotify-${id}`),
      spotify_popularity: nullableFloat64(76),
      release_date: nullableString("2026-01-01"),
      year: nullableInt64(2026),
      total_tracks: nullableInt64(1),
      cover: nullableString(`/covers/${id}.jpg`),
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    tracks: [albumTrack(id * 10, id, trackTitle, 1)],
    artists: [
      {
        id: id + 100,
        name: artistName,
        thumb: nullableString(""),
        spotify_id: nullableString(`artist-${id}`),
      },
    ],
    track_genres: [
      {
        track_id: id * 10,
        genre_id: id,
        tag: "Alternative",
      },
    ],
    album_genres: ["Alternative"],
    total_duration: 180,
  };
}

type MockAlbumDetailsFetchOptions = {
  deleteAlbumResponse?: {
    body: unknown;
    status?: number;
  };
};

function mockAlbumDetailsFetch({
  deleteAlbumResponse,
}: MockAlbumDetailsFetchOptions = {}) {
  const detailsById = new Map<number, AlbumDetailsResponseType>([
    [42, albumDetailsResponse(42, "Blue Record", "The Band", "Alabaster")],
    [43, albumDetailsResponse(43, "Red Record", "The Trio", "Ember")],
  ]);

  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Album User",
            email: "albums@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    const detailsMatch = url.match(/^\/api\/music\/albums\/details\/(\d+)$/);
    if (detailsMatch) {
      const albumId = Number.parseInt(detailsMatch[1], 10);
      const payload = detailsById.get(albumId);
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
          liked_track_ids: [420],
        },
      });
    }

    if (url === "/api/music/albums/42" && method === "DELETE") {
      return jsonResponse(
        deleteAlbumResponse?.body ?? {
          error: false,
        },
        deleteAlbumResponse?.status ?? 200,
      );
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

function createAlbumDetailsQueryClient() {
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

async function renderAlbumDetailsRoute(
  initialEntry: string,
  options?: MockAlbumDetailsFetchOptions,
) {
  vi.stubGlobal("scrollTo", vi.fn());
  mockAlbumDetailsFetch(options);

  const queryClient = createAlbumDetailsQueryClient();
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
  vi.clearAllMocks();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("album details route motion", () => {
  it("renders the album detail page with the three-stage stagger contract", async () => {
    const { container } = await renderAlbumDetailsRoute("/music/album/42");

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
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

  it("replays the detail-page stagger when navigating between album ids on the same route", async () => {
    const { container, router } = await renderAlbumDetailsRoute(
      "/music/album/42",
    );

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("Alabaster")).toBeInTheDocument();

    const firstHeroWrapper = getHeroMotionWrapper(container);
    expect(firstHeroWrapper).toBeDefined();

    await act(async () => {
      await router.navigate({
        to: "/music/album/$id",
        params: { id: "43" },
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /Red Record/i }),
      ).toBeInTheDocument();
    });

    const secondHeroWrapper = getHeroMotionWrapper(container);

    expect(secondHeroWrapper).toBeDefined();
    expect(secondHeroWrapper).not.toBe(firstHeroWrapper);
    expect(screen.getByText("Ember")).toBeInTheDocument();
    expect(screen.queryByText("Alabaster")).not.toBeInTheDocument();
  });
});

describe("album details deletion", () => {
  it("keeps the delete dialog open and reenabled after an API error response", async () => {
    const user = userEvent.setup();
    const { router } = await renderAlbumDetailsRoute("/music/album/42", {
      deleteAlbumResponse: {
        body: {
          error: true,
          message: "Unable to delete album",
        },
      },
    });

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(screen.getByRole("menuitem", { name: "Delete Album" }));

    const dialog = await screen.findByRole("alertdialog", {
      name: "Delete Album",
    });
    const confirmButton = screen.getByRole("button", {
      name: "Delete Album",
    });
    const cancelButton = screen.getByRole("button", { name: "Cancel" });

    await user.click(confirmButton);

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "delete album",
        "Unable to delete album",
      );
    });

    expect(router.state.location.pathname).toBe("/music/album/42");
    expect(dialog).toBeInTheDocument();
    expect(confirmButton).toBeEnabled();
    expect(cancelButton).toBeEnabled();
  });
});
