import { afterEach, describe, expect, it, vi } from "vitest";
import { loadOnEntry } from "@/lib/route-loads";
import { showDetailsQueryOpts } from "@/lib/query-opts";
import { Route as MoviesRoute } from "@/routes/_auth/movies/index";
import { Route as MusicRoute } from "@/routes/_auth/music/index";
import { Route as SearchRoute } from "@/routes/_auth/search/index";
import { Route as ShowDetailsRoute } from "@/routes/_auth/tv-shows/$id/index";
import { Route as TvShowsRoute } from "@/routes/_auth/tv-shows/index";
import { createTestQueryClient } from "../helpers/render";
import { SHOW_ID, showDetails } from "../helpers/show-details";

const NEVER = new Promise<never>(() => {});

// Resolves to whether `promise` settled within a few event-loop turns, which
// is all an unawaited load needs: nothing here touches a timer.
async function settles(promise: Promise<unknown>) {
  const pending = Symbol("pending");
  const winner = await Promise.race([
    promise,
    new Promise(resolve => setTimeout(() => resolve(pending), 20)),
  ]);

  return winner !== pending;
}

describe("loadOnEntry", () => {
  it.each(["enter", "preload"] as const)("waits for every load on %s", async cause => {
    expect(await settles(loadOnEntry(cause, [Promise.resolve(), NEVER]))).toBe(false);
  });

  it("starts the loads without waiting when the route stays", async () => {
    expect(await settles(loadOnEntry("stay", [NEVER]))).toBe(true);
  });

  it("swallows a failed in-page load, which the page's query reports", async () => {
    const unhandled = vi.fn();
    process.on("unhandledRejection", unhandled);

    try {
      await loadOnEntry("stay", [Promise.reject(new Error("offline"))]);
      await new Promise(resolve => setTimeout(resolve, 0));

      expect(unhandled).not.toHaveBeenCalled();
    } finally {
      process.off("unhandledRejection", unhandled);
    }
  });
});

type RouteWithLoader = { options: { loader?: unknown } };

function runLoader(route: RouteWithLoader, ctx: Record<string, unknown>) {
  const loader = route.options.loader as (ctx: never) => Promise<unknown>;

  return loader(ctx as never);
}

// Each fetch hangs, so a loader settles only if it does not wait for one.
function stubHangingFetch() {
  vi.stubGlobal("fetch", vi.fn(() => NEVER));
}

describe("in-page navigations do not wait for the page's queries", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // Every index and search dep, so the loaders pick the branch that loads the
  // most: a genre, the liked view, a category tab.
  const INDEX_LOADS: [string, RouteWithLoader, Record<string, unknown>][] = [
    [
      "movies",
      MoviesRoute,
      {
        allPage: 2,
        sort: "title",
        tab: "playlists",
        genreId: 0,
        genresPage: 1,
        view: "liked",
        playlistsPage: 1,
      },
    ],
    [
      "TV shows",
      TvShowsRoute,
      { allPage: 1, sort: "title", tab: "genres", genreId: 3, genresPage: 2 },
    ],
    ["music", MusicRoute, { albumsPage: 2, musiciansPage: 1 }],
    ["search, all tab", SearchRoute, { q: "frost", tab: "all", page: 1 }],
    ["search, a category", SearchRoute, { q: "frost", tab: "movies", page: 9 }],
  ];

  it.each(INDEX_LOADS)("the %s loader settles at once on stay", async (_name, route, deps) => {
    stubHangingFetch();
    const queryClient = createTestQueryClient();

    expect(await settles(runLoader(route, { context: { queryClient }, cause: "stay", deps }))).toBe(true);
  });

  it.each(INDEX_LOADS)("the %s loader waits when the page is entered", async (_name, route, deps) => {
    stubHangingFetch();
    const queryClient = createTestQueryClient();

    expect(await settles(runLoader(route, { context: { queryClient }, cause: "enter", deps }))).toBe(false);
  });

  // The show's details stay awaited (its head and the season redirect read
  // them); only the selected season's episodes are left to the episode list.
  function showLoad(cause: "enter" | "stay") {
    stubHangingFetch();
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(showDetailsQueryOpts(SHOW_ID).queryKey, {
      error: false,
      data: showDetails(),
    });

    return runLoader(ShowDetailsRoute, {
      context: { queryClient },
      cause,
      params: { id: String(SHOW_ID) },
      deps: { season: 2 },
    });
  }

  it("a season switch returns the show's head data without its episodes", async () => {
    const load = showLoad("stay");

    expect(await settles(load)).toBe(true);
    await expect(load).resolves.toEqual({
      show: {
        title: "Frost Harbor",
        year: 2024,
        overview: "A harbor freezes and a town changes with it.",
      },
    });
  });

  it("entering a show waits for the selected season's episodes", async () => {
    expect(await settles(showLoad("enter"))).toBe(false);
  });
});
