import { fireEvent, screen } from "@testing-library/react";
import { vi } from "vitest";

/**
 * jsdom lays nothing out, so dnd-kit's keyboard sensor has no slot to move
 * to. Stack the rows 50 px apart in `titles` order: an element holding one
 * drag handle is the row named in it. The drag overlay holds a handle too, so
 * it lands on the dragged row.
 */
export function layOutTrackRows(titles: string[]) {
  const original = Element.prototype.getBoundingClientRect;
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
    this: Element,
  ) {
    const handles = screen
      .queryAllByRole("button", { name: "Drag to reorder" })
      .filter(handle => this === handle || this.contains(handle));
    if (handles.length !== 1) return original.call(this);
    let row: Element | null = handles[0];
    while (row && !titles.some(title => row!.textContent?.includes(title))) {
      row = row.parentElement;
    }
    const index = titles.findIndex(title => row?.textContent?.includes(title));
    if (index === -1) return original.call(this);
    return DOMRect.fromRect({ x: 0, y: index * 50, width: 400, height: 50 });
  });
}

/** One key press for dnd-kit's keyboard sensor. */
export function pressKey(target: HTMLElement, code: string) {
  fireEvent.keyDown(target, { code, key: code === "Space" ? " " : code });
}
