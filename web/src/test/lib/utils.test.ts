import { afterEach, describe, expect, it, vi } from "vitest";
import { clearTimeoutRef, isEditableTarget } from "@/lib/utils";

describe("clearTimeoutRef", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("cancels the pending timer and empties the ref", () => {
    vi.useFakeTimers();
    const callback = vi.fn();
    const ref = { current: window.setTimeout(callback, 100) as number | null };

    clearTimeoutRef(ref);
    vi.advanceTimersByTime(100);

    expect(callback).not.toHaveBeenCalled();
    expect(ref.current).toBeNull();
  });

  it("clears a timer whose id is 0", () => {
    const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
    const ref = { current: 0 as number | null };

    clearTimeoutRef(ref);

    expect(clearTimeoutSpy).toHaveBeenCalledWith(0);
    expect(ref.current).toBeNull();
    clearTimeoutSpy.mockRestore();
  });

  it("does nothing for an empty ref", () => {
    const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");

    clearTimeoutRef({ current: null });

    expect(clearTimeoutSpy).not.toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });
});

describe("isEditableTarget", () => {
  it("treats text fields, selects and editable content as editable", () => {
    // jsdom does not implement isContentEditable, so stand in for a browser.
    const editable = document.createElement("div");
    Object.defineProperty(editable, "isContentEditable", { value: true });

    expect(isEditableTarget(document.createElement("input"))).toBe(true);
    expect(isEditableTarget(document.createElement("textarea"))).toBe(true);
    expect(isEditableTarget(document.createElement("select"))).toBe(true);
    expect(isEditableTarget(editable)).toBe(true);
  });

  it("leaves buttons and plain elements to the shortcuts", () => {
    expect(isEditableTarget(document.createElement("button"))).toBe(false);
    expect(isEditableTarget(document.body)).toBe(false);
  });
});
