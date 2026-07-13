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
    ["ArrowRight", "onSeekForward"],
    ["l", "onSeekForward"],
    ["Home", "onSeekToStart"],
    ["0", "onSeekToStart"],
    ["m", "onToggleMute"],
    ["f", "onToggleFullscreen"],
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

  it("ignores shortcuts until the video can play", () => {
    const onTogglePlay = vi.fn();
    const { press } = renderHarness({ onTogglePlay }, { readyState: 0 });

    press("k");
    expect(onTogglePlay).not.toHaveBeenCalled();
  });

  it("still exits fullscreen with Escape before the video is ready", () => {
    const onEscape = vi.fn();
    const { press } = renderHarness(
      { onEscape, fullscreenActive: true },
      { readyState: 0 },
    );

    press("Escape");
    expect(onEscape).toHaveBeenCalledTimes(1);
  });
});
