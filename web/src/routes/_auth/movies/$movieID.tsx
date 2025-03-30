import { createSignal, createMemo } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import {
  FiClock,
  FiDollarSign,
  FiCalendar,
  FiStar,
  FiAward,
  FiPlay,
  FiHeart,
  FiCheck,
} from "solid-icons/fi";
import getImgSrc from "../../../utils/getImgSrc";
import formatDate from "../../../utils/formatDate";
import formatRuntime from "../../../utils/formatRuntime";
import formatDollars from "../../../utils/formatDollars";
import CastSection from "../../../components/CastSection";
import CrewSection from "../../../components/CrewSection";
import GenreList from "../../../components/GenreList";
import YoutubeModal from "../../../components/YoutubeModal";
import StudioList from "../../../components/StudioList";
import ExtrasSection from "../../../components/ExtrasSection";
import PlaybackSettingsModal from "../../../components/PlaybackSettingsModal";
import type { MovieDetailsResponse } from "../../../types/Movie";
import type { MovieHlsOpts } from "../../../types/Transcode";

export const Route = createFileRoute("/_auth/movies/$movieID")({
  component: MovieDetailsPage,
  loader: async ({ params }): Promise<MovieDetailsResponse> => {
    try {
      const res = await fetch(`/api/v1/movies/${params.movieID}`, {
        credentials: "include",
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          `${res.status} - ${data.error ? data.error : res.statusText}`
        );
      }

      return data;
    } catch (err) {
      console.error(err);
      throw new Error(" a network error occurred while fetching movie details");
    }
  },
});

