import { describe, expect, it, vi } from "vitest";
import {
  createPlaybackSessionId,
  stopHlsPlaybackSession,
} from "@/lib/video-playback";
import { episodeMediaRef, movieMediaRef } from "@/lib/media-ref";

const uuidPattern = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;

describe("HLS playback sessions", () => {
  it("mints a fresh UUID for every call", () => {
    const first = createPlaybackSessionId();
    const second = createPlaybackSessionId();

    expect(first).toMatch(uuidPattern);
    expect(second).toMatch(uuidPattern);
    expect(second).not.toBe(first);
  });

  // randomUUID exists only in a secure context; a LAN address over plain
  // HTTP has getRandomValues alone.
  it("mints a version 4 UUID without randomUUID", () => {
    // Shadows the prototype's method on the instance; deleting it restores it.
    Object.defineProperty(globalThis.crypto, "randomUUID", {
      configurable: true,
      value: undefined,
    });
    try {
      const id = createPlaybackSessionId();

      expect(id).toMatch(uuidPattern);
      expect(id[14]).toBe("4");
      expect(["8", "9", "a", "b"]).toContain(id[19]);
    } finally {
      delete (globalThis.crypto as { randomUUID?: unknown }).randomUUID;
    }
  });

  it("stops an episode session on the episode route", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);

    await stopHlsPlaybackSession(
      episodeMediaRef(9),
      "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/shows/episodes/9/hls/session/stop?playback_session=4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4",
      {
        method: "POST",
        credentials: "include",
        keepalive: false,
      },
    );
  });

  it("sends a credentialed keepalive stop request", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);

    await stopHlsPlaybackSession(
      movieMediaRef(6),
      "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4",
      { keepalive: true },
    );

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/movies/6/hls/session/stop?playback_session=4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4",
      {
        method: "POST",
        credentials: "include",
        keepalive: true,
      },
    );
  });

  it("does not send stop requests for malformed playback sessions", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);

    await stopHlsPlaybackSession(movieMediaRef(6), "bad-session", { keepalive: true });

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
