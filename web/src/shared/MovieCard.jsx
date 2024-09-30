import PropTypes from "prop-types";
import { Link } from "react-router-dom";

export default function MovieCard({ movie }) {
  return (
    <div className='bg-primary hover:bg-secondary rounded-lg shadow-lg overflow-hidden transition-colors duration-300'>
      <Link to={`/movies/details/${movie.ID}`}>
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

MovieCard.propTypes = {
  movie: PropTypes.shape({
    ID: PropTypes.number.isRequired,
    title: PropTypes.string.isRequired,
    thumb: PropTypes.string.isRequired,
    year: PropTypes.number.isRequired,
  }).isRequired,
};
