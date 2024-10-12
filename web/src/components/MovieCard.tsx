import { Link } from "@tanstack/react-router";
import type { SimpleMovie } from "@/types/Movie";

export default function MovieCard({ movie }: { movie: SimpleMovie }) {
  return (
    <div className='bg-primary hover:bg-secondary rounded-lg shadow-lg overflow-hidden transition-colors duration-300'>
      <Link to={`/movies/${movie.ID}`}>
        <img
          src={movie.thumb}
          alt={movie.title}
          className='w-full'
          loading='lazy'
        />
      </Link>

      <div className='p-4'>
        <h3 className='text-light text-xl font-bold mb-2'>{movie.title}</h3>

        <p className='text-blue-300'>{movie.year}</p>
      </div>
    </div>
  );
}
