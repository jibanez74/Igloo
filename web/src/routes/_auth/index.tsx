import { createFileRoute } from "@tanstack/react-router";
import {
  latestMoviesQueryOpts,
  latestAlbumsQueryOpts,
  inTheatersQueryOpts,
  watchRoomsQueryOpts,
} from "@/lib/query-opts";
import { Home } from "lucide-react";
import LatestAlbums from "@/components/LatestAlbums";
import LatestMovies from "@/components/LatestMovies";
import MoviesInTheaters from "@/components/MoviesInTheaters";
import WatchRooms from "@/components/WatchRooms";
import { MOTION_SECTION_ENTER_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

const pageTitle = "Home - Igloo";
const pageDescription =
  "Welcome to Igloo - explore your personal media library with recently added movies, TV shows, music, and more.";

export const Route = createFileRoute("/_auth/")({
  loader: async ({ context }) => {
    const { queryClient } = context;

    await Promise.all([
      queryClient.ensureQueryData(watchRoomsQueryOpts()),
      queryClient.ensureQueryData(latestMoviesQueryOpts()),
      queryClient.ensureQueryData(latestAlbumsQueryOpts()),
      queryClient.ensureQueryData(inTheatersQueryOpts()),
    ]);
  },
  component: HomePage,
});

function HomePage() {
  return (
    <div className="min-w-0">
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <section
        aria-labelledby="home-heading"
        className={cn(
          "rounded-3xl border border-slate-800/80 bg-linear-to-br from-slate-900 via-slate-900 to-slate-950 p-5 shadow-[0_24px_80px_-56px_rgba(245,158,11,0.45)] sm:p-6",
          MOTION_SECTION_ENTER_CLASS,
        )}
      >
        <p className="flex items-center gap-2 text-xs font-semibold tracking-[0.2em] text-amber-300 uppercase">
          <Home
            className="size-3.5"
            aria-hidden="true"
          />
          Dashboard
        </p>
        <h1
          id="home-heading"
          className="mt-3 text-2xl font-semibold tracking-tight text-white sm:text-3xl"
        >
          Welcome to Igloo
        </h1>
        <p className="mt-3 max-w-3xl text-sm text-slate-300 sm:text-base">
          Explore your personal media library with recently added movies,
          albums, shared watch rooms, and what is playing in theaters.
        </p>
      </section>

      <WatchRooms />
      <LatestMovies />
      <LatestAlbums />
      <MoviesInTheaters />
    </div>
  );
}
