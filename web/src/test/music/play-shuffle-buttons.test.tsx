import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import PlayShuffleButtons from "@/components/music/PlayShuffleButtons";

describe("PlayShuffleButtons", () => {
  it("names each button for its page and starts the matching queue", async () => {
    const user = userEvent.setup();
    const onPlay = vi.fn();
    const onShuffle = vi.fn();

    render(
      <PlayShuffleButtons
        playLabel="Play All"
        playAriaLabel="Play all 3 tracks"
        shuffleAriaLabel="Shuffle all 3 tracks"
        onPlay={onPlay}
        onShuffle={onShuffle}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Play all 3 tracks" }));
    await user.click(screen.getByRole("button", { name: "Shuffle all 3 tracks" }));

    expect(onPlay).toHaveBeenCalledTimes(1);
    expect(onShuffle).toHaveBeenCalledTimes(1);
  });

  it("falls back to the visible words when there is no fuller name", () => {
    render(
      <PlayShuffleButtons
        playLabel="Play Album"
        shuffleAriaLabel="Shuffle play album"
        onPlay={vi.fn()}
        onShuffle={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Play Album" }),
    ).toBeInTheDocument();
  });

  // A disabled media control is unreachable under iOS VoiceOver (§1.7).
  it("shows progress while loading but stays operable", async () => {
    const user = userEvent.setup();
    const onPlay = vi.fn();

    render(
      <PlayShuffleButtons
        playLabel="Play All"
        playAriaLabel="Play all 3 tracks"
        shuffleAriaLabel="Shuffle all 3 tracks"
        onPlay={onPlay}
        onShuffle={vi.fn()}
        isLoading
      />,
    );

    expect(screen.getAllByRole("status", { name: "Loading" })).toHaveLength(2);
    const play = screen.getByRole("button", { name: "Play all 3 tracks" });
    expect(play).toBeEnabled();
    await user.click(play);
    expect(onPlay).toHaveBeenCalledTimes(1);
  });
});
