import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import WatchProgressBar from "@/components/shared/WatchProgressBar";

function renderBar(progressSec: number, durationSec: number) {
  const { container } = render(
    <WatchProgressBar progressSec={progressSec} durationSec={durationSec} />,
  );
  const track = container.firstElementChild as HTMLElement;
  const fill = track.firstElementChild as HTMLElement;
  return { track, fill };
}

describe("WatchProgressBar", () => {
  it("fills in proportion to the saved position", () => {
    expect(renderBar(675, 2700).fill.style.width).toBe("25%");
  });

  it("stays empty when the duration is unknown", () => {
    expect(renderBar(600, 0).fill.style.width).toBe("0%");
  });

  it("never overflows the track", () => {
    expect(renderBar(3000, 2700).fill.style.width).toBe("100%");
    expect(renderBar(-5, 2700).fill.style.width).toBe("0%");
  });

  it("is decorative: the paired text carries the position for assistive technology", () => {
    expect(renderBar(675, 2700).track).toHaveAttribute("aria-hidden", "true");
  });
});
