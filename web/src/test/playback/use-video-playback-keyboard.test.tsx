import { useRef } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useVideoPlaybackKeyboard } from "@/hooks/useVideoPlaybackKeyboard";

type HookOptions = Omit<
  Parameters<typeof useVideoPlaybackKeyboard>[0],
  "containerRef" | "videoRef"
>;

function Harness(options: HookOptions) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);

  useVideoPlaybackKeyboard({ containerRef, videoRef, ...options });

  return (
    <div ref={containerRef} data-testid="player">
      <video ref={videoRef} data-testid="video" />
      <input aria-label="Inside input" />
      <input type="range" aria-label="Inside slider" />
      <button type="button">Inside button</button>
    </div>
  );
}

// Testing-library cleanup only removes its own containers.
afterEach(() => {
  document.querySelector("button[data-outside]")?.remove();
});

function renderHarness(
  options: HookOptions = {},
  { readyState = 4 }: { readyState?: number } = {},
) {
  const outsideButton = document.createElement("button");
  outsideButton.textContent = "Outside";
  outsideButton.setAttribute("data-outside", "");
  document.body.appendChild(outsideButton);

  const result = render(<Harness {...options} />);
  const video = screen.getByTestId("video") as HTMLVideoElement;
  Object.defineProperty(video, "readyState", {
    configurable: true,
    value: readyState,
  });

  return {
    ...result,
    video,
    outsideButton,
    press: (key: string, init: KeyboardEventInit = {}) =>
      fireEvent.keyDown(document.body, { key, ...init }),
  };
}

