import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import UpNextOverlay from "@/components/playback/UpNextOverlay";
import type { UpNextItem } from "@/types/playback";

function item(overrides: Partial<UpNextItem> = {}): UpNextItem {
  return {
    title: "S1 E4 · The Long Night",
    stillUrl: "https://image.tmdb.org/t/p/w500/still.jpg",
    resume: false,
    onPlay: () => {},
    ...overrides,
  };
}

describe("UpNextOverlay", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("names the next episode, focuses Play now, and counts down to it", async () => {
    const onPlay = vi.fn();
    render(
      <UpNextOverlay
        item={item()}
        countdownSec={3}
        onPlay={onPlay}
        onCancel={() => {}}
      />,
    );

    const region = screen.getByRole("region", { name: "Up next" });
    expect(region).toHaveTextContent("S1 E4 · The Long Night");
    expect(region).toHaveTextContent("Playing in 3s");
    expect(screen.getByRole("button", { name: "Play now" })).toHaveFocus();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(region).toHaveTextContent("Playing in 1s");
    expect(onPlay).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
    expect(onPlay).toHaveBeenCalledTimes(1);
  });

  it("labels a resumable episode and plays it on click", () => {
    const onPlay = vi.fn();
    render(
      <UpNextOverlay
        item={item({ resume: true })}
        countdownSec={10}
        onPlay={onPlay}
        onCancel={() => {}}
      />,
    );

    expect(screen.getByText("Resuming in 10s")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Resume now" }));
    expect(onPlay).toHaveBeenCalledTimes(1);
  });

  it("cancels without playing and stops the countdown on unmount", async () => {
    const onPlay = vi.fn();
    const onCancel = vi.fn();
    const { unmount } = render(
      <UpNextOverlay
        item={item()}
        countdownSec={2}
        onPlay={onPlay}
        onCancel={onCancel}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalledTimes(1);

    unmount();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });
    expect(onPlay).not.toHaveBeenCalled();
  });
});
