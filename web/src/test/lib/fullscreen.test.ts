import { afterEach, describe, expect, it, vi } from "vitest";
import {
  canRequestElementFullscreen,
  exitDocumentFullscreen,
  fullscreenToggleLabel,
  getFullscreenElement,
  isDocumentFullscreenEntryLikely,
  isWebKitVideoFullscreen,
  requestElementFullscreen,
  tryWebKitVideoEnterFullscreen,
  tryWebKitVideoExitFullscreen,
} from "@/lib/fullscreen";

// jsdom implements none of the Fullscreen API, so each case installs the
// standard or the WebKit-prefixed member it needs on document or an element.
function setDocumentMember(name: string, value: unknown) {
  Object.defineProperty(document, name, { configurable: true, value });
}

const installedDocumentMembers = [
  "fullscreenElement",
  "webkitFullscreenElement",
  "exitFullscreen",
  "webkitExitFullscreen",
  "fullscreenEnabled",
  "webkitFullscreenEnabled",
];

afterEach(() => {
  for (const name of installedDocumentMembers) {
    Reflect.deleteProperty(document, name);
  }
});

describe("getFullscreenElement", () => {
  it("prefers the standard element and falls back to WebKit's", () => {
    const standard = document.createElement("div");
    const webkit = document.createElement("div");

    expect(getFullscreenElement()).toBeNull();

    setDocumentMember("webkitFullscreenElement", webkit);
    expect(getFullscreenElement()).toBe(webkit);

    setDocumentMember("fullscreenElement", standard);
    expect(getFullscreenElement()).toBe(standard);
  });
});

describe("exitDocumentFullscreen", () => {
  it("resolves without an exit method to call", async () => {
    await expect(exitDocumentFullscreen()).resolves.toBeUndefined();
  });

  it("returns the standard exit's promise", async () => {
    const exit = vi.fn(() => Promise.reject(new Error("not in fullscreen")));
    setDocumentMember("exitFullscreen", exit);

    await expect(exitDocumentFullscreen()).rejects.toThrow("not in fullscreen");
    expect(exit).toHaveBeenCalledOnce();
  });

  it("calls WebKit's exit, which returns nothing, and resolves", async () => {
    const exit = vi.fn(() => undefined);
    setDocumentMember("webkitExitFullscreen", exit);

    await expect(exitDocumentFullscreen()).resolves.toBeUndefined();
    expect(exit).toHaveBeenCalledOnce();
  });

  it("resolves when the exit throws synchronously", async () => {
    setDocumentMember("exitFullscreen", () => {
      throw new Error("blocked");
    });

    await expect(exitDocumentFullscreen()).resolves.toBeUndefined();
  });
});

describe("requestElementFullscreen", () => {
  it("rejects when the element has no request method", async () => {
    const el = document.createElement("div");

    expect(canRequestElementFullscreen(el)).toBe(false);
    await expect(requestElementFullscreen(el)).rejects.toThrow(
      "Fullscreen API not available",
    );
  });

  it("returns the standard request's promise", async () => {
    const el = document.createElement("div");
    const request = vi.fn(() => Promise.resolve());
    Object.assign(el, { requestFullscreen: request });

    expect(canRequestElementFullscreen(el)).toBe(true);
    await expect(requestElementFullscreen(el)).resolves.toBeUndefined();
    expect(request).toHaveBeenCalledOnce();
  });

  it("calls WebKit's request on the element and resolves", async () => {
    const el = document.createElement("div");
    const request = vi.fn(function (this: HTMLElement) {
      expect(this).toBe(el);
    });
    Object.assign(el, { webkitRequestFullscreen: request });

    expect(canRequestElementFullscreen(el)).toBe(true);
    await expect(requestElementFullscreen(el)).resolves.toBeUndefined();
    expect(request).toHaveBeenCalledOnce();
  });

  it("rejects with the error a request throws", async () => {
    const el = document.createElement("div");
    Object.assign(el, {
      requestFullscreen: () => {
        throw new TypeError("not allowed");
      },
    });

    await expect(requestElementFullscreen(el)).rejects.toThrow(TypeError);
  });

  it("wraps a thrown non-Error in an Error", async () => {
    const el = document.createElement("div");
    Object.assign(el, {
      requestFullscreen: () => {
        throw "denied";
      },
    });

    await expect(requestElementFullscreen(el)).rejects.toThrow(
      "Fullscreen request failed",
    );
  });
});

describe("isDocumentFullscreenEntryLikely", () => {
  it("assumes fullscreen works until a flag says it does not", () => {
    expect(isDocumentFullscreenEntryLikely()).toBe(true);

    setDocumentMember("fullscreenEnabled", false);
    expect(isDocumentFullscreenEntryLikely()).toBe(false);
  });

  it("trusts either flag that says fullscreen is on", () => {
    setDocumentMember("fullscreenEnabled", false);
    setDocumentMember("webkitFullscreenEnabled", true);
    expect(isDocumentFullscreenEntryLikely()).toBe(true);

    setDocumentMember("fullscreenEnabled", true);
    setDocumentMember("webkitFullscreenEnabled", false);
    expect(isDocumentFullscreenEntryLikely()).toBe(true);
  });
});

describe("WebKit video fullscreen", () => {
  it("reports whether it could enter and exit", () => {
    const video = document.createElement("video");

    expect(tryWebKitVideoEnterFullscreen(video)).toBe(false);
    expect(tryWebKitVideoExitFullscreen(video)).toBe(false);

    const enter = vi.fn();
    const exit = vi.fn();
    Object.assign(video, {
      webkitEnterFullscreen: enter,
      webkitExitFullscreen: exit,
    });
    expect(tryWebKitVideoEnterFullscreen(video)).toBe(true);
    expect(tryWebKitVideoExitFullscreen(video)).toBe(true);
    expect(enter).toHaveBeenCalledOnce();
    expect(exit).toHaveBeenCalledOnce();
  });

  it("reports a throwing enter or exit as not done", () => {
    const video = document.createElement("video");
    const fail = () => {
      throw new Error("InvalidStateError");
    };
    Object.assign(video, { webkitEnterFullscreen: fail, webkitExitFullscreen: fail });

    expect(tryWebKitVideoEnterFullscreen(video)).toBe(false);
    expect(tryWebKitVideoExitFullscreen(video)).toBe(false);
  });

  it("reads the displaying flag", () => {
    const video = document.createElement("video");
    expect(isWebKitVideoFullscreen(video)).toBe(false);

    Object.assign(video, { webkitDisplayingFullscreen: true });
    expect(isWebKitVideoFullscreen(video)).toBe(true);
  });
});

describe("fullscreenToggleLabel", () => {
  it.each([
    [{ chromeFullscreenMode: false, isFullscreen: false, isImmersiveViewport: false }, "Fullscreen"],
    [{ chromeFullscreenMode: true, isFullscreen: true, isImmersiveViewport: false }, "Exit fullscreen"],
    [{ chromeFullscreenMode: true, isFullscreen: false, isImmersiveViewport: true }, "Exit expanded view"],
    // Real fullscreen wins over the expanded view it was entered from.
    [{ chromeFullscreenMode: true, isFullscreen: true, isImmersiveViewport: true }, "Exit fullscreen"],
  ])("names the toggle for %o", (state, label) => {
    expect(fullscreenToggleLabel(state)).toBe(label);
  });
});
