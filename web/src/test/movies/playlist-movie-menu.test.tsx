import { useRef, useState } from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import PlaylistMovieMenu from "@/components/movies/PlaylistMovieMenu";
import { renderWithQueryClient } from "../helpers/render";

// A list that drops the card as soon as Remove is chosen: the menu is still
// closing when its trigger leaves, the order a fast removal produces.
function VanishingCard({ onRemove }: { onRemove: () => void }) {
  const headingRef = useRef<HTMLHeadingElement | null>(null);
  const [removed, setRemoved] = useState(false);

  return (
    <>
      <h1 ref={headingRef} tabIndex={-1}>
        Weekend Picks
      </h1>
      {!removed && (
        <PlaylistMovieMenu
          movieTitle="Heat"
          onRemove={() => {
            onRemove();
            setRemoved(true);
          }}
          fallbackFocusRef={headingRef}
        />
      )}
    </>
  );
}

describe("PlaylistMovieMenu", () => {
  it("names its trigger after the movie and offers Remove from Playlist", async () => {
    const onRemove = vi.fn();
    const user = userEvent.setup();
    renderWithQueryClient(<PlaylistMovieMenu movieTitle="Heat" onRemove={onRemove} />);

    await user.click(screen.getByRole("button", { name: "More actions for Heat" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Remove from Playlist" }),
    );

    expect(onRemove).toHaveBeenCalledOnce();
  });

  it("falls back to the given element when its trigger has gone by the time it closes", async () => {
    const onRemove = vi.fn();
    const user = userEvent.setup();
    renderWithQueryClient(<VanishingCard onRemove={onRemove} />);

    await user.click(screen.getByRole("button", { name: "More actions for Heat" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Remove from Playlist" }),
    );

    expect(onRemove).toHaveBeenCalledOnce();
    expect(screen.queryByRole("button", { name: "More actions for Heat" })).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Weekend Picks" })).toHaveFocus();
    });
  });
});
