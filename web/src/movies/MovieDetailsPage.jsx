import { useParams } from "react-router-dom";
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

  if (isPending) return <Spinner />;
  if (isError)
    return (
      <Alert title='Error' msg={error.message} time={6000} variant='danger' />
    );

  return (
    <div className='bg-gray-900 text-white min-h-screen'>
      {/* Backdrop */}
      <div
        className='w-full h-96 bg-cover bg-center'
        style={{ backgroundImage: `url(${movie.Art})` }}
      >
        <div className='w-full h-full bg-black bg-opacity-50 flex items-end'>
          <div className='container mx-auto px-4 py-6'>
            <h1 className='text-4xl font-bold'>{movie.Title}</h1>
            <p className='text-lg text-gray-300 mt-2'>{movie.TagLine}</p>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className='container mx-auto px-4 py-8'>
        <div className='flex flex-col md:flex-row gap-8'>
          {/* Left column */}
          <div className='md:w-1/3'>
            <img
              src={movie.Thumb}
              alt={movie.Title}
              className='w-full rounded-lg shadow-lg'
            />
            <div className='mt-4 space-y-2'>
              <p>
                <strong>Release Date:</strong> {formatDate(movie.ReleaseDate)}
              </p>
              <p>
                <strong>Runtime:</strong> {movie.RunTime} minutes
              </p>
              <p>
                <strong>Content Rating:</strong> {movie.ContentRating}
              </p>
              <p>
                <strong>Budget:</strong> ${movie.Budget.toLocaleString()}
              </p>
              <p>
                <strong>Revenue:</strong> ${movie.Revenue.toLocaleString()}
              </p>
            </div>
          </div>

          {/* Right column */}
          <div className='md:w-2/3'>
            <h2 className='text-2xl font-bold mb-4'>Overview</h2>
            <p className='text-gray-300 mb-6'>{movie.Summary}</p>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Genres</h3>
              <div className='flex flex-wrap gap-2'>
                {movie.Genres.map(genre => (
                  <span
                    key={genre._id}
                    className='bg-gray-700 px-3 py-1 rounded-full text-sm'
                  >
                    {genre.Tag}
                  </span>
                ))}
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Ratings</h3>
              <div className='flex gap-4'>
                <div>
                  <p className='font-bold'>Audience</p>
                  <p>{movie.AudienceRating.toFixed(1)}/10</p>
                </div>
                <div>
                  <p className='font-bold'>Critics</p>
                  <p>{movie.CriticRating.toFixed(1)}/10</p>
                </div>
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Cast</h3>
              <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4'>
                {movie.CastList.slice(0, 8).map(cast => (
                  <div key={cast._id} className='text-center'>
                    <img
                      src={cast.Artist.Thumb}
                      alt={cast.Artist.Name}
                      className='w-24 h-24 object-cover rounded-full mx-auto mb-2'
                    />
                    <p className='font-bold'>{cast.Artist.Name}</p>
                    <p className='text-sm text-gray-400'>{cast.Character}</p>
                  </div>
                ))}
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Crew</h3>
              <div className='grid grid-cols-2 sm:grid-cols-3 gap-4'>
                {movie.CrewList.slice(0, 6).map(crew => (
                  <div key={crew._id}>
                    <p className='font-bold'>{crew.Artist.Name}</p>
                    <p className='text-sm text-gray-400'>{crew.Job}</p>
                  </div>
                ))}
              </div>
            </div>

            <div className='mb-6'>
              <h3 className='text-xl font-bold mb-2'>Studios</h3>
              <div className='flex flex-wrap gap-4'>
                {movie.Studios.map(studio => (
                  <div key={studio._id} className='flex items-center'>
                    <img
                      src={studio.Thumb}
                      alt={studio.Name}
                      className='w-12 h-12 object-contain mr-2'
                    />
                    <span>{studio.Name}</span>
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
              {movie.Trailers.map(trailer => (
                <div key={trailer._id}>
                  <h3 className='font-bold mb-2'>{trailer.Title}</h3>
                  <iframe
                    width='100%'
                    height='200'
                    src={trailer.Url}
                    title={trailer.Title}
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
