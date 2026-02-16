import { createLazyFileRoute } from "@tanstack/react-router";
import { Home } from "lucide-react";
import LatestAlbums from "@/components/LatestAlbums";
import LatestMovies from "@/components/LatestMovies";
import MoviesInTheaters from "@/components/MoviesInTheaters";

const pageTitle = "Home - Igloo";
const pageDescription =
  "Welcome to Igloo - explore your personal media library with recently added movies, TV shows, music, and more.";

export const Route = createLazyFileRoute("/_auth/")({
  component: HomePage,
});

function HomePage() {
  return (
    <>
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <header className="mb-8">
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-white md:text-4xl">
          <Home className="size-6 text-amber-400" aria-hidden="true" />
          <span>Welcome to Igloo</span>
        </h1>

        <p className="mt-2 max-w-2xl text-sm text-slate-400 md:text-base">
          Explore your personal media library — recently added movies, TV shows,
          music, and more.
        </p>
      </header>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2"></div>
      <LatestMovies />
      <LatestAlbums />
      <MoviesInTheaters />
    </>
  );
}
