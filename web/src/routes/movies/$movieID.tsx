import {
  createFileRoute,
  useLoaderData,
  useNavigate,
} from "@tanstack/react-router";
import { FaCog, FaPlay, FaCheck } from "react-icons/fa";
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

  const playHandler = () =>
    navigate({
      to: "/movies/play",
      search: {
        filePath: movie.filePath,
        directPlay: true,
        videoHeight: movie.videoList[0]?.height,
        container: "mp4",
      },
    });

  return (
    <div className='container p-4'>
      <div className='flex flex-col lg:flex-row gap-4'>
        {/* Poster and Backdrop */}
        <div className='flex-shrink-0'>
          <img
            src={`https://image.tmdb.org/t/p/original${movie.thumb}`}
            alt={movie.title}
            className='w-full lg:w-64 mb-4'
          />
          <img
            src={`https://image.tmdb.org/t/p/original${movie.art}`}
            alt={`${movie.title} backdrop`}
            className='w-full lg:w-96'
          />
        </div>

        {/* Movie Information */}
        <div className='flex-grow'>
          <h1 className='text-3xl font-bold text-dark mb-2'>{movie.title}</h1>
          <p className='text-lg text-secondary italic'>{movie.tagLine}</p>
          <p className='text-sm text-light'>{movie.summary}</p>

          {/* Additional Info */}
          <div className='mt-4'>
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
            <p>
              <strong>Audience Rating:</strong> {movie.audienceRating}
            </p>
            <p>
              <strong>Critic Rating:</strong> {movie.criticRating}
            </p>
            <p>
              <strong>Languages:</strong> {movie.spokenLanguages}
            </p>
          </div>

          {/* Genres */}
          <div className='mt-4'>
            <h2 className='text-xl font-semibold text-primary'>Genres</h2>
            <div className='flex flex-wrap gap-2'>
              {movie.genres.map(genre => (
                <span
                  key={genre.ID}
                  className='bg-secondary text-dark px-2 py-1 rounded'
                >
                  {genre.tag}
                </span>
              ))}
            </div>
          </div>

          {/* Cast */}
          <div className='mt-4'>
            <h2 className='text-xl font-semibold text-primary'>Cast</h2>
            <div className='flex flex-wrap gap-2'>
              {movie.castList.map(cast => (
                <div
                  key={cast.ID}
                  className='flex flex-col items-center text-center'
                >
                  <p className='font-medium'>{cast.artist.name}</p>
                  <p className='text-sm text-light'>{cast.character}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Crew */}
          <div className='mt-4'>
            <h2 className='text-xl font-semibold text-primary'>Crew</h2>
            <div className='flex flex-wrap gap-2'>
              {movie.crewList.map(crew => (
                <div
                  key={crew.ID}
                  className='flex flex-col items-center text-center'
                >
                  <p className='font-medium'>{crew.artist.name}</p>
                  <p className='text-sm text-light'>{crew.job}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Buttons */}
          <div className='mt-6 flex space-x-4'>
            <button
              onClick={playHandler}
              className='bg-success text-white px-4 py-2 rounded flex items-center space-x-2 hover:bg-opacity-80'
            >
              <FaPlay className='mr-2' /> Play
            </button>

            <button className='bg-primary text-white px-4 py-2 rounded flex items-center space-x-2 hover:bg-opacity-80'>
              <FaCheck className='mr-2' /> Mark as Watched
            </button>

            <button className='bg-secondary text-white px-4 py-2 rounded flex items-center space-x-2 hover:bg-opacity-80'>
              <FaCog className='mr-2' /> Playback Settings
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
