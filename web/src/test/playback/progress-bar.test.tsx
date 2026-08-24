import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ProgressBar from "@/components/playback/ProgressBar";
import {
  CARD_FOCUS_WITHIN_RING_CLASS,
  MOTION_PROGRESS_FILL_CLASS,
  MOTION_PROGRESS_THUMB_REVEAL_CLASS,
} from "@/lib/constants";

describe("ProgressBar", () => {
  // jsdom does not implement native range-input keyboard stepping, so this
  // exercises the component's own keydown handler (which also overrides the
  // native 1s step with friendlier increments in real browsers).
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
      fireEvent.change(slider, { target: { value: "50" } });

      expect(slider).toHaveAttribute("aria-disabled", "true");
      expect(slider).toHaveAttribute("tabindex", "-1");
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
    const bar = slider.parentElement;
    const fill = bar?.firstElementChild;
    const thumb = fill?.nextElementSibling;

    expect(group).toHaveClass("mb-4", "w-full");
    expect(bar).toHaveClass("h-1.5", "focus-within:ring-ring");
    expect(fill).toHaveClass(
      "bg-primary",
      ...MOTION_PROGRESS_FILL_CLASS.split(" "),
    );
    expect(thumb).toHaveClass(
      "size-3",
      "group-hover:opacity-100",
      "group-focus-within:opacity-100",
      ...MOTION_PROGRESS_THUMB_REVEAL_CLASS.split(" "),
    );
  });

  it.each(["expanded", "minimized", "video", "trailer"] as const)(
    "uses a semantic thumb and the whole-group focus ring on the %s variant",
    variant => {
      render(
        <ProgressBar
          currentTime={30}
          duration={120}
          onSeek={vi.fn()}
          variant={variant}
        />,
      );

      const slider = screen.getByRole("slider");
      const bar = slider.parentElement;
      const thumb = bar?.firstElementChild?.nextElementSibling;

      // Every variant sits on a themed panel, so the thumb is bg-foreground —
      // the over-media white exception (design-system §1.2) does not apply.
      expect(thumb).toHaveClass("bg-foreground");
      expect(thumb).not.toHaveClass("bg-white");
      expect(bar).toHaveClass(...CARD_FOCUS_WITHIN_RING_CLASS.split(" "));
    },
  );

  it("seeks when the range input value changes", () => {
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

    fireEvent.change(slider, { target: { value: "100" } });
    expect(onSeek).toHaveBeenLastCalledWith(100);

    fireEvent.change(slider, { target: { value: "999" } });
    expect(onSeek).toHaveBeenLastCalledWith(200);
  });

  it("keeps the scrubbed position until the pointer is released", () => {
    const onSeek = vi.fn();

    const { rerender } = render(
      <ProgressBar
        currentTime={50}
        duration={200}
        onSeek={onSeek}
        variant="trailer"
      />,
    );

    const slider = screen.getByRole("slider") as HTMLInputElement;

    fireEvent.change(slider, { target: { value: "100" } });

    // A stale timeupdate re-render must not snap the thumb back mid-drag.
    rerender(
      <ProgressBar
        currentTime={51}
        duration={200}
        onSeek={onSeek}
        variant="trailer"
      />,
    );
    expect(slider.value).toBe("100");

    fireEvent.pointerUp(slider);
    rerender(
      <ProgressBar
        currentTime={102}
        duration={200}
        onSeek={onSeek}
        variant="trailer"
      />,
    );
    expect(slider.value).toBe("102");
  });

  it("clears the scrubbed position when the pointer is cancelled", () => {
    const onSeek = vi.fn();

    const { rerender } = render(
      <ProgressBar
        currentTime={50}
        duration={200}
        onSeek={onSeek}
        variant="trailer"
      />,
    );

    const slider = screen.getByRole("slider") as HTMLInputElement;

    fireEvent.change(slider, { target: { value: "100" } });
    fireEvent.pointerCancel(slider);

    rerender(
      <ProgressBar
        currentTime={51}
        duration={200}
        onSeek={onSeek}
        variant="trailer"
      />,
    );
    expect(slider.value).toBe("51");
  });

  it("shows hour-padded time labels and spoken value text past one hour", () => {
    render(
      <ProgressBar
        currentTime={300}
        duration={7500}
        onSeek={vi.fn()}
        variant="video"
      />,
    );

    // Current time is padded to the duration's h:mm:ss shape so the readout
    // width stays stable across the hour mark.
    expect(screen.getByText("0:05:00")).toBeInTheDocument();
    expect(screen.getByText("2:05:00")).toBeInTheDocument();
    expect(screen.getByRole("slider")).toHaveAttribute(
      "aria-valuetext",
      "5 minutes of 2 hours 5 minutes",
    );
  });

  it("drops a pending scrub value when resetKey changes", () => {
    const onSeek = vi.fn();

    const { rerender } = render(
      <ProgressBar
        currentTime={50}
        duration={200}
        onSeek={onSeek}
        variant="expanded"
        resetKey={1}
      />,
    );

    const slider = screen.getByRole("slider") as HTMLInputElement;

    fireEvent.change(slider, { target: { value: "180" } });
    expect(slider.value).toBe("180");

    // Track auto-advances mid-drag: the new track must not inherit the old
    // track's scrub position.
    rerender(
      <ProgressBar
        currentTime={0}
        duration={240}
        onSeek={onSeek}
        variant="expanded"
        resetKey={2}
      />,
    );
    expect(slider.value).toBe("0");
  });
});
