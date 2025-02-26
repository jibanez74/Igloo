import { useEffect } from "react";
import { createFileRoute } from "@tanstack/react-router";
import canPlayNativeHls from "../../../utils/canPlayNativeHls";
import HlsPlayer from "../../../components/HlsPlayer";

type PlayParams = {
  title: string;
  pid: string;
};

export const Route = createFileRoute("/movies/$movieID/play")({
  validateSearch: (search: Record<string, unknown>): PlayParams => ({
    title: String(search.title ?? "unknown"),
    pid: String(search.pid ?? "unknown"),
  }),
  loaderDeps: ({ search }) => search,
  component: PlayMoviePage,
});

function PlayMoviePage() {
  const { movieID } = Route.useParams();
  const { pid, title } = Route.useLoaderDeps();

  useEffect(() => {
    return () => {
      if (window.confirm("are you sure you wish to stop playback?")) {
        fetch(`/api/v1/ffmpeg/jobs/cancel/${pid}`, {
          method: "delete",
          credentials: "include",
        }).then(res => alert(`${res.status} - ${res.statusText}`));
      }
    };
  }, [pid]);

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
