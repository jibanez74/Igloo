import { createLazyFileRoute } from "@tanstack/react-router";
import LatestAlbums from "@/components/LatestAlbums";
import MoviesInTheaters from "@/components/MoviesInTheaters";

export const Route = createLazyFileRoute("/_auth/")({
  component: HomePage,
});

function HomePage() {
  return (
    <>
      <header className='mb-8'>
        <h1 className='text-3xl md:text-4xl font-semibold tracking-tight text-white flex items-center gap-3'>
          <i className='fa-solid fa-house text-amber-400 text-2xl'></i>
          <span>Welcome to Igloo</span>
        </h1>

        <p className='mt-2 text-slate-400 max-w-2xl text-sm md:text-base'>
          Explore your personal media library — recently added movies, TV shows,
          music, and more.
        </p>
      </header>
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4"></div>
      <LatestAlbums />
      <MoviesInTheaters />
    </>
  );
}
