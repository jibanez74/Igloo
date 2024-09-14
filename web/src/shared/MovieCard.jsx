import PropTypes from "prop-types";
import { Link } from "react-router-dom";

export default function MovieCard({ movie }) {
  return (
    <div className='bg-primary hover:bg-secondary rounded-lg shadow-lg overflow-hidden transition-colors duration-300'>
      <Link to={`/movies/details/${movie.ID}`}>
        <img
          src={movie.Thumb}
          alt={movie.Title}
          className='w-full h-64 object-cover'
          loading='lazy'
        />
      </Link>

      <div className='p-4'>
        <h3 className='text-light text-xl font-bold mb-2'>{movie.Title}</h3>

        <p className='text-blue-300'>{movie.Year}</p>
      </div>
    </div>
  );
}

MovieCard.propTypes = {
  movie: PropTypes.shape({
    ID: PropTypes.number.isRequired,
    Title: PropTypes.string.isRequired,
    Thumb: PropTypes.string.isRequired,
    Year: PropTypes.number.isRequired,
  }).isRequired,
};
