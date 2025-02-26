import { useEffect, useRef, useState, useCallback, memo } from "react";
import ReactPlayer from "react-player";
import Hls, { ErrorTypes, Events, ErrorData } from "hls.js";

const HLS_CONFIG = {
  debug: import.meta.env.DEV,
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
} as const;

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
  startTime?: number;
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

const TitleOverlay = memo(({ title }: { title: string }) => (
  <div className='absolute top-0 left-0 right-0 z-10 p-4 bg-gradient-to-b from-black/80 to-transparent'>
    <h1 className='text-white text-xl font-medium'>{title}</h1>
  </div>
));

TitleOverlay.displayName = 'TitleOverlay';

const LoadingOverlay = memo(() => (
  <div className='absolute inset-0 flex items-center justify-center bg-slate-900/80'>
    <div className='text-sky-200'>Loading...</div>
  </div>
));

LoadingOverlay.displayName = 'LoadingOverlay';

function HlsPlayer({
  url,
  useHlsJs,
  title,
  playing = false,
  startTime = 0,
  onPlay,
  onPause,
  onEnded,
  onError,
  onProgress,
  onQualitiesLoaded,
}: HlsPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const hlsRef = useRef<Hls | null>(null);
  const hasPlayedOnceRef = useRef(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const handleTimeUpdate = useCallback((e: React.SyntheticEvent<HTMLVideoElement>) => {
    const target = e.currentTarget;
    onProgress?.({
      played: target.currentTime / target.duration,
      playedSeconds: target.currentTime,
      loaded: target.buffered.length ? target.buffered.end(0) / target.duration : 0,
      loadedSeconds: target.buffered.length ? target.buffered.end(0) : 0,
    });
  }, [onProgress]);

  const handlePlay = useCallback(() => {
    if (videoRef.current && !hasPlayedOnceRef.current) {
      videoRef.current.currentTime = startTime;
      hasPlayedOnceRef.current = true;
    }
    onPlay?.();
  }, [startTime, onPlay]);

  const handleLoadingStart = useCallback(() => setIsLoading(true), []);
  const handleLoadingEnd = useCallback(() => setIsLoading(false), []);

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
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
        video.volume = Math.min(1, video.volume + 0.1);
        break;
      case "arrowdown":
        e.preventDefault();
        video.volume = Math.max(0, video.volume - 0.1);
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
  }, []);

  useEffect(() => {
    if (!useHlsJs || !videoRef.current) return;

    if (hlsRef.current) {
      hlsRef.current.destroy();
    }

    const hls = new Hls(HLS_CONFIG);

    hls.attachMedia(videoRef.current);

    const handleManifestParsed = () => {
      handleLoadingEnd();
      if (playing) {
        videoRef.current?.play().catch(error => {
          console.error('Failed to play video:', error);
          setError(error);
        });
      }
    };

    const handleLevelLoaded = () => {
      const qualities = hls.levels.map(level => ({
        width: level.width,
        height: level.height,
        bitrate: level.bitrate,
      }));
      onQualitiesLoaded?.(qualities);
    };

    const handleError = (_event: unknown, data: ErrorData) => {
      if (data.fatal) {
        const error = new Error(`HLS.js Error: ${data.type}`);
        console.error(error, data);
        
        switch (data.type) {
          case ErrorTypes.NETWORK_ERROR:
            hls.startLoad();
            break;
          case ErrorTypes.MEDIA_ERROR:
            hls.recoverMediaError();
            break;
          default:
            hls.destroy();
            setError(error);
            break;
        }
        onError?.(error);
      }
    };

    hls.on(Events.MEDIA_ATTACHED, () => {
      hls.loadSource(url);
    });

    hls.on(Events.MANIFEST_PARSED, handleManifestParsed);
    hls.on(Events.LEVEL_LOADED, handleLevelLoaded);
    hls.on(Events.ERROR, handleError);

    hlsRef.current = hls;

    return () => {
      hls.off(Events.MANIFEST_PARSED, handleManifestParsed);
      hls.off(Events.LEVEL_LOADED, handleLevelLoaded);
      hls.off(Events.ERROR, handleError);
      hls.destroy();
    };
  }, [url, useHlsJs, onError, playing, onQualitiesLoaded, handleLoadingEnd]);

  useEffect(() => {
    if (useHlsJs) {
      window.addEventListener("keydown", handleKeyDown);
      return () => window.removeEventListener("keydown", handleKeyDown);
    }
  }, [useHlsJs, handleKeyDown]);

  useEffect(() => {
    hasPlayedOnceRef.current = false;
  }, [url]);

  if (error) {
    return (
      <div className="relative w-full h-full flex items-center justify-center bg-slate-900">
        <div className="text-red-500">
          <h3 className="font-medium">Error playing video</h3>
          <p className="text-sm opacity-75">{error.message}</p>
        </div>
      </div>
    );
  }

  if (!useHlsJs) {
    return (
      <div className='relative w-full h-full bg-black rounded-lg overflow-hidden shadow-2xl'>
        {title && <TitleOverlay title={title} />}
        <ReactPlayer
          url={url}
          playing={playing}
          controls={true}
          width='100%'
          height='100%'
          className='absolute top-0 left-0'
          onPlay={handlePlay}
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
      {title && <TitleOverlay title={title} />}
      {isLoading && <LoadingOverlay />}
      <video
        ref={videoRef}
        controls
        playsInline
        autoPlay={playing}
        style={{ width: "100%", height: "100%" }}
        onPlay={handlePlay}
        onPause={onPause}
        onEnded={onEnded}
        onTimeUpdate={handleTimeUpdate}
        onSeeking={handleLoadingStart}
        onSeeked={handleLoadingEnd}
        onWaiting={handleLoadingStart}
        onPlaying={handleLoadingEnd}
        onLoadedData={handleLoadingEnd}
      />
    </div>
  );
}

export default memo(HlsPlayer);
