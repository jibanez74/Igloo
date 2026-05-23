import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getOrCreateMovieHlsPlaybackSessionId,
  movieHlsPlaybackSessionStorageKey,
  stopMovieHlsPlaybackSession,
} from "@/lib/movie-playback";

type MemoryStorage = Pick<Storage, "getItem" | "setItem">;

const uuidPattern = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;

function createMemoryStorage(initial: Record<string, string> = {}): MemoryStorage {
  const values = new Map(Object.entries(initial));
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value);
    },
  };
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("movie HLS playback sessions", () => {
  it("reuses the same stored UUID for the same movie", () => {
    const storage = createMemoryStorage();

    const first = getOrCreateMovieHlsPlaybackSessionId(6, storage);
    const second = getOrCreateMovieHlsPlaybackSessionId(6, storage);

    expect(first).toMatch(uuidPattern);
    expect(second).toBe(first);
  });

  it("uses different storage keys for different movies", () => {
    expect(movieHlsPlaybackSessionStorageKey(6)).not.toBe(
      movieHlsPlaybackSessionStorageKey(7),
    );
  });

  it("replaces malformed stored values", () => {
    const key = movieHlsPlaybackSessionStorageKey(6);
    const storage = createMemoryStorage({ [key]: "bad-session" });

    const next = getOrCreateMovieHlsPlaybackSessionId(6, storage);

    expect(next).toMatch(uuidPattern);
    expect(next).not.toBe("bad-session");
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
