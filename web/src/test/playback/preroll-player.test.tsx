import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import PrerollPlayer from "@/components/playback/PrerollPlayer";
import { PREROLL_MOVIE_WARMUP_SEC } from "@/lib/constants";
import type { PrerollTrailer } from "@/types";

type FakeYouTubePlayer = {
  isReady: boolean;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  error: string | null;
  options: {
    videoId: string | null;
    onEnd?: () => void;
    onError?: (code: number) => void;
  } | null;
  togglePlay: ReturnType<typeof vi.fn>;
  seekTo: ReturnType<typeof vi.fn>;
};

const fake = vi.hoisted((): FakeYouTubePlayer => ({
  isReady: true,
  isPlaying: true,
  currentTime: 0,
  duration: 120,
  error: null,
  options: null,
  togglePlay: vi.fn(),
  seekTo: vi.fn(),
}));

vi.mock("@/hooks/useYouTubePlayer", () => ({
  useYouTubePlayer: (options: FakeYouTubePlayer["options"]) => {
    fake.options = options;
    return {
      containerRef: () => {},
      isReady: fake.isReady,
      isPlaying: fake.isPlaying,
      currentTime: fake.currentTime,
      duration: fake.duration,
      volume: 100,
      isMuted: false,
      error: fake.error,
      play: vi.fn(),
      pause: vi.fn(),
      togglePlay: fake.togglePlay,
      seekTo: fake.seekTo,
      seekForward: vi.fn(),
      seekBackward: vi.fn(),
      setVolume: vi.fn(),
      mute: vi.fn(),
      unmute: vi.fn(),
      toggleMute: vi.fn(),
      retry: vi.fn(),
    };
  },
}));

const prefersCoarse = vi.hoisted(() => ({ value: false }));
vi.mock("@/hooks/use-coarse-pointer", () => ({
  usePrefersCoarsePointer: () => prefersCoarse.value,
}));

function trailer(key: string, title: string): PrerollTrailer {
  return {
    title,
    youtube_key: key,
    source: "theaters",
    movie_id: null,
    tmdb_id: 1,
  };
}

const queue = [trailer("one", "Signal Fire"), trailer("two", "Glacier Run")];

function renderPreroll(
  overrides: Partial<React.ComponentProps<typeof PrerollPlayer>> = {},
) {
  const props = {
    trailers: queue,
    loadFailed: false,
    chromeFullscreenMode: false,
    controlsVisible: true,
    isFullscreen: false,
    isImmersiveViewport: false,
    onFinish: vi.fn(),
    onWarmup: vi.fn(),
    onShowControls: vi.fn(),
    onToggleFullscreen: vi.fn(),
    onExitFullscreen: vi.fn(),
    ...overrides,
  };
  const view = render(<PrerollPlayer {...props} />);
  return { ...view, props };
}

beforeEach(() => {
  fake.isReady = true;
  fake.isPlaying = true;
  fake.currentTime = 0;
  fake.duration = 120;
  fake.error = null;
  fake.options = null;
});

afterEach(() => {
  prefersCoarse.value = false;
});

