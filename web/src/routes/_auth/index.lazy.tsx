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
        <h1 className='flex items-center gap-3 text-3xl font-semibold tracking-tight text-white md:text-4xl'>
          <i className='fa-solid fa-house text-2xl text-amber-400'></i>
          <span>Welcome to Igloo</span>
        </h1>

        <p className='mt-2 max-w-2xl text-sm text-slate-400 md:text-base'>
          Explore your personal media library — recently added movies, TV shows,
          music, and more.
        </p>
      </header>
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2'></div>
      <LatestAlbums />
      <MoviesInTheaters />
    </>
  );
}
