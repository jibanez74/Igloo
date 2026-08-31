import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DETAIL_PAGE_CONTENT_ENTER_CLASS } from "@/lib/constants";
import type { MusicianDetailsResponseType, MusicianTrackType } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { nullableFloat64, nullableInt64, nullableString } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";
import {
  getDetailMotionWrappers,
  getHeroMotionWrapper,
  getLowerMotionWrapper,
} from "../helpers/motion";

const { audioPlayerActionsMock, audioPlayerNowPlayingMock } = vi.hoisted(() => ({
  audioPlayerActionsMock: {
    playQueue: vi.fn(),
    playTrack: vi.fn(),
    shuffleQueue: vi.fn(),
    togglePlay: vi.fn(),
  },
  audioPlayerNowPlayingMock: {
    currentTrackId: null as number | null,
    isPlaying: false,
  },
}));

vi.mock("@/hooks/useAudioPlayerActions", () => ({
  useAudioPlayerActions: () => audioPlayerActionsMock,
}));

vi.mock("@/hooks/useAudioPlayerNowPlaying", () => ({
  useAudioPlayerNowPlaying: () => audioPlayerNowPlayingMock,
}));

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

async function renderMusicianDetailsRoute(initialEntry = "/music/musician/20") {
  mockMusicianDetailsFetch();

  return renderRoute(initialEntry);
}

afterEach(() => {
  audioPlayerNowPlayingMock.currentTrackId = null;
  audioPlayerNowPlayingMock.isPlaying = false;
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

  it("renders section skip links targeting the page headings", async () => {
    await renderMusicianDetailsRoute();

    expect(
      await screen.findByRole("heading", { name: "The Band" }),
    ).toBeInTheDocument();

    const skipNav = screen.getByRole("navigation", { name: "Skip to section" });
    const links = [
      { name: "Skip to musician info", href: "#musician-name" },
      { name: "Skip to discography", href: "#discography-heading" },
      { name: "Skip to all tracks", href: "#tracks-heading" },
    ];

    for (const { name, href } of links) {
      const link = screen.getByRole("link", { name });
      expect(skipNav).toContainElement(link);
      expect(link).toHaveAttribute("href", href);
      expect(document.getElementById(href.slice(1))).toHaveAttribute(
        "tabindex",
        "-1",
      );
    }
  });

  it("renders the discography with the shared album card and Badge-based chips", async () => {
    await renderMusicianDetailsRoute();

    expect(
      await screen.findByRole("heading", { name: "The Band" }),
    ).toBeInTheDocument();

    // Shared AlbumCard: full-label link plus the play overlay button.
    expect(
      screen.getByRole("link", { name: "Blue Record, 2026 · 2 tracks" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Play Blue Record, 2026 · 2 tracks" }),
    ).toBeInTheDocument();

    // Genre and stat chips render as Badge pills inside their labeled lists.
    const genreList = screen.getByRole("list", {
      name: "Genres: Alternative",
    });
    expect(
      genreList.querySelector('[data-slot="badge"]'),
    ).toHaveTextContent("Alternative");

    const statsList = screen.getByRole("list", {
      name: "Musician statistics",
    });
    expect(statsList.querySelectorAll('[data-slot="badge"]')).toHaveLength(3);
    expect(statsList.querySelector("time")).toHaveAttribute(
      "datetime",
      "PT360S",
    );
  });

  it("marks the currently playing track row with aria-current", async () => {
    audioPlayerNowPlayingMock.currentTrackId = 1;
    audioPlayerNowPlayingMock.isPlaying = true;

    const { container } = await renderMusicianDetailsRoute();

    expect(
      await screen.findByRole("heading", { name: "The Band" }),
    ).toBeInTheDocument();

    const currentRow = container.querySelector('[aria-current="true"]');
    expect(currentRow).not.toBeNull();
    expect(currentRow).toHaveTextContent("Alabaster");
  });

  // Row clicks go through playTrack, which toggles play/pause itself when the
  // clicked track is already current (covered in audio-player-queue.test.tsx).
  // playQueue is reserved for the header's start-over buttons.
  it("routes a row click through playTrack rather than restarting the queue", async () => {
    audioPlayerNowPlayingMock.currentTrackId = 1;
    audioPlayerNowPlayingMock.isPlaying = true;

    await renderMusicianDetailsRoute();

    fireEvent.click(
      await screen.findByRole("button", { name: "Pause Alabaster" }),
    );

    expect(audioPlayerActionsMock.playTrack).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1 }),
      expect.arrayContaining([expect.objectContaining({ id: 1 })]),
      expect.objectContaining({ title: "The Band" }),
    );
    expect(audioPlayerActionsMock.playQueue).not.toHaveBeenCalled();
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
