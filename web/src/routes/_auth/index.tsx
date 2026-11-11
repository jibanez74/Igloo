import { createFileRoute } from "@tanstack/react-router";
import {
  continueWatchingQueryOpts,
  latestMoviesQueryOpts,
  latestShowsQueryOpts,
  latestAlbumsQueryOpts,
  inTheatersQueryOpts,
  watchRoomsQueryOpts,
} from "@/lib/query-opts";
import ContinueWatching from "@/components/home/ContinueWatching";
import HomeBillboard from "@/components/home/HomeBillboard";
import LatestAlbums from "@/components/home/LatestAlbums";
import LatestMovies from "@/components/home/LatestMovies";
import LatestShows from "@/components/home/LatestShows";
import MoviesInTheaters from "@/components/home/MoviesInTheaters";
import WatchRooms from "@/components/watch-room/WatchRooms";
import { routeHead } from "@/lib/route-head";

const HOME_HEAD = routeHead(
  "Home",
  "Welcome to Igloo - explore your personal media library with recently added movies, TV shows, music, and more.",
);

export const Route = createFileRoute("/_auth/")({
  head: () => HOME_HEAD,
  loader: async ({ context }) => {
    const { queryClient } = context;

    await Promise.all([
      queryClient.ensureQueryData(watchRoomsQueryOpts()),
      queryClient.ensureQueryData(continueWatchingQueryOpts()),
      queryClient.ensureQueryData(latestMoviesQueryOpts()),
      queryClient.ensureQueryData(latestShowsQueryOpts()),
      queryClient.ensureQueryData(latestAlbumsQueryOpts()),
      queryClient.ensureQueryData(inTheatersQueryOpts()),
    ]);
  },
  component: HomePage,
});

function HomePage() {
  return (
    <div className="min-w-0">
      {/* The page's one h1. The billboard below leads with a title's own
          name, which is not the page's. */}
      <h1 className="sr-only">Home</h1>

      <HomeBillboard />

      <WatchRooms />
      <ContinueWatching />
      <LatestMovies />
      <LatestShows />
      <LatestAlbums />
      <MoviesInTheaters />
    </div>
  );
}
