import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import TrackActionsMenu from "@/components/music/TrackActionsMenu";
import TrackItem from "@/components/music/TrackItem";
import { getLikedTrackIds, toggleLikeTrack } from "@/lib/api";
import {
  LIKED_TRACK_IDS_KEY,
  MOTION_TRACK_ICON_BUTTON_CLASS,
  MOTION_TRACK_MENU_TRIGGER_CLASS,
  MOTION_TRACK_PLAY_BUTTON_CLASS,
  MOTION_TRACK_ROW_CLASS,
} from "@/lib/constants";
import { expectNoClass } from "@/test/helpers/dom";
import { renderWithQueryClient } from "@/test/helpers/render";

vi.mock("@/lib/api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  getLikedTrackIds: vi.fn().mockResolvedValue({
    error: false,
    data: { liked_track_ids: [] },
  }),
  toggleLikeTrack: vi.fn(),
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function likedIds(ids: number[]) {
  return { error: false as const, data: { liked_track_ids: ids } };
}

describe("TrackItem motion", () => {
  it("uses shared motion contracts for row controls", async () => {
    renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        trackIndex={1}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
        isDraggable
      />,
    );

    const row = screen.getByText("Signal Fire").closest(".group");
    expect(row).toHaveClass(...MOTION_TRACK_ROW_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Drag to reorder" })).toHaveClass(
      ...MOTION_TRACK_MENU_TRIGGER_CLASS.split(" "),
    );
    expect(await screen.findByRole("button", { name: "Add Signal Fire to liked" }))
      .toHaveClass(...MOTION_TRACK_ICON_BUTTON_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Play Signal Fire" })).toHaveClass(
      ...MOTION_TRACK_PLAY_BUTTON_CLASS.split(" "),
    );
  });

  it("uses the shared menu trigger contract", () => {
    renderWithQueryClient(
      <TrackActionsMenu trackId={7} trackTitle="Signal Fire" />,
    );

    expect(
      screen.getByRole("button", { name: "More actions for Signal Fire" }),
    ).toHaveClass(...MOTION_TRACK_MENU_TRIGGER_CLASS.split(" "));
  });
});

describe("TrackItem like button", () => {
  it("stays focusable while liked status loads", async () => {
    const status = deferred<Awaited<ReturnType<typeof getLikedTrackIds>>>();
    vi.mocked(getLikedTrackIds).mockReturnValue(status.promise);

    renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );

    const loadingButton = screen.getByRole("button", {
      name: "Loading liked status for Signal Fire",
    });
    expect(loadingButton).toHaveAttribute("aria-disabled", "true");
    expect(loadingButton).not.toBeDisabled();
    loadingButton.focus();
    expect(loadingButton).toHaveFocus();
    fireEvent.click(loadingButton);
    expect(toggleLikeTrack).not.toHaveBeenCalled();

    status.resolve(likedIds([]));
    const readyButton = await screen.findByRole("button", {
      name: "Add Signal Fire to liked",
    });
    expect(readyButton).not.toHaveAttribute("aria-disabled");
  });

  it("toggles the cached liked ids through the shared mutation", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    vi.mocked(toggleLikeTrack).mockResolvedValue({
      error: false,
      data: { track_id: 7, is_liked: true },
    });

    const { queryClient } = renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );
    const likeButton = await screen.findByRole("button", {
      name: "Add Signal Fire to liked",
    });
    expect(likeButton).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(likeButton);

    await waitFor(() => {
      expect(toggleLikeTrack).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual({
        error: false,
        data: { liked_track_ids: [7] },
      });
    });
  });

  it("rolls back only the failed track when another toggle succeeds", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    const firstToggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    const secondToggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    vi.mocked(toggleLikeTrack).mockImplementation(trackId =>
      trackId === 7 ? firstToggle.promise : secondToggle.promise,
    );

    const { queryClient } = renderWithQueryClient(
      <>
        <TrackItem
          id={7}
          title="Signal Fire"
          duration={211}
          variant="album"
          onPlay={vi.fn()}
          showActionsMenu={false}
        />
        <TrackItem
          id={8}
          title="Northern Sky"
          duration={226}
          variant="album"
          onPlay={vi.fn()}
          showActionsMenu={false}
        />
      </>,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Add Signal Fire to liked" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Add Northern Sky to liked" }),
    );

    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([7, 8]),
      );
    });

    secondToggle.resolve({
      error: false,
      data: { track_id: 8, is_liked: true },
    });
    await waitFor(() => {
      expect(toggleLikeTrack).toHaveBeenCalledTimes(2);
    });
    firstToggle.reject(new Error("network down"));

    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([8]),
      );
    });
  });

  it("refetches canonical ids when the cache is invalid at success", async () => {
    vi.mocked(getLikedTrackIds)
      .mockResolvedValueOnce(likedIds([]))
      .mockResolvedValueOnce(likedIds([7]));
    const toggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    vi.mocked(toggleLikeTrack).mockReturnValue(toggle.promise);

    const { queryClient } = renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Add Signal Fire to liked" }),
    );
    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([7]),
      );
    });

    queryClient.setQueryData([LIKED_TRACK_IDS_KEY], {
      error: true,
      message: "stale cache",
    });
    toggle.resolve({
      error: false,
      data: { track_id: 7, is_liked: true },
    });

    await waitFor(() => {
      expect(getLikedTrackIds).toHaveBeenCalledTimes(2);
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([7]),
      );
    });
  });
});

