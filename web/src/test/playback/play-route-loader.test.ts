import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadPlayRoute } from "@/lib/play-route-loader";
import { movieMediaRef } from "@/lib/media-ref";
import { createTestQueryClient } from "../helpers/render";

const preloadHlsLight = vi.hoisted(() => vi.fn());
vi.mock("@/lib/hls-loader", () => ({ preloadHlsLight }));

beforeEach(() => {
  preloadHlsLight.mockClear();
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify({ error: true, message: "offline" }), { status: 503 })),
  );
});

describe("loadPlayRoute hls.js warm-up", () => {
  // The player imports hls.js only once its stream is known; started here,
  // the chunk downloads alongside the queries the player waits for.
  it("starts loading hls.js for a streamed mode", async () => {
    await loadPlayRoute({
      queryClient: createTestQueryClient(),
      media: movieMediaRef(7),
      ensureDetails: async () => undefined,
      deps: { mode: "remux", start: 0 },
    });

    expect(preloadHlsLight).toHaveBeenCalledOnce();
  });

  it("leaves it alone for direct play", async () => {
    await loadPlayRoute({
      queryClient: createTestQueryClient(),
      media: movieMediaRef(7),
      ensureDetails: async () => undefined,
      deps: { mode: "direct", start: 0 },
    });

    expect(preloadHlsLight).not.toHaveBeenCalled();
  });
});
