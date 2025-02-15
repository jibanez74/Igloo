import { createFileRoute } from "@tanstack/react-router";
import { useState, useEffect } from "react";
import HlsPlayer from "@/components/HlsPlayer";
import Hls from "hls.js";

type PlayParams = {
  title: string;
  thumb: string;
};

export const Route = createFileRoute("/movies/$movieID/play")({
  validateSearch: (search: Record<string, unknown>): PlayParams => ({
    title: String(search.title),
    thumb: String(search.thumb),
  }),
  loaderDeps: ({ search }) => search,
  component: PlayMoviePage,
});

function PlayMoviePage() {
  const { movieID } = Route.useParams();
  const { title } = Route.useSearch<PlayParams>();
  const [canUseHls, setCanUseHls] = useState(false);

  useEffect(() => {
    // Check if HLS.js is supported
    setCanUseHls(Hls.isSupported());
  }, []);

  return (
    <div className='min-h-screen bg-slate-900 flex flex-col'>
      {/* Player Container */}
      <div className='flex-1 flex items-center justify-center p-4 md:p-8'>
        <div className='w-full max-w-7xl aspect-video'>
          <HlsPlayer
            url={`/api/v1/movies/${movieID}/hls/master.m3u8`}
            useHlsJs={canUseHls}
            title={title}
            onError={error => {
              console.error("Playback Error:", error);
            }}
          />
        </div>
      </div>
    </div>
  );
}