// Below `sm` the duration moves under the title; jsdom applies no CSS, so both
// copies render and a test must never reach for `getByText("m:ss")` alone.
describe("TrackItem duration", () => {
  it("renders the duration beside the subtitle for phones and at the right for wider screens", () => {
    renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        subtitle="The Band"
        duration={211_000}
        variant="library"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );

    const [inline, right] = screen.getAllByText("3:31");
    expect(screen.getAllByText("3:31")).toHaveLength(2);
    expect(inline.parentElement).toHaveClass("sm:hidden");
    expect(right).toHaveClass("hidden", "sm:inline");
    expect(screen.getByText("·")).toHaveAttribute("aria-hidden", "true");
    // The subtitle and the inline duration share one line.
    expect(screen.getByText("The Band").parentElement).toBe(
      inline.parentElement?.parentElement,
    );
  });

  it("gives an album row without genres a phone-only duration line", () => {
    renderWithQueryClient(
      <TrackItem
        id={8}
        title="Solo"
        duration={61_000}
        trackIndex={1}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );

    const [inline] = screen.getAllByText("1:01");
    expect(screen.getAllByText("1:01")).toHaveLength(2);
    expect(inline.closest("p")).toHaveClass("sm:hidden");
    expect(screen.queryByText("·")).not.toBeInTheDocument();
  });
});

describe("TrackItem cover", () => {
  const cover = (container: HTMLElement) =>
    container.querySelector('[data-slot="track-cover"]');

  function renderRow(
    props: Partial<React.ComponentProps<typeof TrackItem>> = {},
  ) {
    return renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        subtitle="The Band"
        duration={211_000}
        variant="library"
        onPlay={vi.fn()}
        showActionsMenu={false}
        {...props}
      />,
    );
  }

  it("draws the album cover at the start of the row, hidden from assistive tech", () => {
    const { container } = renderRow({ coverUrl: "/api/static/albums/9.jpg" });

    const tile = cover(container);
    expect(tile).toHaveAttribute("aria-hidden", "true");
    const image = tile?.querySelector("img");
    expect(image).toHaveAttribute("src", "/api/static/albums/9.jpg");
    expect(image).toHaveAttribute("alt", "");
    expect(image).toHaveAttribute("loading", "lazy");
    expect(image).toHaveAttribute("width", "40");
    expect(image).toHaveAttribute("height", "40");
    // It is the row's first child, before the title.
    expect(tile?.nextElementSibling).toContainElement(
      screen.getByText("Signal Fire"),
    );
  });

  // 40 px fits inside the row's two text lines, so the virtual lists' fixed
  // row height holds; below sm the title needs the room instead.
  it("is 40 px and gives way to the title below sm", () => {
    const { container } = renderRow({ coverUrl: "/api/static/albums/9.jpg" });

    expect(cover(container)).toHaveClass("size-10", "hidden", "sm:flex");
  });

  it("draws a placeholder tile when the track has no cover or the image fails", () => {
    const { container, unmount } = renderRow({ coverUrl: null });
    expect(cover(container)?.querySelector("img")).toBeNull();
    expect(cover(container)?.querySelector("svg")).not.toBeNull();
    unmount();

    const failed = renderRow({ coverUrl: "/api/static/albums/9.jpg" });
    fireEvent.error(cover(failed.container)!.querySelector("img")!);
    expect(cover(failed.container)?.querySelector("img")).toBeNull();
    expect(cover(failed.container)?.querySelector("svg")).not.toBeNull();
  });

  // The album page leaves coverUrl out: every row would repeat the cover
  // shown above the list.
  it("draws nothing when no cover is asked for", () => {
    const { container } = renderRow({ variant: "album", trackIndex: 1 });

    expect(cover(container)).toBeNull();
  });
});

describe("TrackItem play button", () => {
  const FILLED = ["bg-primary", "text-primary-foreground"] as const;

  function renderRow(
    props: Partial<React.ComponentProps<typeof TrackItem>> = {},
  ) {
    renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211_000}
        variant="library"
        onPlay={vi.fn()}
        showActionsMenu={false}
        {...props}
      />,
    );
    return screen.getByRole("button", { name: /^(Play|Pause) Signal Fire$/ });
  }

  // A list of filled buttons shouts. The button is always there, but it only
  // fills on row hover, on its own focus, and where nothing hovers.
  it.each(["library", "playlist", "musician", "album"] as const)(
    "is quiet on a %s row until hover, focus or a coarse pointer",
    variant => {
      const button = renderRow({ variant });

      expect(button).toHaveClass("text-muted-foreground");
      expectNoClass(button, ...FILLED);
      expect(button).toHaveClass(
        "group-hover:bg-primary",
        "group-hover:text-primary-foreground",
        "focus-visible:bg-primary",
        "focus-visible:text-primary-foreground",
        "pointer-coarse:bg-primary",
        "pointer-coarse:text-primary-foreground",
      );
      // Never hidden: no action sits behind hover alone.
      expectNoClass(button, "opacity-0", "sm:opacity-0", "hidden");
    },
  );

  it("stays filled on the current track, playing or paused", () => {
    const paused = renderRow({ isCurrentTrack: true });
    expect(paused).toHaveClass(...FILLED);
    expectNoClass(paused, "text-muted-foreground");
    expect(paused).toHaveAccessibleName("Play Signal Fire");
  });

  it("names the action for the playing track", () => {
    const playing = renderRow({ isCurrentTrack: true, isPlaying: true });

    expect(playing).toHaveClass(...FILLED);
    expect(playing).toHaveAccessibleName("Pause Signal Fire");
  });
});
