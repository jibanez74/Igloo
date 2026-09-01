import { describe, expect, it, vi } from "vitest";
import {
  getOrCreateMovieHlsPlaybackSessionId,
  stopMovieHlsPlaybackSession,
} from "@/lib/movie-playback";

type MemoryStorage = Pick<Storage, "getItem" | "setItem"> & {
  entries: () => [string, string][];
};

const uuidPattern = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;

function createMemoryStorage(initial: Record<string, string> = {}): MemoryStorage {
  const values = new Map(Object.entries(initial));
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value);
    },
    entries: () => Array.from(values.entries()),
  };
}

describe("movie HLS playback sessions", () => {
  it("reuses the same stored UUID for the same movie", () => {
    const storage = createMemoryStorage();

    const first = getOrCreateMovieHlsPlaybackSessionId(6, storage);
    const second = getOrCreateMovieHlsPlaybackSessionId(6, storage);

    expect(first).toMatch(uuidPattern);
    expect(second).toBe(first);
  });

  it("stores different sessions for different movies", () => {
    const storage = createMemoryStorage();

    const first = getOrCreateMovieHlsPlaybackSessionId(6, storage);
    const second = getOrCreateMovieHlsPlaybackSessionId(7, storage);

    expect(second).toMatch(uuidPattern);
    expect(second).not.toBe(first);
    expect(storage.entries()).toHaveLength(2);
    expect(storage.entries().map(([, value]) => value)).toEqual([
      first,
      second,
    ]);
  });

  it("replaces malformed stored values", () => {
    const storage = createMemoryStorage();
    getOrCreateMovieHlsPlaybackSessionId(6, storage);
    const [[key]] = storage.entries();
    storage.setItem(key, "bad-session");

    const next = getOrCreateMovieHlsPlaybackSessionId(6, storage);

    expect(next).toMatch(uuidPattern);
    expect(next).not.toBe("bad-session");
    expect(storage.getItem(key)).toBe(next);
  });

  it("falls back when storage is unavailable", () => {
    expect(getOrCreateMovieHlsPlaybackSessionId(6, null)).toMatch(uuidPattern);
  });

  it("sends a credentialed keepalive stop request", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);

    await stopMovieHlsPlaybackSession(
      6,
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

    await stopMovieHlsPlaybackSession(6, "bad-session", { keepalive: true });

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
