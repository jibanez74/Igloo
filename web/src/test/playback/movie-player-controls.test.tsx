import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MoviePlayerControls from "@/components/movies/MoviePlayerControls";
import {
  MOVIE_SEEK_STEP_SEC,
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

  it("pads the current time to h:mm:ss for movies over an hour", () => {
    render(
      <MoviePlayerControls
        chromeFullscreenMode
        controlsVisible
        isFullscreen={false}
        isImmersiveViewport
        currentTime={12}
        duration={7500}
        displayedDuration={7500}
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

    // Rendered by both the chrome readout and the progress bar labels.
    expect(screen.getAllByText("0:00:12").length).toBeGreaterThan(0);
    expect(screen.getAllByText("2:05:00").length).toBeGreaterThan(0);
  });
});