describe("PrerollPlayer", () => {
  it("labels the trailer, announces it, and focuses Skip trailer", async () => {
    renderPreroll();

    const region = screen.getByRole("region", {
      name: "Trailers before the movie",
    });
    expect(region).toHaveTextContent("Trailer 1 of 2");
    expect(region).toHaveTextContent("Signal Fire");
    expect(fake.options?.videoId).toBe("one");
    expect(
      screen.getByRole("button", { name: "Skip trailer (N)" }),
    ).toHaveFocus();
    expect(
      await screen.findByText("Trailer 1 of 2: Signal Fire"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Pause trailer (Space or K)" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Trailer keyboard shortcuts/)).toHaveClass(
      "sr-only",
    );
  });

  it("drops shortcut hints on touch-first devices", () => {
    prefersCoarse.value = true;
    renderPreroll();

    expect(
      screen.getByRole("button", { name: "Skip trailer" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Start movie" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Trailer keyboard shortcuts/)).toBeNull();
  });

  it("skips to the next trailer and finishes after the last one", () => {
    const { props } = renderPreroll();

    fireEvent.click(screen.getByRole("button", { name: "Skip trailer (N)" }));
    expect(
      screen.getByRole("region", { name: "Trailers before the movie" }),
    ).toHaveTextContent("Trailer 2 of 2");
    expect(fake.options?.videoId).toBe("two");
    expect(props.onFinish).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Skip trailer (N)" }));
    expect(props.onFinish).toHaveBeenCalledTimes(1);
  });

  it("starts the movie from Start movie and from the S key", () => {
    const first = renderPreroll();
    fireEvent.click(screen.getByRole("button", { name: "Start movie (S)" }));
    expect(first.props.onFinish).toHaveBeenCalledTimes(1);
    first.unmount();

    const second = renderPreroll();
    fireEvent.keyDown(document.body, { key: "s" });
    expect(second.props.onFinish).toHaveBeenCalledTimes(1);
    expect(second.props.onShowControls).toHaveBeenCalled();
  });

  it("drives the trailer from the keyboard: N skips, Space pauses, F goes fullscreen", () => {
    const { props } = renderPreroll();

    fireEvent.keyDown(document.body, { key: "n" });
    expect(fake.options?.videoId).toBe("two");
    fireEvent.keyDown(document.body, { key: " " });
    expect(fake.togglePlay).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(document.body, { key: "F" });
    expect(props.onToggleFullscreen).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(document.body, { key: "Escape" });
    expect(props.onExitFullscreen).toHaveBeenCalledTimes(1);
  });

  it("skips a trailer that fails to embed and finishes when the player itself fails", () => {
    const { props, rerender } = renderPreroll();

    act(() => fake.options?.onError?.(150));
    expect(fake.options?.videoId).toBe("two");
    expect(props.onFinish).not.toHaveBeenCalled();

    // No error code behind it: the IFrame API never loaded.
    fake.error = "Failed to load the YouTube player.";
    rerender(<PrerollPlayer {...props} />);
    expect(props.onFinish).toHaveBeenCalledTimes(1);
  });

  it("moves on when a trailer ends", () => {
    const { props } = renderPreroll();

    act(() => fake.options?.onEnd?.());
    expect(fake.options?.videoId).toBe("two");
    act(() => fake.options?.onEnd?.());
    expect(props.onFinish).toHaveBeenCalledTimes(1);
  });

  it("warms the movie up only in the last trailer's final seconds", () => {
    const { props, rerender } = renderPreroll();

    fake.currentTime = 110;
    rerender(<PrerollPlayer {...props} />);
    expect(props.onWarmup).not.toHaveBeenCalled();

    // The next trailer starts from zero.
    fake.currentTime = 0;
    fireEvent.click(screen.getByRole("button", { name: "Skip trailer (N)" }));
    fake.currentTime = 120 - PREROLL_MOVIE_WARMUP_SEC - 1;
    rerender(<PrerollPlayer {...props} />);
    expect(props.onWarmup).not.toHaveBeenCalled();

    fake.currentTime = 120 - PREROLL_MOVIE_WARMUP_SEC;
    rerender(<PrerollPlayer {...props} />);
    expect(props.onWarmup).toHaveBeenCalledTimes(1);

    fake.currentTime = 119;
    rerender(<PrerollPlayer {...props} />);
    expect(props.onWarmup).toHaveBeenCalledTimes(1);
  });

  it("shows a loading state until the queue arrives and finishes on an empty one", () => {
    const { props, rerender } = renderPreroll({ trailers: undefined });

    expect(
      screen.getByText("Loading trailers…").closest('[role="status"]'),
    ).not.toBeNull();
    expect(props.onFinish).not.toHaveBeenCalled();

    rerender(<PrerollPlayer {...props} trailers={[]} />);
    expect(props.onFinish).toHaveBeenCalledTimes(1);
  });

  it("finishes at once when the queue request failed", () => {
    const { props } = renderPreroll({ trailers: undefined, loadFailed: true });
    expect(props.onFinish).toHaveBeenCalledTimes(1);
  });
});
