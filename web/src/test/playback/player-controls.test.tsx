import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import PlayerControls from "@/components/playback/PlayerControls";
import {
  MOVIE_SEEK_STEP_SEC,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
} from "@/lib/constants";

const prefersCoarse = vi.hoisted(() => ({ value: false }));
vi.mock("@/hooks/use-coarse-pointer", () => ({
  usePrefersCoarsePointer: () => prefersCoarse.value,
}));

afterEach(() => {
  prefersCoarse.value = false;
});

describe("PlayerControls", () => {
  it("keeps playback buttons labelled and on shared chrome contracts", () => {
    render(
      <PlayerControls
        chromeFullscreenMode
        controlsVisible
        isFullscreen={false}
        isImmersiveViewport
        currentTime={12}
        duration={120}
        displayedDuration={120}
        playing={false}
        modeLabel="Direct"
        chapters={[]}
        videoRef={createRef<HTMLVideoElement>()}
        onSeek={vi.fn()}
        onSeekBackward={vi.fn()}
        onSeekForward={vi.fn()}
        onTogglePlay={vi.fn()}
        onToggleFullscreen={vi.fn()}
        onSelectChapter={vi.fn()}
      />,
    );

    expect(screen.getByRole("contentinfo")).toHaveClass(
      ...MOTION_PLAYER_CHROME_PANEL_CLASS.split(" "),
    );
    expect(
      screen.getByRole("group", { name: "Playback controls" }),
    ).toBeInTheDocument();

    for (const name of [
      `Seek backward ${MOVIE_SEEK_STEP_SEC} seconds (J or Left Arrow)`,
      "Play (Space or K)",
      `Seek forward ${MOVIE_SEEK_STEP_SEC} seconds (L or Right Arrow)`,
      "Exit expanded view (F)",
    ]) {
      expect(screen.getByRole("button", { name })).toHaveClass(
        ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
      );
    }

    expect(
      screen.getByRole("button", { name: "Adjust volume" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
  });

  // A touch-first device has no keyboard to press them on (design-system §1.7).
  it("drops the keyboard shortcuts from button names on a touch-first device", () => {
    prefersCoarse.value = true;

    render(
      <PlayerControls
        chromeFullscreenMode
        controlsVisible
        isFullscreen={false}
        isImmersiveViewport
        currentTime={12}
        duration={120}
        displayedDuration={120}
        playing={false}
        modeLabel="Direct"
        chapters={[]}
        videoRef={createRef<HTMLVideoElement>()}
        onSeek={vi.fn()}
        onSeekBackward={vi.fn()}
        onSeekForward={vi.fn()}
        onTogglePlay={vi.fn()}
        onToggleFullscreen={vi.fn()}
        onSelectChapter={vi.fn()}
      />,
    );

    expect(
      screen
        .getAllByRole("button")
        .map((button) => button.getAttribute("aria-label")),
    ).toEqual([
      `Seek backward ${MOVIE_SEEK_STEP_SEC} seconds`,
      "Play",
      `Seek forward ${MOVIE_SEEK_STEP_SEC} seconds`,
      "Adjust volume",
      "Exit expanded view",
    ]);
  });

  // Audit D13: aria-label on a generic span is ignored by assistive tech, so
  // the badge announces its meaning through visually-hidden text instead.
  it("announces the stream-quality badge with screen-reader context", () => {
    render(
      <PlayerControls
        chromeFullscreenMode
        controlsVisible
        isFullscreen={false}
        isImmersiveViewport
        currentTime={12}
        duration={120}
        displayedDuration={120}
        playing={false}
        modeLabel="Original file — English audio"
        chapters={[]}
        videoRef={createRef<HTMLVideoElement>()}
        onSeek={vi.fn()}
        onSeekBackward={vi.fn()}
        onSeekForward={vi.fn()}
        onTogglePlay={vi.fn()}
        onToggleFullscreen={vi.fn()}
        onSelectChapter={vi.fn()}
      />,
    );

    const badge = screen.getByText("Original file — English audio");
    expect(badge).toHaveTextContent(
      "Current playback mode: Original file — English audio",
    );
    expect(badge.querySelector(".sr-only")).not.toBeNull();
    expect(badge).not.toHaveAttribute("aria-label");
  });

  it("pads the current time to h:mm:ss for movies over an hour", () => {
    render(
      <PlayerControls
        chromeFullscreenMode
        controlsVisible
        isFullscreen={false}
        isImmersiveViewport
        currentTime={12}
        duration={7500}
        displayedDuration={7500}
        playing={false}
        modeLabel="Direct"
        chapters={[]}
        videoRef={createRef<HTMLVideoElement>()}
        onSeek={vi.fn()}
        onSeekBackward={vi.fn()}
        onSeekForward={vi.fn()}
        onTogglePlay={vi.fn()}
        onToggleFullscreen={vi.fn()}
        onSelectChapter={vi.fn()}
      />,
    );

    // Rendered by both the chrome readout and the progress bar labels.
    expect(screen.getAllByText("0:00:12").length).toBeGreaterThan(0);
    expect(screen.getAllByText("2:05:00").length).toBeGreaterThan(0);
  });

  // At phone widths the transport row cannot hold the readout, the mode pill
  // and seven buttons: the pill wrapped onto the seek button and the readout
  // truncated. One readout per width instead, and no pill below `sm`.
  it("shows one time readout per width and hides the mode pill on phones", () => {
    render(
      <PlayerControls
        chromeFullscreenMode={false}
        controlsVisible
        isFullscreen={false}
        isImmersiveViewport={false}
        currentTime={12}
        duration={0}
        displayedDuration={2329}
        playing={false}
        modeLabel="Original file — English audio"
        chapters={[]}
        videoRef={createRef<HTMLVideoElement>()}
        onSeek={vi.fn()}
        onSeekBackward={vi.fn()}
        onSeekForward={vi.fn()}
        onTogglePlay={vi.fn()}
        onToggleFullscreen={vi.fn()}
        onSelectChapter={vi.fn()}
      />,
    );

    const pill = screen.getByText("Original file — English audio");
    expect(pill).toHaveClass("hidden", "sm:inline");

    // Both readouts take the catalog duration, even before the element has
    // reported one (an HLS stream starts at 0:00 otherwise).
    const durations = screen.getAllByText("38:49");
    expect(durations).toHaveLength(2);
    const rowReadout = durations.find(node => node.closest(".sm\\:flex"));
    const barReadout = durations.find(node => node.closest(".sm\\:hidden"));
    expect(rowReadout?.closest(".sm\\:flex")).toHaveClass("hidden");
    expect(barReadout).toBeDefined();
  });
});
