import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import DraggableTrackList from "@/components/music/DraggableTrackList";
import { renderWithQueryClient } from "@/test/helpers/render";
import { layOutTrackRows, pressKey } from "@/test/helpers/drag";
import { playlistTrack } from "@/test/helpers/music";

const prefersCoarse = vi.hoisted(() => ({ value: false }));
vi.mock("@/hooks/use-coarse-pointer", () => ({
  usePrefersCoarsePointer: () => prefersCoarse.value,
}));

vi.mock("@/hooks/useTrackPlaybackMatcher", () => ({
  useTrackPlaybackMatcher: () => () => ({ isCurrentTrack: false, isPlaying: false }),
}));

vi.mock("@/lib/api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  getLikedTrackIds: vi.fn().mockResolvedValue({
    error: false,
    data: { liked_track_ids: [] },
  }),
}));

function renderList(
  tracks = [playlistTrack({ id: 1, title: "Alabaster", position: 1 }), playlistTrack({ id: 2, title: "Borrowed Light", position: 2 })],
) {
  return renderWithQueryClient(
    <DraggableTrackList
      tracks={tracks}
      canEdit
      onReorder={vi.fn()}
      onPlayTrack={vi.fn()}
      onRemoveTrack={vi.fn()}
    />,
  );
}

function handleDescription(handle: HTMLElement) {
  const id = handle.getAttribute("aria-describedby");
  return id ? document.getElementById(id)?.textContent ?? "" : "";
}

// dnd-kit's live region can be overwritten by the next announcement before a
// test looks, so record every message it shows.
function recordAnnouncements() {
  const liveRegion = document.querySelector("[aria-live]")!;
  const announced: string[] = [];
  const observer = new MutationObserver(() => {
    announced.push(liveRegion.textContent ?? "");
  });
  observer.observe(liveRegion, { childList: true, characterData: true, subtree: true });
  observers.push(observer);
  return announced;
}

const observers: MutationObserver[] = [];

const layOutRows = () => layOutTrackRows(["Alabaster", "Borrowed Light"]);

afterEach(() => {
  prefersCoarse.value = false;
  vi.restoreAllMocks();
  for (const observer of observers.splice(0)) observer.disconnect();
});

describe("DraggableTrackList screen reader text", () => {
  it("describes the keyboard reorder keys on a device with a keyboard", () => {
    renderList();

    const [handle] = screen.getAllByRole("button", { name: "Drag to reorder" });
    expect(handleDescription(handle)).toMatch(/press the space bar/i);
  });

  it("describes the hold-and-drag gesture on a touch-first device", () => {
    prefersCoarse.value = true;
    renderList();

    const [handle] = screen.getAllByRole("button", { name: "Drag to reorder" });
    const description = handleDescription(handle);
    expect(description).toBe("Touch and hold a track, then drag it to a new position.");
    expect(description).not.toMatch(/space|escape|arrow/i);
  });

  it("names the drop keys when a drag starts from the keyboard", async () => {
    renderList();

    const announced = recordAnnouncements();

    const [handle] = screen.getAllByRole("button", { name: "Drag to reorder" });
    handle.focus();
    fireEvent.keyDown(handle, { code: "Space", key: " " });

    await waitFor(() => {
      expect(announced).toContain(
        "Picked up Alabaster. Press space to drop, or escape to cancel.",
      );
    });
  });

  it("leaves the keys out when a pointer starts the drag", async () => {
    renderList();

    const announced = recordAnnouncements();

    const [handle] = screen.getAllByRole("button", { name: "Drag to reorder" });
    fireEvent.pointerDown(handle, { isPrimary: true, button: 0, clientX: 0, clientY: 0 });
    fireEvent.pointerMove(document, { clientX: 0, clientY: 20 });

    await waitFor(() => {
      expect(announced).toContain("Picked up Alabaster.");
    });
  });

  it("announces a keyboard drag returning to its own slot", async () => {
    layOutRows();
    renderList();
    const announced = recordAnnouncements();

    const [handle] = screen.getAllByRole("button", { name: "Drag to reorder" });
    handle.focus();
    pressKey(handle, "Space");
    await waitFor(() => expect(announced.at(-1)).toMatch(/^Picked up Alabaster/));
    pressKey(handle, "ArrowDown");
    await waitFor(() => expect(announced.at(-1)).toBe("Alabaster is over Borrowed Light"));
    pressKey(handle, "ArrowUp");

    await waitFor(() => {
      expect(announced.at(-1)).toBe("Alabaster is back in its original position");
    });
  });

  it("says a track moved up landed before the one it was dropped on", async () => {
    layOutRows();
    renderList();
    const announced = recordAnnouncements();

    const handle = screen.getAllByRole("button", { name: "Drag to reorder" })[1];
    handle.focus();
    pressKey(handle, "Space");
    await waitFor(() => expect(announced.at(-1)).toMatch(/^Picked up Borrowed Light/));
    pressKey(handle, "ArrowUp");
    await waitFor(() => expect(announced.at(-1)).toBe("Borrowed Light is over Alabaster"));
    pressKey(handle, "Space");

    await waitFor(() => {
      expect(announced.at(-1)).toBe("Borrowed Light was moved before Alabaster");
    });
  });
});
