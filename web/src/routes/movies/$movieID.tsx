import {
  createFileRoute,
  useLoaderData,
  useNavigate,
  Link,
} from "@tanstack/react-router";
import { FaArrowLeft, FaCog, FaPlay, FaCheck } from "react-icons/fa";
import type { Movie } from "@/types/Movie";
import type { Res } from "@/types/Response";

export const Route = createFileRoute("/movies/$movieID")({
  component: MovieDetailsPage,
  loader: async ({ params }): Promise<Movie> => {
    const res = await fetch(`/api/v1/movie/by-id/${params.movieID}`);

    const r: Res<Movie> = await res.json();

    if (r.error) {
      throw new Error(`${res.status} - ${r.message}`);
    }

    return r.data!;
  },
});

function MovieDetailsPage() {
  const movie = useLoaderData({
    from: "/movies/$movieID",
    strict: true,
  });

  const navigate = useNavigate();

  const playHandler = () => {
    navigate({
      to: "/movies/play",
      search: {
        container: "mp4",
        contentType: movie.contentType,
        filePath: movie.filePath,
        thumb: movie.thumb,
      },
    });
  };

  return (
    <div className='bg-dark text-light min-h-screen p-4'>
      <div className='container max-w-4xl mx-auto'>
        {/* Back Button */}
        <Link
          to='/movies'
          className='flex items-center space-x-2 bg-secondary text-dark px-4 py-2 rounded mb-4'
        >
          <FaArrowLeft />
          <span>Back to Movies</span>
        </Link>

        {/* Movie Header */}
        <div
          className='relative h-64 bg-cover bg-center rounded-lg'
          style={{ backgroundImage: `url(${movie.art})` }}
        >
          <div className='absolute inset-0 bg-dark bg-opacity-75 flex flex-col justify-center items-center'>
            <h1 className='text-4xl font-bold text-light'>
              {movie.title} ({movie.year})
            </h1>
            <p className='text-lg text-info'>{movie.tagLine}</p>
          </div>
        </div>

        {/* Button Group */}
        <div className='mt-4 flex justify-center space-x-4'>
          <button
            onClick={playHandler}
            className='bg-success text-white px-4 py-2 rounded flex items-center space-x-2'
          >
            <FaPlay />
            <span>Play</span>
          </button>
          <button className='bg-primary text-white px-4 py-2 rounded flex items-center space-x-2'>
            <FaCheck />
            <span>Mark as Watched</span>
          </button>
          <button className='bg-secondary text-dark px-4 py-2 rounded flex items-center space-x-2'>
            <FaCog />
            <span>Playback Settings</span>
          </button>
        </div>

        {/* Movie Information */}
        <div className='mt-6'>
          <h2 className='text-2xl font-bold text-secondary'>Summary</h2>
          <p className='mt-2'>{movie.summary}</p>

          <div className='grid grid-cols-1 md:grid-cols-2 gap-4 mt-6'>
            {/* Poster */}
            <div>
              <img
                src={movie.thumb}
                alt={movie.title}
                className='rounded-lg shadow-lg'
              />
            </div>

            {/* Movie Details */}
            <div className='space-y-4'>
              <div>
                <h3 className='text-xl font-semibold text-secondary'>
                  Details
                </h3>
                <p>
                  <strong>Release Date:</strong>{" "}
                  {new Date(movie.releaseDate).toLocaleDateString()}
                </p>
                <p>
                  <strong>Runtime:</strong> {movie.runTime} minutes
                </p>
                <p>
                  <strong>Budget:</strong> ${movie.budget.toLocaleString()}
                </p>
                <p>
                  <strong>Revenue:</strong> ${movie.revenue.toLocaleString()}
                </p>
                <p>
                  <strong>Content Rating:</strong> {movie.contentRating}
                </p>
              </div>

              <div>
                <h3 className='text-xl font-semibold text-secondary'>
                  Ratings
                </h3>
                <p>
                  <strong>Audience Rating:</strong> {movie.audienceRating}
                </p>
                <p>
                  <strong>Critic Rating:</strong> {movie.criticRating}
                </p>
              </div>
            </div>
          </div>

          {/* Genres */}
          <div className='mt-6'>
            <h3 className='text-xl font-bold text-secondary'>Genres</h3>
            <div className='flex flex-wrap gap-2 mt-2'>
              {movie.genres.map(genre => (
                <div
                  key={genre.ID}
                  className='bg-info text-dark px-4 py-2 rounded-lg shadow-sm'
                >
                  {genre.tag}
                </div>
              ))}
            </div>
          </div>

          {/* Cast */}
          <div className='mt-6'>
            <h3 className='text-xl font-bold text-secondary'>Cast</h3>
            <div className='flex flex-wrap gap-4 mt-2'>
              {movie.castList.map(cast => (
                <div key={cast.ID} className='flex flex-col items-center'>
                  <img
                    src={cast.artist.thumb}
                    alt={cast.artist.name}
                    className='w-24 h-24 rounded-full shadow-lg'
                  />
                  <p className='text-light'>{cast.artist.name}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Crew */}
          <div className='mt-6'>
            <h3 className='text-xl font-bold text-secondary'>Crew</h3>
            <div className='flex flex-wrap gap-4 mt-2'>
              {movie.crewList.map(crew => (
                <div key={crew.ID} className='flex flex-col items-center'>
                  <img
                    src={crew.artist.thumb}
                    alt={crew.artist.name}
                    className='w-24 h-24 rounded-full shadow-lg'
                  />
                  <p className='text-light'>
                    {crew.artist.name} - {crew.job}
                  </p>
                </div>
              ))}
            </div>
          </div>

          {/* Studios */}
          <div className='mt-6'>
            <h3 className='text-xl font-bold text-secondary'>Studio</h3>
            <div className='flex flex-col items-center'>
              <img
                src={movie.studios.logo}
                alt={movie.studios.name}
                className='w-32 h-32 rounded-lg shadow-lg'
              />
              <p className='text-light mt-2'>{movie.studios.name}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
