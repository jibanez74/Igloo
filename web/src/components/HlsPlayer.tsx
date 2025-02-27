import { useEffect, useRef, useState, useCallback, memo } from "react";
import Hls, { ErrorTypes, Events, ErrorData } from "hls.js";
import { useMutation } from "@tanstack/react-query";

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

// Optional: Analytics and metadata functions
const logPlaybackAnalytics = async (event: { 
  url: string;
  type: 'play' | 'pause' | 'ended' | 'error';
  timestamp: number;
  error?: Error;
}) => {
  // Implementation for sending analytics
  console.log('Logging playback event:', event);
};

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
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [isStartTimeSet, setIsStartTimeSet] = useState(false);

  // Optional: Analytics mutation
  const { mutate: logEvent } = useMutation({
    mutationFn: logPlaybackAnalytics,
    onError: (error) => {
      console.warn('Failed to log playback event:', error);
    }
  });

  const handleLoadingStart = useCallback(() => setIsLoading(true), []);
  const handleLoadingEnd = useCallback(() => setIsLoading(false), []);

  // Reset start time state when URL changes
  useEffect(() => {
    setIsStartTimeSet(false);
  }, [url]);

  const setStartTimeAndPlay = useCallback(async () => {
    const video = videoRef.current;
    if (!video || isStartTimeSet) return;

    try {
      // Set start time
      video.currentTime = startTime;
      setIsStartTimeSet(true);
      
      // Only autoplay if playing prop is true
      if (playing) {
        await video.play();
      }
    } catch (error) {
      console.error('Failed to set start time or play:', error);
      setError(error instanceof Error ? error : new Error('Playback failed'));
    }
  }, [startTime, playing, isStartTimeSet]);

  const handleLoadedData = useCallback(() => {
    handleLoadingEnd();
    setStartTimeAndPlay();
  }, [handleLoadingEnd, setStartTimeAndPlay]);

  const handlePlay = useCallback(() => {
    // Ensure start time is set before allowing play
    if (!isStartTimeSet) {
      setStartTimeAndPlay();
      return;
    }

    logEvent({
      url,
      type: 'play',
      timestamp: Date.now()
    });
    
    onPlay?.();
  }, [url, logEvent, onPlay, isStartTimeSet, setStartTimeAndPlay]);

  const handleTimeUpdate = useCallback((e: React.SyntheticEvent<HTMLVideoElement>) => {
    const target = e.currentTarget;
    onProgress?.({
      played: target.currentTime / target.duration,
      playedSeconds: target.currentTime,
      loaded: target.buffered.length ? target.buffered.end(0) / target.duration : 0,
      loadedSeconds: target.buffered.length ? target.buffered.end(0) : 0,
    });
  }, [onProgress]);

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

  // Initialize HLS.js if needed
  useEffect(() => {
    if (!videoRef.current) return;

    // If we're using native HLS (Safari/iOS), just set the source
    if (!useHlsJs) {
      videoRef.current.src = url;
      return;
    }

    // Check if HLS.js is supported
    if (!Hls.isSupported()) {
      console.error('HLS.js is not supported');
      setError(new Error('HLS.js is not supported in this browser'));
      return;
    }

    // Clean up existing HLS instance
    if (hlsRef.current) {
      hlsRef.current.destroy();
    }

    const hls = new Hls(HLS_CONFIG);
    hlsRef.current = hls;

    hls.attachMedia(videoRef.current);

    const handleManifestParsed = () => {
      handleLoadingEnd();
      setStartTimeAndPlay();
    };

    const handleLevelLoaded = () => {
      const qualities = hls.levels.map(level => ({
        width: level.width,
        height: level.height,
        bitrate: level.bitrate,
      }));
      onQualitiesLoaded?.(qualities);
    };

    const handleError = (_event: Events.ERROR, data: ErrorData) => {
      if (data.fatal) {
        const error = new Error(`HLS.js Error: ${data.type}`);
        console.error(error, data);
        
        logEvent({
          url,
          type: 'error',
          timestamp: Date.now(),
          error
        });

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

    return () => {
      hls.off(Events.MANIFEST_PARSED, handleManifestParsed);
      hls.off(Events.LEVEL_LOADED, handleLevelLoaded);
      hls.off(Events.ERROR, handleError);
      hls.destroy();
    };
  }, [url, useHlsJs, handleLoadingEnd, setStartTimeAndPlay, onQualitiesLoaded, logEvent, onError]);

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);

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

  return (
    <div className='relative w-full h-full bg-black rounded-lg overflow-hidden shadow-2xl'>
      {title && <TitleOverlay title={title} />}
      {isLoading && <LoadingOverlay />}
      <video
        ref={videoRef}
        controls
        playsInline
        style={{ width: "100%", height: "100%" }}
        onPlay={handlePlay}
        onPause={onPause}
        onEnded={onEnded}
        onTimeUpdate={handleTimeUpdate}
        onLoadedData={handleLoadedData}
        onSeeking={handleLoadingStart}
        onSeeked={handleLoadingEnd}
        onWaiting={handleLoadingStart}
        onPlaying={handleLoadingEnd}
      />
    </div>
  );
}

export default memo(HlsPlayer);
