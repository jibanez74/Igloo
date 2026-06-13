import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MoviePlayerControls from "@/components/MoviePlayerControls";
import {
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
} from "@/lib/constants";

describe("MoviePlayerControls", () => {
  it("keeps playback buttons labelled and on shared chrome contracts", () => {
    render(
      <MoviePlayerControls
        chromeFullscreenMode
        controlsVisible
        isFullscreen={false}
        isImmersiveViewport
        currentTime={12}
        duration={120}
        displayedDuration={120}
        playing={false}
        qualityLabel="Direct"
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
      "Seek backward 10 seconds",
      "Play",
      "Seek forward 10 seconds",
      "Exit expanded view",
    ]) {
      expect(screen.getByRole("button", { name })).toHaveClass(
        ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
      );
    }

    expect(
      screen.getByRole("button", { name: "Adjust volume" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
  });
});
