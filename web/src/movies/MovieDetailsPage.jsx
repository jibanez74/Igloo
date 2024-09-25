import { useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getMovieByID } from "./httpMovie";
import Spinner from "../shared/Spinner";
import Alert from "../shared/Alert";
import formatDate from "../utils/formatDate";

export default function MovieDetailsPage() {
  const { id } = useParams();

  const {
    data: movie,
    isPending,
    isError,
    error,
  } = useQuery({
    queryKey: ["movie", id],
    queryFn: () => getMovieByID(id),
    refetchOnWindowFocus: false,
  });

  const navigate = useNavigate();

  const playMovie = () => navigate(`/movies/play/${id}`);

  if (isPending) return <Spinner />;

  if (isError) {
    alert(error);
  }

  return (
    <div className='bg-gray-900 text-white min-h-screen'>
      {/* Backdrop */}
      <div
        className='w-full h-96 bg-cover bg-center relative'
        style={{ backgroundImage: `url(${movie.Art})` }}
      >
        <div className='w-full h-full bg-black bg-opacity-50 flex items-end'>
          <div className='container mx-auto px-4 py-6'>
            <h1 className='text-4xl font-bold'>{movie.title}</h1>
            <p className='text-lg text-gray-300 mt-2'>{movie.tagline}</p>
          </div>
        </div>
        {/* Play button */}
        <button
          onClick={playMovie}
          className='absolute top-1/2 left-1/2 transform -translate-x-1/2 -translate-y-1/2 bg-red-600 text-white rounded-full p-4 flex items-center justify-center hover:bg-red-700 transition-colors'
        >
          <svg
            xmlns='http://www.w3.org/2000/svg'
            className='h-12 w-12'
            viewBox='0 0 20 20'
            fill='currentColor'
          >
            <path
              fillRule='evenodd'
              d='M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z'
              clipRule='evenodd'
            />
          </svg>
          <span className='sr-only'>Play movie</span>
        </button>
      </div>

      {/* Main content */}
      <div className='container mx-auto px-4 py-8'>
        <div className='flex flex-col md:flex-row gap-8'>
          {/* Left column */}
          <div className='md:w-1/3'>
            <img
              src={movie.thumb}
              alt={movie.title}
              className='w-full rounded-lg shadow-lg'
            />
            <div className='mt-4 space-y-2'>
              <p>
                <strong>Release Date:</strong> {formatDate(movie.releaseDate)}
              </p>
              <p>
                <strong>Runtime:</strong> {movie.runTime} minutes
              </p>
              <p>
                <strong>Content Rating:</strong> {movie.contentRating}
              </p>
              <p>
                <strong>Budget:</strong> ${movie.budget}
              </p>
              <p>
                <strong>Revenue:</strong> ${movie.revenue}
              </p>
            </div>
          </div>

          {/* Right column */}
          <div className='md:w-2/3'>
            <h2 className='text-2xl font-bold mb-4'>Summary</h2>
            <p className='text-gray-300 mb-6'>{movie.summary}</p>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Genres</h3>
              <div className='flex flex-wrap gap-2'>
                {movie.genres.map(genre => (
                  <span
                    key={genre._id}
                    className='bg-gray-700 px-3 py-1 rounded-full text-sm'
                  >
                    {genre.tag}
                  </span>
                ))}
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Ratings</h3>
              <div className='flex gap-4'>
                <div>
                  <p className='font-bold'>Audience</p>
                  <p>{movie.audienceRating.toFixed(1)}/10</p>
                </div>
                <div>
                  <p className='font-bold'>Critics</p>
                  <p>{movie.criticRating.toFixed(1)}/10</p>
                </div>
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Cast</h3>
              <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4'>
                {movie.castList.slice(0, 8).map(cast => (
                  <div key={cast._id} className='text-center'>
                    <img
                      src={cast.artist.thumb}
                      alt={cast.artist.name}
                      className='w-24 h-24 object-cover rounded-full mx-auto mb-2'
                    />
                    <p className='font-bold'>{cast.artist.name}</p>
                    <p className='text-sm text-gray-400'>{cast.character}</p>
                  </div>
                ))}
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Crew</h3>
              <div className='grid grid-cols-2 sm:grid-cols-3 gap-4'>
                {movie.crewList.slice(0, 6).map(crew => (
                  <div key={crew._id}>
                    <p className='font-bold'>{crew.artist.name}</p>
                    <p className='text-sm text-gray-400'>{crew.job}</p>
                  </div>
                ))}
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Studios</h3>
              <div className='flex flex-wrap gap-4'>
                {movie.studios.map(studio => (
                  <div key={studio._id} className='flex items-center'>
                    <img
                      src={studio.thumb}
                      alt={studio.name}
                      className='w-12 h-12 object-contain mr-2'
                    />
                    <span>{studio.name}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>

        {/* Trailers */}
        {movie.Trailers && movie.Trailers.length > 0 && (
          <div className='mt-12'>
            <h2 className='text-2xl font-bold mb-4'>Trailers</h2>
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4'>
              {movie.trailers.map(trailer => (
                <div key={trailer._id}>
                  <h3 className='font-bold mb-2'>{trailer.title}</h3>
                  <iframe
                    width='100%'
                    height='200'
                    src={trailer.url}
                    title={trailer.title}
                    frameBorder='0'
                    allow='accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture'
                    allowFullScreen
                  ></iframe>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
