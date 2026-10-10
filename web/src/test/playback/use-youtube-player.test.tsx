import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useYouTubePlayer } from "@/hooks/useYouTubePlayer";

type FakeInstance = {
  events: YT.PlayerEvents;
  mute: ReturnType<typeof vi.fn>;
  unMute: ReturnType<typeof vi.fn>;
  playVideo: ReturnType<typeof vi.fn>;
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

    pauseVideo() {}
    seekTo() {}
    setVolume() {}
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
      return 5;
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
