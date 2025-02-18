import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  FiClock,
  FiDollarSign,
  FiCalendar,
  FiStar,
  FiAward,
  FiPlay,
  FiHeart,
  FiCheck,
  FiMoreHorizontal,
  FiChevronDown,
  FiChevronUp,
  FiYoutube,
} from "react-icons/fi";
import getImgSrc from "@/utils/getImgSrc";
import formatDate from "@/utils/formatDate";
import formatRuntime from "@/utils/formatRuntime";
import formatDollars from "@/utils/formatDollars";
import type { Movie } from "@/types/Movie";
import YoutubeModal from "@/components/YoutubeModal";

export const Route = createFileRoute("/movies/$movieID/")({
  component: MovieDetailsPage,
  loader: async ({ params }) => {
    const res = await fetch(`/api/v1/movies/${params.movieID}`, {
      credentials: "include",
    });

    if (!res.ok) {
      throw new Error("Failed to fetch movie details");
    }

    return res.json();
  },
});

function MovieDetailsPage() {
  const { movie } = Route.useLoaderData() as { movie: Movie };

  const [showAllCast, setShowAllCast] = useState(false);
  const [showAllCrew, setShowAllCrew] = useState(false);
  const [selectedVideo, setSelectedVideo] = useState<string | null>(null);

  const navigate = Route.useNavigate();

  const displayedCast = showAllCast ? movie.cast : movie.cast.slice(0, 6);
  const displayedCrew = showAllCrew ? movie.crew : movie.crew.slice(0, 6);

  const getYouTubeVideoId = (url: string) => {
    const regExp =
      /^.*(youtu.be\/|v\/|u\/\w\/|embed\/|watch\?v=|&v=)([^#&?]*).*/;
    const match = url.match(regExp);
    return match && match[2].length === 11 ? match[2] : null;
  };

  const handleKeyPress = (
    e: React.KeyboardEvent<HTMLButtonElement>,
    url: string
  ) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      setSelectedVideo(url);
    }
  };

  const handlePlayMovie = async () => {
    const hlsOpts = {
      audio_codec: "copy",
      video_codec: "copy",
    };

    const res = await fetch(`/api/v1/movies/create-hls/${movie.id}`, {
      method: "post",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(hlsOpts),
    });

    if (!res.ok) {
      const data = await res.json();
      alert(JSON.stringify(data));
      return;
    }

    navigate({
      to: "/movies/$movieID/play",
      params: { movieID: movie.id.toString() },
      search: { title: movie.title },
    });
  };

  return (
    <main className='min-h-screen pt-16'>
      {/* Hero Section with Backdrop */}
      <div className='relative min-h-[90vh] w-full'>
        {/* Backdrop Image */}
        <div
          className='absolute inset-0 bg-cover bg-center'
          style={{ backgroundImage: `url(${getImgSrc(movie.art)})` }}
        >
          {/* Even lighter overlays */}
          <div className='absolute inset-0 bg-gradient-to-r from-slate-900/80 via-slate-900/50 to-transparent' />
          <div className='absolute inset-0 bg-gradient-to-t from-slate-900/90 via-slate-900/40 to-transparent' />
        </div>

        {/* Content */}
        <div className='relative h-full container mx-auto px-4 py-12 flex flex-col justify-end'>
          <div className='flex flex-col md:flex-row gap-8 items-end md:items-end'>
            {/* Movie Poster */}
            <div className='relative z-10 md:translate-y-16'>
              <div className='w-48 md:w-72 aspect-[2/3] rounded-lg overflow-hidden ring-1 ring-white/10 shadow-xl shadow-black/50'>
                <img
                  src={getImgSrc(movie.thumb)}
                  alt={`${movie.title} Poster`}
                  className='w-full h-full object-cover'
                />
              </div>
            </div>

            {/* Movie Info */}
            <div className='flex-1 max-w-3xl relative z-10'>
              <h1 className='text-4xl md:text-6xl font-bold text-white mb-4 drop-shadow-xl'>
                {movie.title}
              </h1>
              {movie.tag_line && (
                <p className='text-xl text-sky-200 mb-4 italic drop-shadow-xl'>
                  {movie.tag_line}
                </p>
              )}

              {/* Movie Meta */}
              <div className='flex flex-wrap gap-4 text-sm text-sky-200 mb-6 drop-shadow-lg bg-slate-900/20 backdrop-blur-sm rounded-lg px-4 py-2'>
                <div className='flex items-center gap-1'>
                  <FiCalendar className='w-4 h-4' aria-hidden='true' />
                  <span>{formatDate(movie.release_date)}</span>
                </div>

                <div className='flex items-center gap-1'>
                  <FiClock className='w-4 h-4' aria-hidden='true' />
                  <span>{formatRuntime(movie.run_time)}</span>
                </div>
                {movie.content_rating && (
                  <span className='px-2 py-0.5 bg-sky-500/20 rounded'>
                    {movie.content_rating}
                  </span>
                )}

                {movie.genres.map(genre => (
                  <span
                    key={`genre-${genre.id}`}
                    className='px-2 py-0.5 bg-sky-500/10 rounded'
                  >
                    {genre.tag}
                  </span>
                ))}
              </div>

              {/* Ratings */}
              <div className='flex gap-6 mb-6'>
                {movie.audience_rating > 0 && (
                  <div className='flex items-center gap-2'>
                    <FiStar
                      className='w-5 h-5 text-sky-400'
                      aria-hidden='true'
                    />
                    <div>
                      <div className='text-lg font-medium text-white'>
                        {movie.audience_rating.toFixed(1)}
                      </div>
                      <div className='text-xs text-sky-200'>
                        Audience Rating
                      </div>
                    </div>
                  </div>
                )}
                {movie.critic_rating > 0 && (
                  <div className='flex items-center gap-2'>
                    <FiAward
                      className='w-5 h-5 text-sky-400'
                      aria-hidden='true'
                    />
                    <div>
                      <div className='text-lg font-medium text-white'>
                        {movie.critic_rating.toFixed(1)}
                      </div>
                      <div className='text-xs text-sky-200'>Critic Rating</div>
                    </div>
                  </div>
                )}
              </div>

              {/* Summary */}
              <p className='text-sky-200 leading-relaxed mb-8'>
                {movie.summary}
              </p>

              {/* Action Buttons */}
              <div className='flex flex-wrap items-center gap-4 mb-8'>
                <button
                  type='button'
                  onClick={handlePlayMovie}
                  className='inline-flex items-center gap-2 px-6 py-3 bg-sky-500 hover:bg-sky-400 
                           text-white font-medium rounded-lg transition-colors'
                >
                  <FiPlay className='w-5 h-5' aria-hidden='true' />
                  Play
                </button>

                <button
                  type='button'
                  className='inline-flex items-center gap-2 px-4 py-3 bg-rose-500/20 hover:bg-rose-500/30 
                           text-rose-300 hover:text-rose-200 font-medium rounded-lg transition-colors'
                >
                  <FiHeart className='w-5 h-5' aria-hidden='true' />
                  Like
                </button>

                <button
                  type='button'
                  className='inline-flex items-center gap-2 px-4 py-3 bg-emerald-500/20 hover:bg-emerald-500/30 
                           text-emerald-300 hover:text-emerald-200 font-medium rounded-lg transition-colors'
                >
                  <FiCheck className='w-5 h-5' aria-hidden='true' />
                  Mark as Watched
                </button>

                <button
                  type='button'
                  className='inline-flex items-center justify-center w-12 h-12 bg-slate-800/50 hover:bg-slate-800 
                           text-sky-200 hover:text-sky-100 rounded-lg transition-colors'
                  aria-label='More options'
                >
                  <FiMoreHorizontal className='w-5 h-5' aria-hidden='true' />
                </button>
              </div>

              {/* Additional Info */}
              <div className='grid grid-cols-2 md:grid-cols-3 gap-6'>
                {movie.budget > 0 && (
                  <div>
                    <div className='flex items-center gap-1 text-sky-400 mb-1'>
                      <FiDollarSign className='w-4 h-4' aria-hidden='true' />
                      <span className='text-sm font-medium'>Budget</span>
                    </div>

                    <div className='text-white'>
                      {formatDollars(movie.budget)}
                    </div>
                  </div>
                )}

                {movie.revenue > 0 && (
                  <div>
                    <div className='flex items-center gap-1 text-sky-400 mb-1'>
                      <FiDollarSign className='w-4 h-4' aria-hidden='true' />
                      <span className='text-sm font-medium'>Revenue</span>
                    </div>
                    <div className='text-white'>
                      {formatDollars(movie.revenue)}
                    </div>
                  </div>
                )}

                {movie.studios.length > 0 && (
                  <div>
                    <div className='text-sm font-medium text-sky-400 mb-1'>
                      Studios
                    </div>
                    <div className='text-white'>
                      {movie.studios.map(studio => (
                        <span key={studio.id}>{studio.name}</span>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Cast & Crew Section */}
      <section className='container mx-auto px-4 py-12'>
        <h2 className='text-2xl font-bold text-white mb-6'>Cast & Crew</h2>

        {/* Cast */}
        {movie.cast.length > 0 && (
          <div className='mb-12'>
            <h3 className='text-lg font-medium text-sky-200 mb-4'>Cast</h3>
            <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4'>
              {displayedCast.map((cast, i) => (
                <div key={`cast-${i}`} className='text-center'>
                  <div className='aspect-[2/3] mb-2 rounded-lg bg-slate-800/50 overflow-hidden'>
                    {cast.thumb ? (
                      <img
                        src={getImgSrc(cast.thumb)}
                        alt={cast.name}
                        className='w-full h-full object-cover'
                      />
                    ) : (
                      <div className='w-full h-full flex items-center justify-center bg-slate-800/50'>
                        <span className='text-sky-200/50'>No Image</span>
                      </div>
                    )}
                  </div>
                  <div className='text-sm font-medium text-white'>
                    {cast.name}
                  </div>
                  <div className='text-xs text-sky-200'>{cast.character}</div>
                </div>
              ))}
            </div>
            {movie.cast.length > 6 && (
              <div className='flex justify-center mt-6'>
                <button
                  type='button'
                  onClick={() => setShowAllCast(!showAllCast)}
                  className='inline-flex items-center gap-2 px-4 py-2 bg-slate-800/50 hover:bg-slate-800 
                           text-sky-200 hover:text-sky-100 rounded-lg transition-colors text-sm font-medium'
                >
                  {showAllCast ? (
                    <>
                      Show Less{" "}
                      <FiChevronUp className='w-4 h-4' aria-hidden='true' />
                    </>
                  ) : (
                    <>
                      Show All Cast ({movie.cast.length}){" "}
                      <FiChevronDown className='w-4 h-4' aria-hidden='true' />
                    </>
                  )}
                </button>
              </div>
            )}
          </div>
        )}

        {/* Crew */}
        {movie.crew.length > 0 && (
          <div>
            <h3 className='text-lg font-medium text-sky-200 mb-4'>Crew</h3>
            <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4'>
              {displayedCrew.map((crew, i) => (
                <div key={`crew-${i}`} className='text-center'>
                  <div className='aspect-[2/3] mb-2 rounded-lg bg-slate-800/50 overflow-hidden'>
                    {crew.thumb ? (
                      <img
                        src={getImgSrc(crew.thumb)}
                        alt={crew.name}
                        className='w-full h-full object-cover'
                      />
                    ) : (
                      <div className='w-full h-full flex items-center justify-center bg-slate-800/50'>
                        <span className='text-sky-200/50'>No Image</span>
                      </div>
                    )}
                  </div>
                  <div className='text-sm font-medium text-white'>
                    {crew.name}
                  </div>
                  <div className='text-xs text-sky-200'>{crew.job}</div>
                </div>
              ))}
            </div>
            {movie.crew.length > 6 && (
              <div className='flex justify-center mt-6'>
                <button
                  type='button'
                  onClick={() => setShowAllCrew(!showAllCrew)}
                  className='inline-flex items-center gap-2 px-4 py-2 bg-slate-800/50 hover:bg-slate-800 
                           text-sky-200 hover:text-sky-100 rounded-lg transition-colors text-sm font-medium'
                >
                  {showAllCrew ? (
                    <>
                      Show Less{" "}
                      <FiChevronUp className='w-4 h-4' aria-hidden='true' />
                    </>
                  ) : (
                    <>
                      Show All Crew ({movie.crew.length}){" "}
                      <FiChevronDown className='w-4 h-4' aria-hidden='true' />
                    </>
                  )}
                </button>
              </div>
            )}
          </div>
        )}
      </section>

      {/* Extras Section */}
      {movie.extras.length > 0 && (
        <section className='container mx-auto px-4 py-12 border-t border-sky-200/10'>
          <h2 className='text-2xl font-bold text-white mb-6'>Extras</h2>
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6'>
            {movie.extras.map((extra, i) => {
              const videoId = getYouTubeVideoId(extra.url);
              const thumbnailUrl = videoId
                ? `https://img.youtube.com/vi/${videoId}/maxresdefault.jpg`
                : "";

              return (
                <button
                  key={`extra-${i}`}
                  onClick={() => setSelectedVideo(extra.url)}
                  onKeyDown={e => handleKeyPress(e, extra.url)}
                  className='group relative aspect-video w-full rounded-xl overflow-hidden focus:outline-none 
                           focus:ring-2 focus:ring-sky-500/40 focus:ring-offset-2 focus:ring-offset-slate-800'
                  aria-label={`Play ${extra.title}`}
                >
                  {/* Thumbnail */}
                  <img
                    src={thumbnailUrl}
                    alt={extra.title}
                    className='w-full h-full object-cover transition-transform duration-300 
                             group-hover:scale-105'
                  />

                  {/* Overlay */}
                  <div
                    className='absolute inset-0 bg-gradient-to-t from-slate-900 via-slate-900/50 to-transparent 
                                opacity-80 group-hover:opacity-90 transition-opacity'
                  />

                  {/* Play Button */}
                  <div className='absolute inset-0 flex items-center justify-center'>
                    <div
                      className='rounded-full bg-sky-500/90 p-4 transform transition-all duration-300 
                                  group-hover:bg-sky-400 group-hover:scale-110'
                    >
                      <FiYoutube
                        className='w-8 h-8 text-white'
                        aria-hidden='true'
                      />
                    </div>
                  </div>

                  {/* Text Content */}
                  <div className='absolute bottom-0 left-0 right-0 p-4'>
                    <h3
                      className='text-base font-medium text-white line-clamp-2 mb-1 group-hover:text-sky-200 
                                 transition-colors'
                    >
                      {extra.title}
                    </h3>
                    <p className='text-sm text-sky-200/80 capitalize'>
                      {extra.kind}
                    </p>
                  </div>
                </button>
              );
            })}
          </div>
        </section>
      )}

      {/* YouTube Modal */}
      <YoutubeModal
        isOpen={!!selectedVideo}
        onClose={() => setSelectedVideo(null)}
        videoUrl={selectedVideo}
      />
    </main>
  );
}
