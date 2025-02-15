import ReactPlayer from "react-player";
import Hls from "hls.js";

type HlsPlayerProps = {
  url: string;
  useHlsJs: boolean;
  title?: string;
  playing?: boolean;
  onPlay?: () => void;
  onPause?: () => void;
  onEnded?: () => void;
  onError?: (error: Error) => void;
  onProgress?: (state: {
    played: number;
    playedSeconds: number;
    loaded: number;
    loadedSeconds: number;
  }) => void;
};

export default function HlsPlayer({
  url,
  useHlsJs,
  title,
  playing = false,
  onPlay,
  onPause,
  onEnded,
  onError,
  onProgress,
}: HlsPlayerProps) {
  return (
    <div className='relative w-full h-full bg-black rounded-lg overflow-hidden shadow-2xl'>
      {title && (
        <div className='absolute top-0 left-0 right-0 z-10 p-4 bg-gradient-to-b from-black/80 to-transparent'>
          <h1 className='text-white text-xl font-medium'>{title}</h1>
        </div>
      )}
      <div className='relative w-full h-full'>
        <ReactPlayer
          url={url}
          playing={playing}
          controls={true}
          width='100%'
          height='100%'
          className='absolute top-0 left-0'
          config={{
            file: {
              forceHLS: useHlsJs,
              attributes: {
                playsInline: true,
              },
              hlsOptions: useHlsJs
                ? {
                    enableWorker: true,
                    autoStartLoad: true,
                    startLevel: -1,
                    debug: false,
                    maxBufferLength: 30,
                    maxMaxBufferLength: 60,
                    maxBufferSize: 200 * 1000 * 1000,
                    maxBufferHole: 0.5,
                    lowLatencyMode: true,
                  }
                : undefined,
              hlsVersion: useHlsJs ? Hls.version : undefined,
            },
          }}
          onPlay={onPlay}
          onPause={onPause}
          onEnded={onEnded}
          onError={error => {
            console.error(
              `HLS Playback Error (${useHlsJs ? "HLS.js" : "Native"}):`,
              error
            );
            onError?.(error);
          }}
          onProgress={onProgress}
          progressInterval={1000}
        />
      </div>
    </div>
  );
}
