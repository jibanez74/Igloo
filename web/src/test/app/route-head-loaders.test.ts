import { describe, expect, it } from "vitest";
import {
  movieDetailsQueryOpts,
  playlistDetailsQueryOpts,
  watchRoomQueryOpts,
} from "@/lib/query-opts";
import { Route as MovieDetailsRoute } from "@/routes/_auth/movies/$id/index";
import { Route as PlayMovieRoute } from "@/routes/_auth/movies/$id/play";
import { Route as InTheatersRoute } from "@/routes/_auth/movies/in-theaters.$id";
import { Route as MoviePlaylistRoute } from "@/routes/_auth/movies/playlist.$id";
import { Route as AlbumRoute } from "@/routes/_auth/music/album.$id";
import { Route as MusicianRoute } from "@/routes/_auth/music/musician.$id";
import { Route as MusicPlaylistRoute } from "@/routes/_auth/music/playlist.$id";
import { Route as TrailerRoute } from "@/routes/_auth/trailer";
import { Route as ShowDetailsRoute } from "@/routes/_auth/tv-shows/$id/index";
import { Route as PlayEpisodeRoute } from "@/routes/_auth/tv-shows/$id/episodes/$episodeId/play";
import { Route as WatchRoomRoute } from "@/routes/_auth/watch-rooms/$id";
import { createTestQueryClient } from "../helpers/render";

// Route loaders may be a function or a config object; these routes all use
// the function form, which is what the test calls.
type RouteWithHead = {
  options: {
    loader?: unknown;
    head?: (ctx: never) => unknown;
  };
};

function runLoader(route: RouteWithHead, ctx: Record<string, unknown>) {
  const loader = route.options.loader as (ctx: never) => unknown;

  return loader(ctx as never);
}

// A route's head runs even when its loader never produced data (a failed or
// redirected load), and a throw there is only logged, so each dynamic head has
// to fall back to a generic title on its own.
const DYNAMIC_ROUTES: [string, RouteWithHead, string][] = [
  ["library movie", MovieDetailsRoute, "Movie - Igloo"],
  ["in-theaters movie", InTheatersRoute, "Movie - Igloo"],
  ["movie playlist", MoviePlaylistRoute, "Movie Playlist - Igloo"],
  ["movie player", PlayMovieRoute, "Playing Movie - Igloo"],
  ["show", ShowDetailsRoute, "TV Show - Igloo"],
  ["episode player", PlayEpisodeRoute, "Playing Episode - Igloo"],
  ["album", AlbumRoute, "Album - Igloo"],
  ["musician", MusicianRoute, "Musician - Igloo"],
  ["music playlist", MusicPlaylistRoute, "Playlist - Igloo"],
  ["watch room", WatchRoomRoute, "Watch Room - Igloo"],
  ["trailer", TrailerRoute, "Trailer - Igloo"],
];

describe("dynamic route heads", () => {
  it.each(DYNAMIC_ROUTES)(
    "titles the %s route generically without loader data",
    (_name, route, title) => {
      const head = route.options.head?.({ loaderData: undefined } as never);

      expect(head).toEqual({ meta: [{ title }] });
    },
  );
});

const FAILURE = { error: true, message: "Not found" } as const;

// Runs a route's loader against a query cache seeded with `seed`, then hands
// the result to its head, exactly as the router does after a load. Seeded data
// satisfies ensureQueryData, so nothing reaches the network.
async function headAfterLoad(
  route: RouteWithHead,
  seed: [queryKey: readonly unknown[], data: unknown][],
  loaderContext: Record<string, unknown>,
) {
  const queryClient = createTestQueryClient();
  for (const [queryKey, data] of seed) {
    queryClient.setQueryData(queryKey, data);
  }

  const loaderData = await runLoader(route, {
    context: { queryClient },
    ...loaderContext,
  });

  return route.options.head?.({ loaderData } as never);
}

describe("route loaders feed their head", () => {
  const playlistKey = playlistDetailsQueryOpts(7).queryKey;
  const playlist = {
    error: false,
    data: {
      playlist: { id: 7, name: "Road Trip" },
      track_count: 12,
      duration: 2_700_000,
    },
  };

  it("titles and describes a music playlist from its details", async () => {
    expect(
      await headAfterLoad(MusicPlaylistRoute, [[playlistKey, playlist]], {
        params: { id: "7" },
      }),
    ).toEqual({
      meta: [
        { title: "Road Trip - Igloo" },
        {
          name: "description",
          content:
            "Listen to Road Trip - 12 tracks, 45m 0s in your Igloo playlist.",
        },
      ],
    });
  });

  it("titles a music playlist generically when its details failed", async () => {
    expect(
      await headAfterLoad(MusicPlaylistRoute, [[playlistKey, FAILURE]], {
        params: { id: "7" },
      }),
    ).toEqual({ meta: [{ title: "Playlist - Igloo" }] });
  });

  const roomKey = watchRoomQueryOpts(5).queryKey;

  it("titles a watch room after its movie", async () => {
    expect(
      await headAfterLoad(
        WatchRoomRoute,
        [[roomKey, { error: false, data: { room: { movie_title: "Heat" } } }]],
        { params: { id: 5 } },
      ),
    ).toEqual({
      meta: [
        { title: "Heat Watch Room - Igloo" },
        {
          name: "description",
          content: "Watch Heat together in a shared synchronized room.",
        },
      ],
    });
  });

  it("titles a watch room generically when the room failed to load", async () => {
    expect(
      await headAfterLoad(WatchRoomRoute, [[roomKey, FAILURE]], {
        params: { id: 5 },
      }),
    ).toEqual({ meta: [{ title: "Watch Room - Igloo" }] });
  });

  const tmdbKey = movieDetailsQueryOpts(550).queryKey;
  const fightClub = { error: false, data: { movie: { title: "Fight Club" } } };

  it("titles a movie trailer after the movie", async () => {
    expect(
      await headAfterLoad(TrailerRoute, [[tmdbKey, fightClub]], {
        deps: { mediaType: "movie", mediaId: 550, videoKey: undefined },
      }),
    ).toEqual({ meta: [{ title: "Fight Club - Trailer - Igloo" }] });
  });

  it("titles only a movie's trailer, as the page does", async () => {
    expect(
      await headAfterLoad(TrailerRoute, [[tmdbKey, fightClub]], {
        deps: { mediaType: "show", mediaId: 550, videoKey: undefined },
      }),
    ).toEqual({ meta: [{ title: "Trailer - Igloo" }] });
  });

  it("leaves a direct video key untitled without loading a movie", async () => {
    const queryClient = createTestQueryClient();
    const loaderData = await runLoader(TrailerRoute, {
      context: { queryClient },
      deps: { mediaType: "movie", mediaId: 550, videoKey: "abc123" },
    });

    expect(loaderData).toEqual({ movieTitle: null });
    expect(queryClient.getQueryCache().getAll()).toHaveLength(0);
  });
});
