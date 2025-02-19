import ReactPlayer from "react-player";
import Hls from "hls.js";
import { useEffect, useRef, useState } from "react";

type Quality = {
  width: number;
  height: number;
  bitrate: number;
};

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
  onQualitiesLoaded?: (qualities: Quality[]) => void;
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
  onQualitiesLoaded,
}: HlsPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const hlsRef = useRef<Hls | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (!useHlsJs || !videoRef.current) return;

    if (hlsRef.current) {
      hlsRef.current.destroy();
    }

    const hls = new Hls({
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
    });

    hls.attachMedia(videoRef.current);

    hls.on(Hls.Events.MEDIA_ATTACHED, () => {
      hls.loadSource(url);
    });

    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      setIsLoading(false);
      if (playing) {
        videoRef.current?.play();
      }
    });

    hls.on(Hls.Events.LEVEL_LOADED, () => {
      const qualities = hls.levels.map(level => ({
        width: level.width,
        height: level.height,
        bitrate: level.bitrate,
      }));
      onQualitiesLoaded?.(qualities);
    });

    hls.on(Hls.Events.ERROR, (_event, data) => {
      if (data.fatal) {
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            console.error("Network error:", data);
            hls.startLoad();
            break;
          case Hls.ErrorTypes.MEDIA_ERROR:
            console.error("Media error:", data);
            hls.recoverMediaError();
            break;
          default:
            console.error("Fatal error:", data);
            hls.destroy();
            break;
        }
        onError?.(new Error(`HLS.js Error: ${data.type}`));
      }
    });

    hlsRef.current = hls;

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
      }
    };
  }, [url, useHlsJs, onError, playing, onQualitiesLoaded]);

  const handleKeyDown = (e: KeyboardEvent) => {
    const video = videoRef.current;
    if (!video) return;

    switch (e.key.toLowerCase()) {
      case " ":
        e.preventDefault();
        if (video.paused) {
          video.play();
        } else {
          video.pause();
        }
        break;
      case "arrowleft":
        e.preventDefault();
        video.currentTime = Math.max(0, video.currentTime - 10);
        break;
      case "arrowright":
        e.preventDefault();
        video.currentTime = Math.min(video.duration, video.currentTime + 10);
        break;
      case "arrowup":
        e.preventDefault();
        video.volume = Math.min(1, video.volume + 0.1); // Increase volume by 10%
        break;
      case "arrowdown":
        e.preventDefault();
        video.volume = Math.max(0, video.volume - 0.1); // Decrease volume by 10%
        break;
      case "m":
        e.preventDefault();
        video.muted = !video.muted;
        break;
      case "f":
        e.preventDefault();
        if (document.fullscreenElement) {
          document.exitFullscreen();
        } else {
          video.requestFullscreen();
        }
        break;
    }
  };

  useEffect(() => {
    if (useHlsJs) {
      window.addEventListener("keydown", handleKeyDown);
      return () => window.removeEventListener("keydown", handleKeyDown);
    }
  }, [useHlsJs]);

  if (!useHlsJs) {
    return (
      <div className='relative w-full h-full bg-black rounded-lg overflow-hidden shadow-2xl'>
        {title && (
          <div className='absolute top-0 left-0 right-0 z-10 p-4 bg-gradient-to-b from-black/80 to-transparent'>
            <h1 className='text-white text-xl font-medium'>{title}</h1>
          </div>
        )}
        <ReactPlayer
          url={url}
          playing={playing}
          controls={true}
          width='100%'
          height='100%'
          className='absolute top-0 left-0'
          onPlay={onPlay}
          onPause={onPause}
          onEnded={onEnded}
          onError={onError}
          onProgress={onProgress}
          progressInterval={1000}
        />
      </div>
    );
  }

  return (
    <div className='relative w-full h-full bg-black rounded-lg overflow-hidden shadow-2xl'>
      {title && (
        <div className='absolute top-0 left-0 right-0 z-10 p-4 bg-gradient-to-b from-black/80 to-transparent'>
          <h1 className='text-white text-xl font-medium'>{title}</h1>
        </div>
      )}
      {isLoading && (
        <div className='absolute inset-0 flex items-center justify-center bg-slate-900/80'>
          <div className='text-sky-200'>Loading...</div>
        </div>
      )}
      <video
        ref={videoRef}
        controls
        playsInline
        autoPlay={playing}
        style={{ width: "100%", height: "100%" }}
        onPlay={onPlay}
        onPause={onPause}
        onEnded={onEnded}
        onTimeUpdate={e => {
          onProgress?.({
            played: e.currentTarget.currentTime / e.currentTarget.duration,
            playedSeconds: e.currentTarget.currentTime,
            loaded: e.currentTarget.buffered.length
              ? e.currentTarget.buffered.end(0) / e.currentTarget.duration
              : 0,
            loadedSeconds: e.currentTarget.buffered.length
              ? e.currentTarget.buffered.end(0)
              : 0,
          });
        }}
        onSeeking={() => setIsLoading(true)}
        onSeeked={() => setIsLoading(false)}
        onWaiting={() => setIsLoading(true)}
        onPlaying={() => setIsLoading(false)}
        onLoadedData={() => setIsLoading(false)}
      />
    </div>
  );
}
