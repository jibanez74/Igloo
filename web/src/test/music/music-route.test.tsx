import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ALBUMS_PER_PAGE,
  CONTENT_FADE_TRANSITION_MS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MUSICIANS_PER_PAGE,
  TRACKS_INFINITE_PAGE_SIZE,
} from "@/lib/constants";
import { jsonResponse, requestURL } from "../helpers/api";
import { runContentFadeTransitionTimeout } from "../helpers/content-fade-transition";
import { restoreMatchMedia, setReducedMotionPreference } from "../helpers/dom";
import { authUser, nullableInt64, nullableString } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

const { audioPlayerActionsMock } = vi.hoisted(() => ({
  audioPlayerActionsMock: {
    playQueue: vi.fn(),
    playTrack: vi.fn(),
    startPlayAllPlayback: vi.fn(),
    startShufflePlayback: vi.fn(),
  },
}));

vi.mock("@/hooks/useAudioPlayerActions", () => ({
  useAudioPlayerActions: () => audioPlayerActionsMock,
}));

vi.mock("@/hooks/useAudioPlayerNowPlaying", () => ({
  useAudioPlayerNowPlaying: () => ({
    currentTrackId: null,
    isPlaying: false,
  }),
}));

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

type MockMusicFetchOptions = {
  spotifyAvailable?: boolean;
  emptyMusicians?: boolean;
  failFirstTracksRequest?: boolean;
};

function mockMusicFetch(options: MockMusicFetchOptions = {}) {
  const spotifyAvailable = options.spotifyAvailable ?? true;
  const emptyMusicians = options.emptyMusicians ?? false;
  let tracksRequestCount = 0;
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse(
        authUser({ name: "Music User", email: "music@example.com" }),
      );
    }

    if (url === "/api/spotify/status") {
      return jsonResponse({
        error: false,
        data: {
          available: spotifyAvailable,
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

    if (url === `/api/music/albums?page=1&per_page=${ALBUMS_PER_PAGE}`) {
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
          per_page: ALBUMS_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === `/api/music/musicians?page=1&per_page=${MUSICIANS_PER_PAGE}`) {
      if (emptyMusicians) {
        return jsonResponse({
          error: false,
          data: {
            musicians: [],
            total: 0,
            page: 1,
            per_page: MUSICIANS_PER_PAGE,
            total_pages: 0,
          },
        });
      }

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
          per_page: MUSICIANS_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === `/api/music/tracks?limit=${TRACKS_INFINITE_PAGE_SIZE}&offset=0`) {
      tracksRequestCount += 1;

      if (options.failFirstTracksRequest && tracksRequestCount === 1) {
        return jsonResponse({
          error: true,
          message: "The tracks library is temporarily unavailable.",
        });
      }

      return jsonResponse({
        error: false,
        data: {
          tracks: [track(1, "Alabaster"), track(2, "Borrowed Light")],
          total: 2,
          offset: 0,
          limit: TRACKS_INFINITE_PAGE_SIZE,
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

async function renderMusicRoute(
  initialEntry: string,
  options: MockMusicFetchOptions = {},
) {
  mockMusicFetch(options);

  return renderRoute(initialEntry);
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  restoreMatchMedia();
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

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(() => {
      expect(screen.getByText("Nina Simone")).toBeInTheDocument();
    });
  }, 10_000);

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

describe("musicians tab empty state", () => {
  it("renders and announces the empty musicians state", async () => {
    setReducedMotionPreference(true);

    await renderMusicRoute("/music/?tab=musicians", { emptyMusicians: true });

    expect(
      await screen.findByText("No musicians found in your library."),
    ).toBeInTheDocument();

    await waitFor(() => {
      const statusRegions = screen.getAllByRole("status");
      expect(
        statusRegions.some(
          region => region.textContent === "No musicians found",
        ),
      ).toBe(true);
    });
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

describe("music route more menu", () => {
  it("opens the Request Album dialog from More options", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/");

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Request Album" }),
    );

    expect(
      await screen.findByRole("dialog", { name: "Request Album" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByLabelText("Album title")).toHaveFocus();
    });
  });

  it("disables Request Album when Spotify search is unavailable", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/", { spotifyAvailable: false });

    await user.click(screen.getByRole("button", { name: "More options" }));

    const requestAlbumItem = await screen.findByRole("menuitem", {
      name: /Request Album unavailable/i,
    });

    expect(requestAlbumItem).toHaveAttribute("data-disabled");
    expect(requestAlbumItem).toHaveAttribute(
      "title",
      "Spotify search is unavailable on this server.",
    );
  });
});

describe("music route tracks tab", () => {
  it("shows an API failure and loads tracks after retrying", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/?tab=tracks", {
      failFirstTracksRequest: true,
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The tracks library is temporarily unavailable.",
    );
    expect(
      screen.queryByText("No tracks found in your library."),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByRole("list", { name: "Tracks" })).toBeInTheDocument();
    expect(screen.getByText("Alabaster")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

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
