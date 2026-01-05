import { Link } from "@tanstack/react-router";
import { TMDB_IMAGE_BASE } from "@/lib/constants";
import type { TheaterMovieType } from "@/types";

type MovieCardProps = {
  movie: TheaterMovieType;
};

export default function MovieCard({ movie }: MovieCardProps) {
  const { id, title, poster_path, vote_average, release_date } = movie;

  const posterUrl = poster_path ? `${TMDB_IMAGE_BASE}${poster_path}` : null;
  const rating = vote_average ? vote_average.toFixed(1) : null;
  const year = release_date ? new Date(release_date).getFullYear() : null;

  // Alaska-themed rating colors (aurora borealis inspired)
  const getRatingColor = (score: number) => {
    if (score >= 7) return "bg-teal-500 text-white"; // Northern lights green-blue
    if (score >= 5) return "bg-sky-500 text-white"; // Glacier blue
    return "bg-slate-500 text-white"; // Frozen gray
  };

  return (
    <article className='group relative rounded-xl overflow-hidden bg-slate-900 border border-slate-800 transition-all duration-300 hover:shadow-xl hover:shadow-cyan-500/20 hover:border-cyan-500/50 hover:-translate-y-1'>
      <Link
        to='/movies/in-theaters/$id'
        params={{ id: id.toString() }}
        className='block focus:outline-none focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 rounded-xl'
        aria-label={`${title}${year ? `, ${year}` : ""}${rating ? `, rated ${rating} out of 10` : ""}`}
      >
      {/* Poster with 2:3 aspect ratio (standard movie poster) */}
        <div className='aspect-2/3 bg-slate-800 relative'>
        {posterUrl ? (
          <img
            src={posterUrl}
              alt=''
              width='500'
              height='750'
              loading='lazy'
              decoding='async'
              fetchPriority='low'
              className='h-full w-full object-cover transition-transform duration-300 group-hover:scale-105'
          />
        ) : (
            <div className='h-full w-full flex items-center justify-center'>
              <i
                className='fa-solid fa-film text-slate-600 text-4xl'
                aria-hidden='true'
              ></i>
          </div>
        )}

        {/* Rating badge */}
        {rating && (
          <div
            className={`absolute top-2 right-2 px-2 py-0.5 rounded-md text-xs font-bold shadow-lg ${getRatingColor(vote_average)}`}
              aria-hidden='true'
          >
              <i className='fa-solid fa-star text-[10px] mr-1'></i>
            {rating}
          </div>
        )}

        {/* Gradient overlay for text readability */}
          <div className='absolute inset-x-0 bottom-0 h-24 bg-linear-to-t from-black/90 via-black/50 to-transparent' />
      </div>

      {/* Movie info */}
        <div className='absolute bottom-0 inset-x-0 p-3'>
          <h3 className='text-sm font-semibold truncate text-white drop-shadow-lg'>
          {title}
        </h3>
        {year && (
            <p className='text-xs text-slate-300 mt-0.5 drop-shadow-lg'>
              {year}
            </p>
        )}
      </div>
      </Link>
    </article>
  );
}
