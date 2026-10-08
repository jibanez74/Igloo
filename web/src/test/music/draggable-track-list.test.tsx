import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import DraggableTrackList from "@/components/music/DraggableTrackList";
import { renderWithQueryClient } from "@/test/helpers/render";
import { nullableInt64, nullableString } from "@/test/helpers/fixtures";
import type { PlaylistTrackType } from "@/types";

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

function playlistTrack(id: number, title: string, position: number): PlaylistTrackType {
  return {
    playlist_track_id: 100 + id,
    position,
    added_at: "2026-01-01T00:00:00Z",
    added_by: nullableInt64(),
    id,
    title,
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    album_id: nullableInt64(),
    musician_id: nullableInt64(),
    album_title: nullableString(),
    album_cover: nullableString(),
    musician_name: nullableString(),
  };
}

function renderList(
  tracks = [playlistTrack(1, "Alabaster", 1), playlistTrack(2, "Borrowed Light", 2)],
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

// jsdom lays nothing out, so the keyboard sensor has no slot to move to. Stack
// the rows 50 px apart: an element holding one drag handle is the row named in
// it. The drag overlay holds a handle too, so it lands on the dragged row.
const TITLES = ["Alabaster", "Borrowed Light"];

function layOutRows() {
  const original = Element.prototype.getBoundingClientRect;
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
    this: Element,
  ) {
    const handles = screen
      .queryAllByRole("button", { name: "Drag to reorder" })
      .filter(handle => this === handle || this.contains(handle));
    if (handles.length !== 1) return original.call(this);
    let row: Element | null = handles[0];
    while (row && !TITLES.some(title => row!.textContent?.includes(title))) {
      row = row.parentElement;
    }
    const index = TITLES.findIndex(title => row?.textContent?.includes(title));
    if (index === -1) return original.call(this);
    return DOMRect.fromRect({ x: 0, y: index * 50, width: 400, height: 50 });
  });
}

function pressKey(target: HTMLElement, code: string) {
  fireEvent.keyDown(target, { code, key: code === "Space" ? " " : code });
}

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