describe("useVideoPlaybackKeyboard", () => {
  it.each([
    [" ", "onTogglePlay"],
    ["k", "onTogglePlay"],
    ["K", "onTogglePlay"],
    ["ArrowLeft", "onSeekBackward"],
    ["j", "onSeekBackward"],
    ["J", "onSeekBackward"],
    ["ArrowRight", "onSeekForward"],
    ["l", "onSeekForward"],
    ["L", "onSeekForward"],
    ["Home", "onSeekToStart"],
    ["0", "onSeekToStart"],
    ["m", "onToggleMute"],
    ["M", "onToggleMute"],
    ["f", "onToggleFullscreen"],
    ["F", "onToggleFullscreen"],
    ["Escape", "onEscape"],
  ] as const)("dispatches %j to %s", (key, callbackName) => {
    const callback = vi.fn();
    const { press } = renderHarness({ [callbackName]: callback });

    press(key);
    expect(callback).toHaveBeenCalledTimes(1);
  });

  it("prevents the default action for consumed shortcuts", () => {
    const { press } = renderHarness({ onTogglePlay: vi.fn() });

    // fireEvent returns false when preventDefault was called.
    expect(press(" ")).toBe(false);
  });

  // A consumed shortcut stops at the document, so a page-level handler (the
  // app shell's own keys) never sees it a second time.
  it("stops consumed shortcuts from reaching window listeners", () => {
    const windowListener = vi.fn();
    window.addEventListener("keydown", windowListener);
    try {
      const { press } = renderHarness({ onTogglePlay: vi.fn() });

      press(" ");
      press("q");
      expect(windowListener.mock.calls.map(([event]) => (event as KeyboardEvent).key)).toEqual(["q"]);
    } finally {
      window.removeEventListener("keydown", windowListener);
    }
  });

  it("adjusts volume through the callback with the configured step", () => {
    const onAdjustVolume = vi.fn();
    const { press } = renderHarness({ onAdjustVolume, volumeStep: 0.1 });

    press("ArrowUp");
    expect(onAdjustVolume).toHaveBeenLastCalledWith(0.1);

    press("ArrowDown");
    expect(onAdjustVolume).toHaveBeenLastCalledWith(-0.1);
  });

  it("falls back to mutating the video element for volume and mute", () => {
    const { press, video } = renderHarness({ volumeStep: 0.25 });
    video.volume = 0.5;
    video.muted = true;

    press("ArrowUp");
    expect(video.volume).toBe(0.75);
    expect(video.muted).toBe(false);

    press("ArrowDown");
    expect(video.volume).toBe(0.5);

    press("m");
    expect(video.muted).toBe(true);
  });

  it("keeps the element's volume between silent and full", () => {
    const { press, video } = renderHarness({ volumeStep: 0.1 });

    video.volume = 0.95;
    press("ArrowUp");
    expect(video.volume).toBe(1);

    video.volume = 0.05;
    press("ArrowDown");
    expect(video.volume).toBe(0);
  });

  it("shows the controls on any shortcut while fullscreen", () => {
    const onShowControls = vi.fn();
    const { press } = renderHarness({
      fullscreenActive: true,
      onShowControls,
      onTogglePlay: vi.fn(),
    });

    press("k");
    expect(onShowControls).toHaveBeenCalledTimes(1);
  });

  it("leaves the controls alone outside fullscreen", () => {
    const onShowControls = vi.fn();
    const { press } = renderHarness({ onShowControls, onTogglePlay: vi.fn() });

    press("k");
    expect(onShowControls).not.toHaveBeenCalled();
  });

  it("ignores keys while typing in an input", () => {
    const onTogglePlay = vi.fn();
    renderHarness({ onTogglePlay });

    fireEvent.keyDown(screen.getByLabelText("Inside input"), { key: "k" });
    expect(onTogglePlay).not.toHaveBeenCalled();
  });

  it("ignores keys with modifiers, when disabled, and outside the player", () => {
    const onTogglePlay = vi.fn();
    const { press, outsideButton, rerender } = renderHarness({ onTogglePlay });

    press("k", { ctrlKey: true });
    press("k", { metaKey: true });
    press("k", { altKey: true });
    fireEvent.keyDown(outsideButton, { key: "k" });
    expect(onTogglePlay).not.toHaveBeenCalled();

    rerender(<Harness onTogglePlay={onTogglePlay} enabled={false} />);
    press("k");
    expect(onTogglePlay).not.toHaveBeenCalled();
  });

  it("plays and pauses before metadata but holds seeks until there is a timeline", () => {
    const onTogglePlay = vi.fn();
    const onSeekBackward = vi.fn();
    const onToggleFullscreen = vi.fn();
    const { press, video } = renderHarness(
      { onTogglePlay, onSeekBackward, onToggleFullscreen },
      { readyState: 0 },
    );

    press("k");
    press("f");
    press("j");
    press("ArrowLeft");
    expect(onTogglePlay).toHaveBeenCalledTimes(1);
    expect(onToggleFullscreen).toHaveBeenCalledTimes(1);
    expect(onSeekBackward).not.toHaveBeenCalled();

    Object.defineProperty(video, "readyState", {
      configurable: true,
      value: HTMLMediaElement.HAVE_METADATA,
    });
    press("j");
    expect(onSeekBackward).toHaveBeenCalledTimes(1);
  });

  it("holds the jump to the start until there is a timeline", () => {
    const onSeekToStart = vi.fn();
    const { press } = renderHarness({ onSeekToStart }, { readyState: 0 });

    press("Home");
    press("0");
    expect(onSeekToStart).not.toHaveBeenCalled();
  });

  it("keeps play and pause during a mid-playback stall", () => {
    const onTogglePlay = vi.fn();
    const { press } = renderHarness(
      { onTogglePlay },
      { readyState: HTMLMediaElement.HAVE_CURRENT_DATA },
    );

    expect(press(" ")).toBe(false);
    expect(onTogglePlay).toHaveBeenCalledTimes(1);
  });

  it("only exits fullscreen once the video has failed", () => {
    const onTogglePlay = vi.fn();
    const onEscape = vi.fn();
    const { press, video } = renderHarness({
      onTogglePlay,
      onEscape,
      fullscreenActive: true,
    });
    Object.defineProperty(video, "error", {
      configurable: true,
      value: { code: 3 },
    });

    press("k");
    press("Escape");
    expect(onTogglePlay).not.toHaveBeenCalled();
    expect(onEscape).toHaveBeenCalledTimes(1);
  });

  it("exits fullscreen with Escape before the video has loaded", () => {
    const onEscape = vi.fn();
    const { press } = renderHarness(
      { onEscape, fullscreenActive: true },
      { readyState: 0 },
    );

    press("Escape");
    expect(onEscape).toHaveBeenCalledTimes(1);
  });

  it("leaves Escape to the page while a failed video is not in fullscreen", () => {
    const onEscape = vi.fn();
    const { press } = renderHarness({ onEscape }, { readyState: 0 });
    Object.defineProperty(screen.getByTestId("video"), "error", {
      configurable: true,
      value: { code: 3 },
    });

    press("Escape");
    expect(onEscape).not.toHaveBeenCalled();
  });

  it("keeps the letter shortcuts on a focused slider but leaves it its arrows", () => {
    const onTogglePlay = vi.fn();
    const onSeekForward = vi.fn();
    const onToggleFullscreen = vi.fn();
    renderHarness({ onTogglePlay, onSeekForward, onToggleFullscreen });
    const slider = screen.getByLabelText("Inside slider");

    expect(fireEvent.keyDown(slider, { key: " " })).toBe(false);
    fireEvent.keyDown(slider, { key: "f" });
    fireEvent.keyDown(slider, { key: "l" });
    fireEvent.keyDown(slider, { key: "ArrowRight" });
    fireEvent.keyDown(slider, { key: "End" });
    expect(onTogglePlay).toHaveBeenCalledTimes(1);
    expect(onToggleFullscreen).toHaveBeenCalledTimes(1);
    expect(onSeekForward).toHaveBeenCalledTimes(1);
  });

  it("leaves Space and Enter to a focused button", () => {
    const onTogglePlay = vi.fn();
    renderHarness({ onTogglePlay });
    const button = screen.getByRole("button", { name: "Inside button" });

    expect(fireEvent.keyDown(button, { key: " " })).toBe(true);
    fireEvent.keyDown(button, { key: "Enter" });
    expect(onTogglePlay).not.toHaveBeenCalled();

    fireEvent.keyDown(button, { key: "k" });
    expect(onTogglePlay).toHaveBeenCalledTimes(1);
  });

  it("ignores a key a widget already handled", () => {
    const onTogglePlay = vi.fn();
    renderHarness({ onTogglePlay });
    const event = new KeyboardEvent("keydown", {
      key: "k",
      bubbles: true,
      cancelable: true,
    });
    event.preventDefault();

    document.body.dispatchEvent(event);
    expect(onTogglePlay).not.toHaveBeenCalled();
  });
});

