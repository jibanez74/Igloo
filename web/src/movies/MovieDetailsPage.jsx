import { useParams, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getMovie } from "./httpMovie";
import Spinner from "../shared/Spinner";
import Alert from "../shared/Alert";
import formatDate from "../utils/formatDate";
import { v4 } from "uuid";

export default function MovieDetailsPage() {
  const { id } = useParams();
  const navigate = useNavigate();

  const {
    data: movie,
    isPending,
    isError,
    error,
  } = useQuery({
    queryKey: ["movie", id],
    queryFn: () => getMovie(id),
    refetchOnWindowFocus: false,
  });

  const playMovie = async () => {
    const body = {
      FilePath: movie.FilePath,
      OutputDir: "/home/romany/transcode",
      OutputName: v4(),
      Container: movie.Container,
      AudioCodec: movie.AudioCodec,
      VideoCodec: movie.VideoCodec,
      Resolution: movie.Reso.ution,
      AudioChannels: 2.0,
    };

    const res = await fetch("/api/v1/transcode/video", {
      method: "post",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      alert(`${res.status} ${res.statusText}`);
      return;
    }

    const { PID: pid } = await res.json();

    const filePath = encodeURIComponent(
      `${body.OutputDir}/${body.OutputName}.m3u8`
    );

    navigate(`/play/video?pid=${pid}&file_path=${filePath}`);
  };

  if (isPending) return <Spinner />;
  if (isError)
    return (
      <Alert title='Error' msg={error.message} time={6000} variant='danger' />
    );

  return (
    <div
      className='relative min-h-screen bg-cover bg-center'
      style={{ backgroundImage: `url(${movie.Art})` }}
    >
      <div className='absolute inset-0 bg-black bg-opacity-70'></div>
      <div className='relative z-10 container mx-auto px-4 py-8'>
        <div className='flex flex-col md:flex-row'>
          <div className='md:w-1/3 mb-8 md:mb-0'>
            <img
              src={movie.Thumb}
              alt={movie.Title}
              className='w-full rounded-lg shadow-lg'
            />
          </div>
          <div className='md:w-2/3 md:pl-8'>
            <h1 className='text-4xl font-bold text-white mb-4'>
              {movie.Title}
            </h1>
            <p className='text-xl text-gray-300 mb-4'>{movie.TagLine}</p>
            <div className='flex space-x-4 mb-6'>
              <button
                onClick={playMovie}
                className='bg-blue-500 hover:bg-blue-600 text-white font-bold py-2 px-4 rounded'
              >
                Play Movie
              </button>
              <button className='bg-green-500 hover:bg-green-600 text-white font-bold py-2 px-4 rounded'>
                Mark as Watched
              </button>
              <button className='bg-purple-500 hover:bg-purple-600 text-white font-bold py-2 px-4 rounded'>
                Playback Settings
              </button>
            </div>
            <div className='grid grid-cols-1 md:grid-cols-2 gap-4 text-white'>
              <div>
                <p>
                  <strong>Year:</strong> {movie.Year}
                </p>
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
                  <strong>Audience Rating:</strong>{" "}
                  {movie.AudienceRating.toFixed(1)}/10
                </p>
                <p>
                  <strong>Critic Rating:</strong>{" "}
                  {movie.CriticRating.toFixed(1)}/10
                </p>
              </div>
              <div>
                <p>
                  <strong>Budget:</strong> ${movie.Budget.toLocaleString()}
                </p>
                <p>
                  <strong>Revenue:</strong> ${movie.Revenue.toLocaleString()}
                </p>
                <p>
                  <strong>TMDB ID:</strong> {movie.TmdbID}
                </p>
                <p>
                  <strong>IMDB ID:</strong> {movie.ImdbID}
                </p>
                <p>
                  <strong>File Size:</strong>{" "}
                  {(movie.Size / (1024 * 1024)).toFixed(2)} MB
                </p>
                <p>
                  <strong>Container:</strong> {movie.Container}
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className='mt-8 text-white'>
          <h2 className='text-2xl font-bold mb-4'>Summary</h2>
          <p className='text-gray-300'>{movie.Summary}</p>
        </div>

        <div className='mt-8 text-white'>
          <h2 className='text-2xl font-bold mb-4'>Genres</h2>
          <div className='flex flex-wrap gap-2'>
            {movie.Genres.map(genre => (
              <span
                key={genre.ID}
                className='bg-gray-700 px-3 py-1 rounded-full text-sm'
              >
                {genre.Tag}
              </span>
            ))}
          </div>
        </div>

        <div className='mt-8 text-white'>
          <h2 className='text-2xl font-bold mb-4'>Studios</h2>
          <div className='flex flex-wrap gap-4'>
            {movie.Studios.map(studio => (
              <div key={studio.ID} className='flex items-center'>
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

        <div className='mt-8 text-white'>
          <h2 className='text-2xl font-bold mb-4'>Cast</h2>
          <div className='grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4'>
            {movie.CastList.map(cast => (
              <div key={cast.ID} className='text-center'>
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

        <div className='mt-8 text-white'>
          <h2 className='text-2xl font-bold mb-4'>Crew</h2>
          <div className='grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4'>
            {movie.CrewList.map(crew => (
              <div key={crew.ID}>
                <p className='font-bold'>{crew.Artist.Name}</p>
                <p className='text-sm text-gray-400'>{crew.Job}</p>
                <p className='text-xs text-gray-500'>{crew.Department}</p>
              </div>
            ))}
          </div>
        </div>

        <div className='mt-8 text-white'>
          <h2 className='text-2xl font-bold mb-4'>Trailers</h2>
          <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4'>
            {movie.Trailers.map(trailer => (
              <div key={trailer.ID}>
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

        <div className='mt-8 text-white'>
          <h2 className='text-2xl font-bold mb-4'>Technical Details</h2>
          <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4'>
            <div>
              <h3 className='text-xl font-bold mb-2'>Video</h3>
              {movie.VideoList.map((video, index) => (
                <div key={index} className='mb-2'>
                  <p>
                    <strong>Codec:</strong> {video.Codec}
                  </p>
                  <p>
                    <strong>Resolution:</strong> {video.Width}x{video.Height}
                  </p>
                  <p>
                    <strong>Aspect Ratio:</strong> {video.AspectRatio}
                  </p>
                </div>
              ))}
            </div>
            <div>
              <h3 className='text-xl font-bold mb-2'>Audio</h3>
              {movie.AudioList.map((audio, index) => (
                <div key={index} className='mb-2'>
                  <p>
                    <strong>Language:</strong> {audio.Language}
                  </p>
                  <p>
                    <strong>Codec:</strong> {audio.Codec}
                  </p>
                  <p>
                    <strong>Channels:</strong> {audio.Channels}
                  </p>
                </div>
              ))}
            </div>
            <div>
              <h3 className='text-xl font-bold mb-2'>Subtitles</h3>
              {movie.SubtitleList.map((subtitle, index) => (
                <div key={index} className='mb-2'>
                  <p>
                    <strong>Language:</strong> {subtitle.Language}
                  </p>
                  <p>
                    <strong>Format:</strong> {subtitle.Format}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
