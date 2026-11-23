import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL, stubApi } from "../helpers/api";
import { authUser } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

vi.mock("@/hooks/useAudioPlayerActions", () => import("../helpers/audio-player"));

function mockTrailerFetch(movieResponse: () => Promise<Response>, tmdbId = "19") {
  stubApi({ [`/api/tmdb/movies/${tmdbId}`]: () => movieResponse() });
}

describe("trailer route load errors", () => {
  // The server answers a lowercase constant; the UI owns the copy (§3.4).
  it("words a failed TMDB lookup for the reader instead of echoing the server", async () => {
    mockTrailerFetch(() =>
      jsonResponse({ error: true, message: "failed to fetch movie from tmdb" }, 500),
    );

    await renderRoute("/trailer?mediaType=movie&tmdbId=19");

    expect(
      await screen.findByRole("heading", { name: "Unable to Load Trailer" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Couldn’t load the trailer details from TMDB.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/failed to fetch movie from tmdb/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try Again" })).toBeInTheDocument();
  });

  it("says the movie is missing on a 404", async () => {
    mockTrailerFetch(() =>
      jsonResponse({ error: true, message: "movie not found" }, 404),
    );

    await renderRoute("/trailer?mediaType=movie&tmdbId=19");

    expect(
      await screen.findByText("We couldn’t find that movie on TMDB."),
    ).toBeInTheDocument();
  });
});

describe("trailer route without a trailer", () => {
  it("lays the Go Back icon and label out on one line", async () => {
    mockTrailerFetch(() =>
      jsonResponse({
        error: false,
        data: { movie: { title: "Resident Evil", videos: { results: [] } } },
      }),
    );

    await renderRoute("/trailer?mediaType=movie&tmdbId=19");

    expect(
      await screen.findByRole("heading", { name: "No Trailer Available" }),
    ).toBeInTheDocument();
    // Without inline-flex the block-level svg stacks above the label.
    expect(screen.getByRole("button", { name: "Go Back" })).toHaveClass(
      "inline-flex",
      "items-center",
    );
  });
});

describe("trailer route with a hand-edited link", () => {
  it("opens the dialog without a trailer instead of failing the route", async () => {
    const movieResponse = vi.fn(() => jsonResponse({ error: false, data: {} }));
    mockTrailerFetch(movieResponse, "abc");

    await renderRoute("/trailer?mediaType=movie&tmdbId=abc");

    expect(
      await screen.findByRole("heading", { name: "No Trailer Available" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Something went wrong!")).not.toBeInTheDocument();
    expect(movieResponse).not.toHaveBeenCalled();
  });
});

type FakeTrailerPlayer = {
  events: YT.PlayerEvents;
  volume: number;
  muted: boolean;
  playVideo: ReturnType<typeof vi.fn>;
  pauseVideo: ReturnType<typeof vi.fn>;
  setVolume: ReturnType<typeof vi.fn>;
};

// useYouTubePlayer skips the iframe_api script when window.YT.Player exists.
function installFakeYouTube(players: FakeTrailerPlayer[]) {
  class FakePlayer {
    events: YT.PlayerEvents;
    volume = 0;
    muted = false;
    playVideo = vi.fn();
    pauseVideo = vi.fn();
    setVolume = vi.fn((level: number) => {
      this.volume = level;
    });
    constructor(_id: string, options: YT.PlayerOptions) {
      this.events = options.events ?? {};
      players.push(this);
    }
    mute() {
      this.muted = true;
    }
    unMute() {
      this.muted = false;
    }
    isMuted() {
      return this.muted;
    }
    getVolume() {
      return this.volume;
    }
    getDuration() {
      return 120;
    }
    getCurrentTime() {
      return 0;
    }
    getPlayerState() {
      return 2;
    }
    seekTo() {}
    destroy() {}
  }
  (window as unknown as { YT: unknown }).YT = {
    Player: FakePlayer,
    PlayerState: { UNSTARTED: -1, ENDED: 0, PLAYING: 1, PAUSED: 2, BUFFERING: 3, CUED: 5 },
    PlayerError: { INVALID_PARAM: 2, HTML5_ERROR: 5, NOT_FOUND: 100, NOT_ALLOWED: 101, NOT_ALLOWED_DISGUISE: 150 },
  };
}

describe("trailer route player", () => {
  const originalSession = Object.getOwnPropertyDescriptor(navigator, "mediaSession");

  afterEach(() => {
    delete (window as unknown as { YT?: unknown }).YT;
    if (originalSession) Object.defineProperty(navigator, "mediaSession", originalSession);
    else Reflect.deleteProperty(navigator, "mediaSession");
  });

  async function openPlayer() {
    const players: FakeTrailerPlayer[] = [];
    installFakeYouTube(players);
    const handlers = new Map<string, ((details: MediaSessionActionDetails) => void) | null>();
    Object.defineProperty(navigator, "mediaSession", {
      configurable: true,
      value: {
        metadata: null,
        playbackState: "none",
        setActionHandler: (
          action: string,
          handler: ((details: MediaSessionActionDetails) => void) | null,
        ) => handlers.set(action, handler),
        setPositionState: vi.fn(),
      },
    });
    // Close lands on Home, which renders its error states from these: every
    // request but the signed-in user fails, whatever Home asks for.
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) =>
        requestURL(input) === "/api/auth/user"
          ? jsonResponse(authUser())
          : jsonResponse({ error: true, message: "unavailable" }, 503),
      ),
    );
    const view = await renderRoute("/trailer?videoKey=signal-fire&returnTo=/");
    await waitFor(() => expect(players).toHaveLength(1));
    const player = players[0];
    act(() => player.events.onReady?.({ target: player as unknown as YT.Player }));
    return { ...view, player, handlers };
  }

  // A Tab stop there handed focus, the shortcuts and Escape to the iframe.
  it("keeps the embed out of the tab order", async () => {
    const { container } = await openPlayer();
    const embed = document.querySelector('[id^="yt-player-"]')?.parentElement;

    expect(embed).toHaveAttribute("inert");
    expect(container).toBeTruthy();
  });

  it("takes the OS media controls for the trailer", async () => {
    const { player, handlers } = await openPlayer();

    expect(handlers.get("play")).toBeTypeOf("function");
    act(() => handlers.get("play")?.({ action: "play" }));
    expect(player.playVideo).toHaveBeenCalled();
  });

  it("brings the sound back when unmuting at volume 0", async () => {
    const { player } = await openPlayer();

    fireEvent.click(screen.getByRole("button", { name: /^Unmute/ }));
    expect(player.setVolume).toHaveBeenCalledWith(50);
  });

  it("hands focus to the page once Close has navigated away", async () => {
    await openPlayer();

    fireEvent.click(screen.getByRole("button", { name: /^Close trailer/ }));
    await waitFor(() => {
      expect(document.activeElement?.id).toBe("main");
    });
  });
});
