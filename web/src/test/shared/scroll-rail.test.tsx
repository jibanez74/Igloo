import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import ScrollRail from "@/components/shared/ScrollRail";
import {
  SCROLL_RAIL_BLEED_CLASS,
  SCROLL_RAIL_SCROLLER_CLASS,
} from "@/lib/constants";
import { restoreMatchMedia, setReducedMotionPreference } from "../helpers/dom";

const prefersCoarse = vi.hoisted(() => ({ value: false }));
vi.mock("@/hooks/use-coarse-pointer", () => ({
  usePrefersCoarsePointer: () => prefersCoarse.value,
}));

type Metrics = { scrollWidth: number; clientWidth: number; scrollLeft: number };

// jsdom lays nothing out, so the scroller's geometry is declared by hand and
// a scroll event asks the rail to measure again.
function setMetrics(el: Element, metrics: Partial<Metrics>) {
  for (const [key, value] of Object.entries(metrics)) {
    Object.defineProperty(el, key, { configurable: true, writable: true, value });
  }
  fireEvent.scroll(el);
}

const scrollBy = vi.fn();

beforeEach(() => {
  Object.defineProperty(Element.prototype, "scrollBy", {
    configurable: true,
    writable: true,
    value: scrollBy,
  });
});

afterEach(() => {
  prefersCoarse.value = false;
  restoreMatchMedia();
  delete (Element.prototype as { scrollBy?: unknown }).scrollBy;
});

function renderRail() {
  render(
    <ScrollRail label="cast" asChild>
      <ul role="list" aria-label="Cast members" tabIndex={0}>
        <li>One</li>
        <li>Two</li>
      </ul>
    </ScrollRail>,
  );
  return screen.getByRole("list", { name: "Cast members" });
}

const edge = (side: "start" | "end") =>
  document.querySelector(`[data-rail-edge="${side}"]`);

describe("ScrollRail", () => {
  it("keeps the child as the scroller and puts the bleed on the wrapper", () => {
    const list = renderRail();

    expect(list).toHaveAttribute("tabindex", "0");
    expect(list).toHaveClass(...SCROLL_RAIL_SCROLLER_CLASS.split(" "));
    expect(list.parentElement).toHaveClass(...SCROLL_RAIL_BLEED_CLASS.split(" "));
  });

  it("shows no edges or arrows while everything fits", () => {
    const list = renderRail();
    setMetrics(list, { scrollWidth: 400, clientWidth: 400, scrollLeft: 0 });

    expect(edge("start")).toBeNull();
    expect(edge("end")).toBeNull();
    expect(screen.queryByRole("button", { name: /Scroll cast/ })).toBeNull();
  });

  it("fades only the side that overflows, and keeps the other arrow inert rather than gone", () => {
    const list = renderRail();
    setMetrics(list, { scrollWidth: 1200, clientWidth: 400, scrollLeft: 0 });

    expect(edge("end")).toHaveAttribute("data-overflow", "true");
    expect(edge("end")).toHaveClass("bg-linear-to-l");
    expect(edge("start")).not.toHaveAttribute("data-overflow");
    expect(edge("start")).not.toHaveClass("bg-linear-to-r");

    const left = screen.getByRole("button", { name: "Scroll cast left" });
    const right = screen.getByRole("button", { name: "Scroll cast right" });
    expect(left).toHaveAttribute("aria-disabled", "true");
    expect(left).not.toBeDisabled();
    expect(right).not.toHaveAttribute("aria-disabled");

    setMetrics(list, { scrollLeft: 800 });
    expect(edge("start")).toHaveAttribute("data-overflow", "true");
    expect(edge("end")).not.toHaveAttribute("data-overflow");
    expect(right).toHaveAttribute("aria-disabled", "true");
    expect(left).not.toHaveAttribute("aria-disabled");
  });

  it("pages by most of the visible width, smoothly unless motion is reduced", async () => {
    const user = userEvent.setup();
    const list = renderRail();
    setMetrics(list, { scrollWidth: 1200, clientWidth: 400, scrollLeft: 100 });

    await user.click(screen.getByRole("button", { name: "Scroll cast right" }));
    expect(scrollBy).toHaveBeenLastCalledWith({ left: 320, behavior: "smooth" });

    await user.click(screen.getByRole("button", { name: "Scroll cast left" }));
    expect(scrollBy).toHaveBeenLastCalledWith({ left: -320, behavior: "smooth" });

    setReducedMotionPreference(true);
    await user.click(screen.getByRole("button", { name: "Scroll cast right" }));
    expect(scrollBy).toHaveBeenLastCalledWith({ left: 320, behavior: "auto" });
  });

  it("ignores a press on the exhausted side", async () => {
    const user = userEvent.setup();
    const list = renderRail();
    setMetrics(list, { scrollWidth: 1200, clientWidth: 400, scrollLeft: 0 });

    await user.click(screen.getByRole("button", { name: "Scroll cast left" }));
    expect(scrollBy).not.toHaveBeenCalled();
  });

  it("re-measures when the window resizes", () => {
    const list = renderRail();
    setMetrics(list, { scrollWidth: 400, clientWidth: 400, scrollLeft: 0 });
    expect(edge("end")).toBeNull();

    Object.defineProperty(list, "scrollWidth", { configurable: true, value: 900 });
    act(() => {
      window.dispatchEvent(new Event("resize"));
    });

    expect(edge("end")).toHaveAttribute("data-overflow", "true");
  });

  it("re-measures when items are added after mount", async () => {
    const rail = (items: string[]) => (
      <ScrollRail label="cast" asChild>
        <ul role="list" aria-label="Cast members">
          {items.map(item => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </ScrollRail>
    );
    const { rerender } = render(rail(["One", "Two"]));
    const list = screen.getByRole("list", { name: "Cast members" });
    setMetrics(list, { scrollWidth: 400, clientWidth: 400, scrollLeft: 0 });
    expect(edge("end")).toBeNull();

    // No scroll and no window resize: only the new item can trigger this.
    Object.defineProperty(list, "scrollWidth", { configurable: true, value: 900 });
    rerender(rail(["One", "Two", "Three"]));

    await waitFor(() => {
      expect(edge("end")).toHaveAttribute("data-overflow", "true");
    });
  });

  it("keeps the fades but drops the arrows on a touch-first device", () => {
    prefersCoarse.value = true;
    const list = renderRail();
    setMetrics(list, { scrollWidth: 1200, clientWidth: 400, scrollLeft: 0 });

    expect(edge("end")).toHaveClass("bg-linear-to-l");
    expect(screen.queryByRole("button", { name: /Scroll cast/ })).toBeNull();
  });
});
