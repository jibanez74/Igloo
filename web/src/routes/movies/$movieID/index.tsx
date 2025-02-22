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
  FiInfo,
} from "react-icons/fi";
import getImgSrc from "@/utils/getImgSrc";
import formatDate from "@/utils/formatDate";
import formatRuntime from "@/utils/formatRuntime";
import formatDollars from "@/utils/formatDollars";
import CastSection from "@/components/CastSection";
import CrewSection from "@/components/CrewSection";
import GenreList from "@/components/GenreList";
import YoutubeModal from "@/components/YoutubeModal";
import StudioList from "@/components/StudioList";
import ExtrasSection from "@/components/ExtrasSection";
import Dropdown from "@/components/Dropdown";
import PlaybackSettingsModal from "@/components/PlaybackSettingsModal";
import type { Movie } from "@/types/Movie";
import type { MovieHlsOpts } from "@/types/Transcode";

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

  const audioOpts =
    movie.audio_streams.length > 0
      ? movie.audio_streams.map(s => ({
          value: s.index,
          label: `${s.title} ${s.language}`,
        }))
      : [];

  const subtitleOpts =
    movie.subtitles.length > 0
      ? movie.subtitles.map(s => ({
          value: s.index,
          label: `${s.title} ${s.language}`,
        }))
      : [];

  const [selectedVideo, setSelectedVideo] = useState<string | null>(null);
  const [isPlaybackModalOpen, setIsPlaybackModalOpen] = useState(false);

  const navigate = Route.useNavigate();

  const handlePlayMovie = async () => {
    const search: MovieHlsOpts = {
      title: movie.title,
      audio_codec: "aac",
      audio_bit_rate: 192,
      audio_channels: 2,
      audio_stream_index: movie.audio_streams[0].index,
      video_stream_index: movie.video_streams[0].index,
      video_codec: "copy",
    };

    navigate({
      to: "/movies/$movieID/play",
      params: { movieID: movie.id.toString() },
      search,
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

                <GenreList genres={movie.genres} />
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

                <Dropdown
                  trigger={
                    <button
                      type='button'
                      className='inline-flex items-center justify-center w-12 h-12 bg-slate-800/50 hover:bg-slate-800 
                               text-sky-200 hover:text-sky-100 rounded-lg transition-colors'
                    >
                      <FiMoreHorizontal
                        className='w-5 h-5'
                        aria-hidden='true'
                      />
                    </button>
                  }
                  label={`More options for ${movie.title}`}
                  items={[
                    {
                      label: "Playback Settings",
                      icon: <FiPlay className='w-4 h-4' />,
                      onClick: () => setIsPlaybackModalOpen(true),
                    },
                    {
                      label: "Movie Details",
                      icon: <FiInfo className='w-4 h-4' />,
                      onClick: () => {
                        console.log("Show details");
                      },
                    },
                  ]}
                />
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

                <StudioList studios={movie.studios} />
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Cast & Crew Section */}
      <section className='container mx-auto px-4 py-12'>
        <h2 className='text-2xl font-bold text-white mb-6'>Cast & Crew</h2>
        {movie.cast.length > 0 && <CastSection cast={movie.cast} />}
        {movie.crew.length > 0 && <CrewSection crew={movie.crew} />}
      </section>

      {/* Extras Section */}
      <ExtrasSection
        extras={movie.extras}
        onSelectVideo={url => setSelectedVideo(url)}
      />

      {/* YouTube Modal */}
      <YoutubeModal
        isOpen={!!selectedVideo}
        onClose={() => setSelectedVideo(null)}
        videoUrl={selectedVideo}
      />

      <PlaybackSettingsModal
        isOpen={isPlaybackModalOpen}
        onClose={() => setIsPlaybackModalOpen(false)}
        audioOpts={audioOpts}
        subtitleOpts={subtitleOpts}
      />
    </main>
  );
}
