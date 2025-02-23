import { Link } from "@tanstack/react-router";
import { FiPlay } from "react-icons/fi";
import getImgSrc from "@/utils/getImgSrc";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  showPlayButton?: boolean;
};

export default function MovieCard({
  movie,
  showPlayButton = true,
}: MovieCardProps) {
  const imgSrc = getImgSrc(movie.thumb);

  return (
    <Link
      to='/movies/$movieID'
      params={{ movieID: movie.id.toString() }}
      className='block w-full transition-transform hover:scale-105 focus:outline-none focus:ring-2 focus:ring-sky-500 rounded-xl'
    >
      <div className='relative aspect-[2/3] rounded-xl overflow-hidden bg-slate-800'>
        <img
          src={imgSrc}
          alt={`${movie.title} poster`}
          className='w-full h-full object-cover'
          loading='lazy'
        />
        <div className='absolute bottom-0 left-0 right-0 p-2 bg-gradient-to-t from-black/80 to-transparent'>
          <h3 className='text-sm font-medium text-white truncate'>
            {movie.title}
          </h3>
          <p className='text-xs text-sky-200'>{movie.year}</p>
        </div>
      </div>

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
    </Link>
  );
}
