import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ProgressBar from "@/components/ProgressBar";
import {
  MOTION_PROGRESS_FILL_CLASS,
  MOTION_PROGRESS_THUMB_REVEAL_CLASS,
} from "@/lib/constants";

describe("ProgressBar", () => {
  it("seeks with keyboard controls", () => {
    const onSeek = vi.fn();

    render(
      <ProgressBar
        currentTime={50}
        duration={120}
        onSeek={onSeek}
        variant="trailer"
      />,
    );

    const slider = screen.getByRole("slider", {
      name: "Seek through track",
    });

    for (const [key, expectedTime] of [
      ["ArrowLeft", 45],
      ["ArrowDown", 45],
      ["ArrowRight", 55],
      ["ArrowUp", 55],
      ["Home", 0],
      ["End", 120],
      ["PageDown", 38],
      ["PageUp", 62],
    ] as const) {
      fireEvent.keyDown(slider, { key });
      expect(onSeek).toHaveBeenLastCalledWith(expectedTime);
    }
  });

  it.each([0, Number.NaN, Number.POSITIVE_INFINITY])(
    "does not seek when duration is %s",
    duration => {
      const onSeek = vi.fn();

      render(
        <ProgressBar
          currentTime={10}
          duration={duration}
          onSeek={onSeek}
          variant="trailer"
        />,
      );

      const slider = screen.getByRole("slider");

      fireEvent.keyDown(slider, { key: "ArrowRight" });
      fireEvent.pointerDown(slider, {
        clientX: 50,
        isPrimary: true,
        pointerId: 1,
      });

      expect(slider).toHaveAttribute("aria-disabled", "true");
      expect(onSeek).not.toHaveBeenCalled();
    },
  );

  it("uses custom accessible labels", () => {
    render(
      <ProgressBar
        currentTime={30}
        duration={90}
        onSeek={vi.fn()}
        variant="trailer"
        ariaLabel="Seek through trailer"
        groupLabel="Playback progress"
      />,
    );

    expect(
      screen.getByRole("group", { name: "Playback progress" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("slider", { name: "Seek through trailer" }),
    ).toBeInTheDocument();
  });

  it("applies trailer variant classes", () => {
    render(
      <ProgressBar
        currentTime={30}
        duration={120}
        onSeek={vi.fn()}
        variant="trailer"
      />,
    );

    const group = screen.getByRole("group");
    const slider = screen.getByRole("slider");
    const fill = slider.firstElementChild;
    const thumb = fill?.nextElementSibling;

    expect(group).toHaveClass("mb-4", "w-full");
    expect(slider).toHaveClass("h-1.5", "focus:ring-amber-400");
    expect(fill).toHaveClass(
      "bg-amber-400",
      ...MOTION_PROGRESS_FILL_CLASS.split(" "),
    );
    expect(thumb).toHaveClass(
      "size-3",
      "group-hover:opacity-100",
      "group-focus:opacity-100",
      ...MOTION_PROGRESS_THUMB_REVEAL_CLASS.split(" "),
    );
  });

  it("seeks from pointer position", () => {
    const onSeek = vi.fn();

    render(
      <ProgressBar
        currentTime={0}
        duration={200}
        onSeek={onSeek}
        variant="trailer"
      />,
    );

    const slider = screen.getByRole("slider");
    vi.spyOn(slider, "getBoundingClientRect").mockReturnValue({
      bottom: 10,
      height: 10,
      left: 20,
      right: 220,
      top: 0,
      width: 200,
      x: 20,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerDown(slider, {
      clientX: 120,
      isPrimary: true,
      pointerId: 1,
    });

    expect(onSeek).toHaveBeenCalledWith(100);
  });
});