function PlayerReadyHarness(
  options: Omit<Parameters<typeof useVideoPlaybackKeyboard>[0], "containerRef">,
) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  useVideoPlaybackKeyboard({ containerRef, ...options });
  return <div ref={containerRef} />;
}

describe("useVideoPlaybackKeyboard without a video element", () => {
  it("waits for playerReady, then drives the callbacks", () => {
    const onTogglePlay = vi.fn();
    const onAdjustVolume = vi.fn();
    const onToggleMute = vi.fn();
    const props = { onTogglePlay, onAdjustVolume, onToggleMute, volumeStep: 0.1 };
    const { rerender } = render(<PlayerReadyHarness {...props} />);

    // An embed still loading has nothing to control yet.
    fireEvent.keyDown(document.body, { key: "k" });
    fireEvent.keyDown(document.body, { key: "ArrowUp" });
    expect(onTogglePlay).not.toHaveBeenCalled();
    expect(onAdjustVolume).not.toHaveBeenCalled();

    rerender(<PlayerReadyHarness {...props} playerReady />);
    fireEvent.keyDown(document.body, { key: "k" });
    fireEvent.keyDown(document.body, { key: "ArrowUp" });
    fireEvent.keyDown(document.body, { key: "m" });
    expect(onTogglePlay).toHaveBeenCalledTimes(1);
    expect(onAdjustVolume).toHaveBeenCalledWith(0.1);
    expect(onToggleMute).toHaveBeenCalledTimes(1);
  });
});
