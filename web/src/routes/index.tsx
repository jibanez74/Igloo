import { createFileRoute } from "@tanstack/react-router";
import { FiPlay, FiFilm } from "react-icons/fi";
import LatestMovies from "@/components/LatestMovies";


export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  return (
    <main className='min-h-screen'>
      {/* Hero Section */}
      <section className='relative h-[60vh] flex items-center justify-center overflow-hidden'>
        {/* Background with gradient overlay */}
        <div className='absolute inset-0 bg-gradient-to-b from-slate-900/90 via-slate-900/80 to-slate-900' />

        {/* Hero Content */}
        <div className='relative z-10 text-center px-4 sm:px-6 lg:px-8'>
          <div className='max-w-3xl mx-auto space-y-6'>
            <FiFilm
              className='w-16 h-16 mx-auto text-blue-400'
              aria-hidden='true'
            />
            <h1 className='text-4xl sm:text-5xl font-bold text-white'>
              Welcome to Igloo
            </h1>
            <p className='text-xl text-blue-200'>
              Your personal media server for movies, TV shows, and more
            </p>
            <div className='flex justify-center gap-4'>
              <button
                type='button'
                className='inline-flex items-center gap-2 px-6 py-3 rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors'
              >
                <FiPlay className='w-5 h-5' aria-hidden='true' />
                <span>Start Watching</span>
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* Content Sections */}
      <div className='bg-slate-900'>
        {/* Latest Movies Section */}
        <LatestMovies />
      </div>
    </main>
  );
}
