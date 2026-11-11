import { createMemoryHistory, createRouter } from "@tanstack/react-router";
import { describe, expect, it } from "vitest";
import { routeTree } from "@/routeTree.gen";
import { createTestQueryClient } from "../helpers/render";

// The players and the watch room take the whole window; every other page
// keeps the sidebar and header. A new player route has to be added here on
// purpose, and a page route marked by mistake fails this.
describe("immersive routes", () => {
  it("are exactly the four that play video", () => {
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/login"] }),
      context: { queryClient: createTestQueryClient() },
    });

    const immersive = Object.values(router.routesById)
      .filter(route => route.options.staticData?.immersive === true)
      .map(route => route.id)
      .sort();

    expect(immersive).toEqual([
      "/_auth/movies/$id/play",
      "/_auth/trailer",
      "/_auth/tv-shows/$id/episodes/$episodeId/play",
      "/_auth/watch-rooms/$id",
    ]);
  });
});
