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
                    debug: true,
                    enableWorker: true,
                    autoStartLoad: true,
                    startLevel: -1,
                    manifestLoadPolicy: {
                      default: {
                        maxTimeToFirstByteMs: 10000,
                        maxLoadTimeMs: 10000,
                        timeoutRetry: {
                          maxNumRetry: 3,
                          retryDelayMs: 1000,
                          maxRetryDelayMs: 0,
                        },
                        errorRetry: {
                          maxNumRetry: 3,
                          retryDelayMs: 1000,
                          maxRetryDelayMs: 8000,
                        },
                      },
                    },
                    playlistLoadPolicy: {
                      default: {
                        maxTimeToFirstByteMs: 10000,
                        maxLoadTimeMs: 10000,
                        timeoutRetry: {
                          maxNumRetry: 2,
                          retryDelayMs: 0,
                          maxRetryDelayMs: 0,
                        },
                        errorRetry: {
                          maxNumRetry: 2,
                          retryDelayMs: 1000,
                          maxRetryDelayMs: 8000,
                        },
                      },
                    },
                    fragLoadPolicy: {
                      default: {
                        maxTimeToFirstByteMs: 20000,
                        maxLoadTimeMs: 20000,
                        timeoutRetry: {
                          maxNumRetry: 4,
                          retryDelayMs: 0,
                          maxRetryDelayMs: 0,
                        },
                        errorRetry: {
                          maxNumRetry: 6,
                          retryDelayMs: 1000,
                          maxRetryDelayMs: 8000,
                        },
                      },
                    },
                    maxBufferSize: 0,
                    maxBufferLength: 30,
                    maxMaxBufferLength: 600,
                    progressive: false,
                    lowLatencyMode: false,
                    enableSoftwareAES: true,
                    abrEwmaDefaultEstimate: 500000,
                    backBufferLength: 90,
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
