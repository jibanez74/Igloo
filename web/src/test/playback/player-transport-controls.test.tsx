import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PlayerTransportControls from "@/components/playback/PlayerTransportControls";

function renderTransport(variant: "expanded" | "minimized") {
  render(
    <PlayerTransportControls
      variant={variant}
      isPlaying={false}
      isLoading={false}
      hasNext
      previousAriaLabel="Previous track"
      nextAriaLabel="Next track"
      playPauseAriaLabel="Play"
      onPrevious={vi.fn()}
      onTogglePlay={vi.fn()}
      onNext={vi.fn()}
    />,
  );
}

describe("PlayerTransportControls", () => {
  // The play button is a primary control in both chromes, so its glow is the
  // shadow-glow token there too (design-system §1.4), not a literal halo.
  it.each(["expanded", "minimized"] as const)(
    "gives the %s play button the glow token",
    variant => {
      renderTransport(variant);

      const play = screen.getByRole("button", { name: "Play" });
      expect(play).toHaveClass("shadow-glow");
      expect(play).not.toHaveClass("shadow-xl");
    },
  );
});
