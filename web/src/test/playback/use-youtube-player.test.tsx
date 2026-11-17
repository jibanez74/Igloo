import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useYouTubePlayer } from "@/hooks/useYouTubePlayer";

type FakeInstance = {
  events: YT.PlayerEvents;
  muted: boolean;
  state: number;
  mute: ReturnType<typeof vi.fn>;
  unMute: ReturnType<typeof vi.fn>;
  playVideo: ReturnType<typeof vi.fn>;
  pauseVideo: ReturnType<typeof vi.fn>;
  seekTo: ReturnType<typeof vi.fn>;
  setVolume: ReturnType<typeof vi.fn>;
};

const instances: FakeInstance[] = [];

// `useYouTubePlayer` skips the iframe_api script when `window.YT.Player`
// already exists, so a fake API installed up front is all the hook needs.
function installFakeYouTube() {
  class FakePlayer {
    events: YT.PlayerEvents;
    muted = false;
    mute = vi.fn(() => {
      this.muted = true;
    });
    unMute = vi.fn(() => {
      this.muted = false;
    });
    playVideo = vi.fn();

    constructor(_id: string, options: YT.PlayerOptions) {
      this.events = options.events ?? {};
      instances.push(this);
    }

    pauseVideo = vi.fn();
    seekTo = vi.fn();
    setVolume = vi.fn();
    state = 5;
    getVolume() {
      return 100;
    }
    getCurrentTime() {
      return 0;
    }
    getDuration() {
      return 100;
    }
    isMuted() {
      return this.muted;
    }
    getPlayerState() {
      return this.state;
    }
    destroy() {}
  }

  (window as unknown as { YT: unknown }).YT = {
    Player: FakePlayer,
    PlayerState: { UNSTARTED: -1, ENDED: 0, PLAYING: 1, PAUSED: 2, BUFFERING: 3, CUED: 5 },
    PlayerError: { INVALID_PARAM: 2, HTML5_ERROR: 5, NOT_FOUND: 100, NOT_ALLOWED: 101, NOT_ALLOWED_DISGUISE: 150 },
  };
}

function Probe({ muteOnAutoplayBlocked }: { muteOnAutoplayBlocked?: boolean }) {
  const { containerRef, isReady, isMuted, unmute } = useYouTubePlayer({
    videoId: "one",
    autoplay: true,
    controls: false,
    muteOnAutoplayBlocked,
  });

  return (
    <div>
      <div ref={containerRef} />
      <output>{`${isReady ? "ready" : "loading"}:${isMuted ? "muted" : "sound"}`}</output>
      <button type="button" onClick={unmute}>
        unmute
      </button>
    </div>
  );
}

async function renderProbe(muteOnAutoplayBlocked?: boolean) {
  installFakeYouTube();
  render(<Probe muteOnAutoplayBlocked={muteOnAutoplayBlocked} />);
  await waitFor(() => expect(instances).toHaveLength(1));
  const player = instances[0];
  const target = player as unknown as YT.Player;
  act(() => player.events.onReady?.({ target }));
  return { player, target };
}

afterEach(() => {
  instances.length = 0;
  delete (window as unknown as { YT?: unknown }).YT;
});

describe("useYouTubePlayer autoplay fallback", () => {
  it("retries a blocked autoplay muted, once, and unmutes on request", async () => {
    const { player, target } = await renderProbe(true);
    expect(screen.getByText("ready:sound")).toBeInTheDocument();

    act(() => player.events.onAutoplayBlocked?.({ target }));
    expect(player.mute).toHaveBeenCalledTimes(1);
    expect(player.playVideo).toHaveBeenCalledTimes(1);
    expect(screen.getByText("ready:muted")).toBeInTheDocument();

    // A second refusal (the muted retry itself) is not retried again.
    act(() => player.events.onAutoplayBlocked?.({ target }));
    expect(player.mute).toHaveBeenCalledTimes(1);
    expect(player.playVideo).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: "unmute" }));
    expect(player.unMute).toHaveBeenCalledTimes(1);
    expect(screen.getByText("ready:sound")).toBeInTheDocument();
  });

  it("leaves a blocked autoplay alone without the option", async () => {
    const { player, target } = await renderProbe();

    act(() => player.events.onAutoplayBlocked?.({ target }));
    expect(player.mute).not.toHaveBeenCalled();
    expect(player.playVideo).not.toHaveBeenCalled();
    expect(screen.getByText("ready:sound")).toBeInTheDocument();
  });
});

