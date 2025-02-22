import { createFileRoute } from "@tanstack/react-router";
import canPlayNativeHls from "@/utils/canPlayNativeHls";
import HlsPlayer from "@/components/HlsPlayer";
import type { MovieHlsOpts } from "@/types/Transcode";

export const Route = createFileRoute("/movies/$movieID/play")({
  validateSearch: (search: Record<string, unknown>): MovieHlsOpts => ({
    title: String(search.title ?? "unknown"),
    audio_codec: String(search.audio_codec),
    audio_bit_rate: Number(search.audio_bit_rate),
    audio_channels: Number(search.audio_channels),
    audio_stream_index: Number(search.audio_stream_index),
    video_codec: String(search.video_codec),
    video_stream_index: Number(search.video_stream_index),
    video_bit_rate: Number(search.video_bit_rate),
    video_height: Number(search.video_height),
    video_profile: String(search.video_profile),
    preset: String(search.preset),
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ deps, params }) => {
    const res = await fetch(
      `/api/v1/ffmpeg/movie/create-hls/${params.movieID}`,
      {
        method: "post",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(deps),
      }
    );

    const data = await res.json();

    if (data.error) {
      throw data.error;
    }

    return {
      pid: data?.pid,
      movieID: params.movieID,
      title: deps.title,
    };
  },
  component: PlayMoviePage,
});

function PlayMoviePage() {
  const { movieID, title, pid } = Route.useLoaderData();

  console.log(pid);

  return (
    <div className='min-h-screen bg-slate-900 flex flex-col'>
      {/* Player Container */}
      <div className='flex-1 flex items-center justify-center p-4 md:p-8'>
        <div className='w-full max-w-7xl aspect-video'>
          <HlsPlayer
            url={`/api/v1/hls/movies/${movieID}/playlist.m3u8`}
            useHlsJs={canPlayNativeHls()}
            title={title}
            startTime={300} // Start at 5 minutes (300 seconds)
            onError={error => {
              console.error("Playback Error:", error);
            }}
          />
        </div>
      </div>
    </div>
  );
}
