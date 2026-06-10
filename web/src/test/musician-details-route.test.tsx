import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { routeTree } from "@/routeTree.gen";
import type { MusicianDetailsResponseType, MusicianTrackType } from "@/types";

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

function musicianTrack(id: number, title: string): MusicianTrackType {
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
    album_id: nullableInt64(7),
    album_title: nullableString("Blue Record"),
    album_cover: nullableString(""),
  };
}

function musicianDetailsResponse(): MusicianDetailsResponseType {
  return {
    musician: {
      id: 20,
      name: "The Band",
      sort_name: "Band, The",
      summary: nullableString("A focused test artist with two tracks."),
      spotify_popularity: nullableFloat64(74),
      spotify_followers: nullableInt64(12000),
      spotify_id: nullableString("spotify-20"),
      thumb: nullableString(""),
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    albums: [
      {
        id: 7,
        title: "Blue Record",
        cover: nullableString(""),
        year: nullableInt64(2026),
        release_date: nullableString("2026-01-01"),
        spotify_popularity: nullableFloat64(70),
        track_count: 2,
      },
    ],
    tracks: [
      musicianTrack(1, "Alabaster"),
      musicianTrack(2, "Borrowed Light"),
    ],
    genres: ["Alternative"],
    total_duration: 360000,
  };
}

function mockMusicianDetailsFetch() {
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

    if (url === "/api/music/musicians/20") {
      return jsonResponse({
        error: false,
        data: musicianDetailsResponse(),
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

async function renderMusicianDetailsRoute() {
  vi.stubGlobal("scrollTo", vi.fn());
  mockMusicianDetailsFetch();

  const queryClient = createMusicianDetailsQueryClient();
  const history = createMemoryHistory({
    initialEntries: ["/music/musician/20"],
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

    for (let index = 0; index < 20 && document.activeElement !== playAll; index += 1) {
      await user.tab();
      expect(document.activeElement).not.toBe(summary);
    }

    expect(playAll).toHaveFocus();
  });
});