function ControlsProbe() {
  const {
    containerRef,
    isPlaying,
    isMuted,
    togglePlay,
    seekTo,
    seekForward,
    setVolume,
    toggleMute,
  } = useYouTubePlayer({ videoId: "one", autoplay: true, controls: false });

  return (
    <div>
      <div ref={containerRef} />
      <output>{`${isPlaying ? "playing" : "paused"}:${isMuted ? "muted" : "sound"}`}</output>
      <button type="button" onClick={togglePlay}>
        toggle
      </button>
      <button
        type="button"
        onClick={() => {
          seekTo(30);
          seekForward(10);
          setVolume(50);
          toggleMute();
        }}
      >
        everything
      </button>
    </div>
  );
}

async function renderControlsProbe() {
  installFakeYouTube();
  render(<ControlsProbe />);
  await waitFor(() => expect(instances).toHaveLength(1));
  const player = instances[0];
  return { player, target: player as unknown as YT.Player };
}

describe("useYouTubePlayer controls", () => {
  it("leaves the embed alone until it is ready", async () => {
    const { player, target } = await renderControlsProbe();

    // Before onReady the real embed has none of these methods; calling them
    // threw a TypeError when a shortcut was pressed while the trailer loaded.
    fireEvent.click(screen.getByRole("button", { name: "everything" }));
    fireEvent.click(screen.getByRole("button", { name: "toggle" }));
    expect(player.seekTo).not.toHaveBeenCalled();
    expect(player.setVolume).not.toHaveBeenCalled();
    expect(player.mute).not.toHaveBeenCalled();
    expect(player.playVideo).not.toHaveBeenCalled();

    act(() => player.events.onReady?.({ target }));
    fireEvent.click(screen.getByRole("button", { name: "everything" }));
    expect(player.seekTo).toHaveBeenLastCalledWith(10, true);
    expect(player.setVolume).toHaveBeenCalledWith(50);
    expect(player.mute).toHaveBeenCalledTimes(1);
  });

  it("keeps the play state through a seek's buffering, playing or paused", async () => {
    const { player, target } = await renderControlsProbe();
    act(() => player.events.onReady?.({ target }));
    const enter = (state: number) => {
      player.state = state;
      act(() =>
        player.events.onStateChange?.({ target, data: state } as YT.OnStateChangeEvent),
      );
    };

    // Playing, then a seek: BUFFERING still reads as playing, and a toggle
    // there pauses.
    enter(1);
    enter(3);
    expect(screen.getByText("playing:sound")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "toggle" }));
    expect(player.pauseVideo).toHaveBeenCalledTimes(1);
    expect(player.playVideo).not.toHaveBeenCalled();

    // Paused, then a seek: BUFFERING still reads as paused, and a toggle
    // there plays.
    enter(2);
    enter(3);
    expect(screen.getByText("paused:sound")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "toggle" }));
    expect(player.playVideo).toHaveBeenCalledTimes(1);
  });

  it("picks up a mute made outside its own controls", async () => {
    const { player, target } = await renderControlsProbe();
    act(() => player.events.onReady?.({ target }));
    player.state = 1;
    act(() =>
      player.events.onStateChange?.({ target, data: 1 } as YT.OnStateChangeEvent),
    );

    player.muted = true;
    await waitFor(() =>
      expect(screen.getByText("playing:muted")).toBeInTheDocument(),
    );
  });
});

describe("useYouTubePlayer API script", () => {
  it("marks a replacement script that also timed out, so the next retry replaces it", async () => {
    vi.useFakeTimers();
    try {
      const scriptSelector = 'script[src="https://www.youtube.com/iframe_api"]';
      function Loader() {
        const { containerRef, error, retry } = useYouTubePlayer({
          videoId: "one",
        });
        return (
          <div>
            <div ref={containerRef} />
            <output>{error ?? "no error"}</output>
            <button type="button" onClick={retry}>
              retry
            </button>
          </div>
        );
      }
      render(<Loader />);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(15_000);
      });
      expect(screen.getByText("Failed to load the YouTube player.")).toBeInTheDocument();
      const first = document.querySelector(scriptSelector);

      fireEvent.click(screen.getByRole("button", { name: "retry" }));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      const second = document.querySelector(scriptSelector);
      expect(second).not.toBe(first);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(15_000);
      });
      expect(second).toHaveAttribute("data-yt-api-load-error", "true");

      fireEvent.click(screen.getByRole("button", { name: "retry" }));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      expect(document.querySelector(scriptSelector)).not.toBe(second);
    } finally {
      document.querySelectorAll('script[src*="youtube.com/iframe_api"]').forEach((node) => node.remove());
      vi.useRealTimers();
    }
  });
});
