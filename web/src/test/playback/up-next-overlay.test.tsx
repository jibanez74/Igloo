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

    // The latch: the interval keeps ticking at zero and the parent hands a
    // fresh onPlay down on every render, but the hand-off starts once.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });
    expect(onPlay).toHaveBeenCalledTimes(1);
  });

  it("announces the offer once, without a tick-by-tick countdown", async () => {
    render(
      <UpNextOverlay
        item={item()}
        countdownSec={3}
        onPlay={() => {}}
        onCancel={() => {}}
      />,
    );

    const announcement =
      "Up next: S1 E4 · The Long Night. Playing in 3 seconds.";
    // LiveAnnouncer holds the message back one beat before posting it.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(100);
    });
    expect(screen.getByText(announcement)).toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    // Still the mount-time wording: the live region never restates the ticks.
    expect(screen.getByText(announcement)).toBeInTheDocument();
  });

  it("keeps a click on the card off the player's toggle surface", () => {
    const onSurfaceClick = vi.fn();
    render(
      // Stands in for the player's fullscreen click-to-toggle surface.
      // react-doctor-disable-next-line react-doctor/click-events-have-key-events, react-doctor/no-static-element-interactions
      <div onClick={onSurfaceClick}>
        <UpNextOverlay
          item={item()}
          countdownSec={10}
          onPlay={() => {}}
          onCancel={() => {}}
        />
      </div>,
    );

    fireEvent.click(screen.getByText("S1 E4 · The Long Night"));
    expect(onSurfaceClick).not.toHaveBeenCalled();
  });

  it("labels a resumable episode and plays it on click", async () => {
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
    await act(async () => {
      await vi.advanceTimersByTimeAsync(100);
    });
    expect(
      screen.getByText(
        "Up next: S1 E4 · The Long Night. Resuming in 10 seconds.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Resume now" }));
    expect(onPlay).toHaveBeenCalledTimes(1);

    // Playing now spends the one hand-off: the card stays mounted while the
    // next episode loads, and the countdown behind it must not fire again.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });
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
