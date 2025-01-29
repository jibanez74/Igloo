import ReactPlayer from "react-player";
import Hls from "hls.js";

type HlsPlayerProps = {
  url: string;
  useHlsJs: boolean;
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
  playing = false,
  onPlay,
  onPause,
  onEnded,
  onError,
  onProgress,
}: HlsPlayerProps) {
  return (
    <div className='w-full bg-black'>
      <div className='relative w-full pt-[56.25%]'>
        <div className='absolute inset-0'>
          <ReactPlayer
            url={url}
            playing={playing}
            controls={true}
            width='100%'
            height='100%'
            style={{ position: "absolute", top: 0, left: 0 }}
            config={{
              file: {
                forceHLS: useHlsJs,
                attributes: {
                  crossOrigin: "anonymous",
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
    </div>
  );
}