function MovieDetailsPage() {
  const data = Route.useLoaderData();
  const { movie } = data();

  const audioOpts = createMemo(() => {
    if (!movie.audio_streams.length) return [];

    const opts: { value: number; label: string }[] = [];
    for (const stream of movie.audio_streams) {
      opts.push({
        value: stream.index,
        label: `${stream.title} ${stream.language}`,
      });
    }

    return opts;
  });

  const subtitleOpts = createMemo(() => {
    if (!movie.subtitles.length) return [];

    const opts: { value: number; label: string }[] = [];
    for (const sub of movie.subtitles) {
      opts.push({
        value: sub.index,
        label: `${sub.title} ${sub.language}`,
      });
    }

    return opts;
  });

  const [selectedVideo, setSelectedVideo] = createSignal<string | null>(null);
  const [isPlaybackModalOpen, setIsPlaybackModalOpen] = createSignal(false);

  const navigate = Route.useNavigate();

  const handlePlayMovie = async () => {
    try {
      const opts: MovieHlsOpts = {
        title: movie.title,
        audio_codec: "aac",
        audio_bit_rate: 192,
        audio_channels: 2,
        audio_stream_index: movie.audio_streams[0].index,
        video_stream_index: movie.video_streams[0].index,
        video_codec: "copy",
      };

      const res = await fetch(`/api/v1/ffmpeg/create-hls-movie/${movie.id}`, {
        method: "post",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(opts),
      });

      if (!res.ok) {
        alert("the request failed with status " + res.status);
        return;
      }

      const data = await res.json();

      if (data.error) {
        alert(`${res.status} - ${data.error ? data.error : res.statusText}`);
        return;
      }

      alert("about to navigate");

      navigate({
        to: "/movies/$movieID/play",
        from: Route.fullPath,
        params: {
          movieID: movie.id.toString(),
        },
        search: {
          title: movie.title,
          thumb: movie.thumb,
          pid: data.pid,
          m3u8Url: data.m3u8_url,
        },
      });
    } catch (err) {
      console.error(err);
      alert("an error occurred while playing the movie");
    }
  };

  return (
    <>
      {" "}
      {/* Hero Section with Backdrop */}
      <div class="relative min-h-[90vh] w-full">
        {/* Backdrop Image */}
        <div
          class="absolute inset-0 bg-cover bg-center"
          style={{ "background-image": `url(${getImgSrc(movie.art)})` }}
        >
          {/* Gradient overlays */}
          <div class="absolute inset-0 bg-gradient-to-r from-blue-950/80 via-blue-950/50 to-transparent" />
          <div class="absolute inset-0 bg-gradient-to-t from-blue-950/90 via-blue-950/40 to-transparent" />
        </div>

        {/* Content */}
        <div class="relative h-full container mx-auto px-4 py-12 flex flex-col justify-end">
          <div class="flex flex-col md:flex-row gap-8 items-end md:items-end">
            {/* Movie Poster */}
            <div class="relative z-10 md:translate-y-16">
              <div class="w-48 md:w-72 aspect-[2/3] rounded-lg overflow-hidden ring-1 ring-white/10 shadow-xl shadow-black/50">
                <img
                  src={getImgSrc(movie.thumb)}
                  alt={`${movie.title} Poster`}
                  class="w-full h-full object-cover"
                />
              </div>
            </div>

            {/* Movie Info */}
            <div class="flex-1 max-w-3xl relative z-10">
              <h1 class="text-4xl md:text-6xl font-bold text-white mb-4 drop-shadow-xl">
                {movie.title}
              </h1>
              {movie.tag_line && (
                <p class="text-xl text-blue-200 mb-4 italic drop-shadow-xl">
                  {movie.tag_line}
                </p>
              )}

              {/* Movie Meta */}
              <div class="flex flex-wrap gap-4 text-sm text-blue-200 mb-6 drop-shadow-lg bg-blue-950/20 backdrop-blur-sm rounded-lg px-4 py-2">
                <div class="flex items-center gap-1">
                  <FiCalendar class="w-4 h-4" aria-hidden="true" />
                  <span>{formatDate(movie.release_date)}</span>
                </div>

                <div class="flex items-center gap-1">
                  <FiClock class="w-4 h-4" aria-hidden="true" />
                  <span>{formatRuntime(movie.run_time)}</span>
                </div>
                {movie.content_rating && (
                  <span class="px-2 py-0.5 bg-yellow-300/20 rounded">
                    {movie.content_rating}
                  </span>
                )}

                <GenreList genres={movie.genres} />
              </div>

              {/* Ratings */}
              <div class="flex gap-6 mb-6">
                {movie.audience_rating > 0 && (
                  <div class="flex items-center gap-2">
                    <FiStar
                      class="w-5 h-5 text-yellow-300"
                      aria-hidden="true"
                    />
                    <div>
                      <div class="text-lg font-medium text-white">
                        {movie.audience_rating.toFixed(1)}
                      </div>
                      <div class="text-xs text-blue-200">Audience Rating</div>
                    </div>
                  </div>
                )}
                {movie.critic_rating > 0 && (
                  <div class="flex items-center gap-2">
                    <FiAward
                      class="w-5 h-5 text-yellow-300"
                      aria-hidden="true"
                    />
                    <div>
                      <div class="text-lg font-medium text-white">
                        {movie.critic_rating.toFixed(1)}
                      </div>
                      <div class="text-xs text-blue-200">Critic Rating</div>
                    </div>
                  </div>
                )}
              </div>

              {/* Summary */}
              <p class="text-blue-200 leading-relaxed mb-8">{movie.summary}</p>

              {/* Action Buttons */}
              <div class="flex flex-wrap items-center gap-4 mb-8">
                <button
                  type="button"
                  onClick={handlePlayMovie}
                  class="inline-flex items-center gap-2 px-6 py-3 bg-yellow-300 hover:bg-yellow-400 
                           text-blue-950 font-medium rounded-lg transition-colors"
                >
                  <FiPlay class="w-5 h-5" aria-hidden="true" />
                  Play
                </button>

                <button
                  type="button"
                  class="inline-flex items-center gap-2 px-4 py-3 bg-rose-500/20 hover:bg-rose-500/30 
                           text-rose-300 hover:text-rose-200 font-medium rounded-lg transition-colors"
                >
                  <FiHeart class="w-5 h-5" aria-hidden="true" />
                  Like
                </button>

                <button
                  type="button"
                  class="inline-flex items-center gap-2 px-4 py-3 bg-emerald-500/20 hover:bg-emerald-500/30 
                           text-emerald-300 hover:text-emerald-200 font-medium rounded-lg transition-colors"
                >
                  <FiCheck class="w-5 h-5" aria-hidden="true" />
                  Mark as Watched
                </button>

                {/* <Dropdown
                  trigger={
                    <button
                      type="button"
                      class="inline-flex items-center justify-center w-12 h-12 bg-slate-800/50 hover:bg-slate-800 
                               text-sky-200 hover:text-sky-100 rounded-lg transition-colors"
                    >
                      <FiMoreHorizontal
                        class="w-5 h-5"
                        aria-hidden="true"
                      />
                    </button>
                  }
                  label={`More options for ${movie.title}`}
                  items={[
                    {
                      label: "Playback Settings",
                      icon: <FiPlay class="w-4 h-4" />,
                      onClick: () => setIsPlaybackModalOpen(true),
                    },
                    {
                      label: "Movie Details",
                      icon: <FiInfo class="w-4 h-4" />,
                      onClick: () => {
                        console.log("Show details");
                      },
                    },
                  ]}
                /> */}
              </div>

              {/* Additional Info */}
              <div class="grid grid-cols-2 md:grid-cols-3 gap-6">
                {movie.budget > 0 && (
                  <div>
                    <div class="flex items-center gap-1 text-yellow-300 mb-1">
                      <FiDollarSign class="w-4 h-4" aria-hidden="true" />
                      <span class="text-sm font-medium">Budget</span>
                    </div>
                    <div class="text-white">{formatDollars(movie.budget)}</div>
                  </div>
                )}

                {movie.revenue > 0 && (
                  <div>
                    <div class="flex items-center gap-1 text-yellow-300 mb-1">
                      <FiDollarSign class="w-4 h-4" aria-hidden="true" />
                      <span class="text-sm font-medium">Revenue</span>
                    </div>
                    <div class="text-white">{formatDollars(movie.revenue)}</div>
                  </div>
                )}

                <StudioList studios={movie.studios} />
              </div>
            </div>
          </div>
        </div>
      </div>
      {/* Cast & Crew Section */}
      <section class="container mx-auto px-4 py-12">
        <h2 class="text-2xl font-bold text-yellow-300 mb-6">Cast & Crew</h2>
        {movie.cast.length > 0 && <CastSection cast={movie.cast} />}
        {movie.crew.length > 0 && <CrewSection crew={movie.crew} />}
      </section>
      {/* Extras Section */}
      <ExtrasSection
        extras={movie.extras}
        onSelectVideo={(url) => setSelectedVideo(url)}
      />
      {/* YouTube Modal */}
      <YoutubeModal
        isOpen={!!selectedVideo()}
        onClose={() => setSelectedVideo(null)}
        videoUrl={selectedVideo()}
      />
      <PlaybackSettingsModal
        isOpen={isPlaybackModalOpen()}
        onClose={() => setIsPlaybackModalOpen(false)}
        audioOpts={audioOpts()}
        subtitleOpts={subtitleOpts()}
      />
    </>
  );
}
