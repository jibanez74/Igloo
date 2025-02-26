import { Link } from "@tanstack/react-router";
import { FiPlay } from "react-icons/fi";
import getImgSrc from "../utils/getImgSrc";
import type { SimpleMovie } from "../types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  showPlayButton?: boolean;
  imgLoading: "eager" | "lazy";
};

export default function MovieCard({
  movie,
  showPlayButton = true,
  imgLoading,
}: MovieCardProps) {
  const imgSrc = getImgSrc(movie.thumb);

  return (
    <Link
      to='/movies/$movieID'
      params={{
        movieID: movie.id.toString(),
      }}
    >
      <div className='group relative bg-slate-900/50 backdrop-blur-sm rounded-xl overflow-hidden shadow-lg shadow-blue-900/20 transition-all duration-300 hover:shadow-blue-500/20 hover:scale-[1.02]'>
        {/* Movie Poster with Link */}

        <img
          src={imgSrc}
          alt={`${movie.title}`}
          className='w-full h-full object-cover transition-transform duration-300 group-hover:scale-105'
          loading={imgLoading}
        />

        <div className='absolute inset-0 bg-gradient-to-t from-slate-900 via-transparent to-transparent opacity-50' />

        {/* Movie Info */}
        <div className='p-4 space-y-4'>
          <div>
            <h3
              className='text-lg font-semibold text-white line-clamp-1 group/title relative'
              title={movie.title}
            >
              <span className='absolute invisible group-hover/title:visible opacity-0 group-hover/title:opacity-100 bottom-full mb-2 left-1/2 -translate-x-1/2 px-2 py-1 bg-slate-800 rounded text-sm whitespace-nowrap transition-all duration-200 z-10 shadow-lg shadow-black/20'>
                {movie.title}
              </span>
              {movie.title}
            </h3>
            <p className='text-sm text-blue-200'>{movie.year}</p>
          </div>

          {/* Play Button */}
          {showPlayButton && (
            <button
              className='w-full flex items-center justify-center gap-2 py-2 px-4 bg-blue-600/20 hover:bg-blue-600 text-blue-200 hover:text-white rounded-lg transition-colors duration-300'
              type='button'
            >
              <FiPlay className='w-4 h-4' aria-hidden='true' />
              <span>Play Movie</span>
            </button>
          )}
        </div>
      </div>
    </Link>
  );
}
