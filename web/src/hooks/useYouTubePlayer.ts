import { useRef, useState, useEffect, useId, useLayoutEffect } from "react";
import "@/types/youtube.d.ts";

// Singleton to track API loading state
let apiLoaded = false;
let apiLoading = false;
const apiReadyCallbacks: (() => void)[] = [];

/**
 * Load the YouTube IFrame API script (singleton pattern)
 */
function loadYouTubeAPI(): Promise<void> {
  return new Promise((resolve) => {
    // Already loaded
    if (apiLoaded && window.YT?.Player) {
      resolve();
      return;
    }

    // Currently loading - queue the callback
    if (apiLoading) {
      apiReadyCallbacks.push(resolve);
      return;
    }

    // Start loading
    apiLoading = true;
    apiReadyCallbacks.push(resolve);

    // Set up the global callback
    window.onYouTubeIframeAPIReady = () => {
      apiLoaded = true;
      apiLoading = false;
      apiReadyCallbacks.forEach((cb) => cb());
      apiReadyCallbacks.length = 0;
    };

    // Create and append the script tag
    const script = document.createElement("script");
    script.src = "https://www.youtube.com/iframe_api";
    script.async = true;
    document.body.appendChild(script);
  });
}

export type UseYouTubePlayerOptions = {
  videoId: string | null;
  autoplay?: boolean;
  controls?: boolean;
  onReady?: () => void;
  onStateChange?: (state: YT.PlayerState) => void;
  onError?: (error: YT.PlayerError) => void;
  onEnd?: () => void;
};

export type UseYouTubePlayerReturn = {
  containerRef: React.RefObject<HTMLDivElement | null>;
  isReady: boolean;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isMuted: boolean;
  error: string | null;
  play: () => void;
  pause: () => void;
  togglePlay: () => void;
  seekTo: (seconds: number) => void;
  seekForward: (seconds: number) => void;
  seekBackward: (seconds: number) => void;
  setVolume: (volume: number) => void;
  mute: () => void;
  unmute: () => void;
  toggleMute: () => void;
};

/**
 * Hook for integrating YouTube IFrame Player API
 */
export function useYouTubePlayer(
  options: UseYouTubePlayerOptions
): UseYouTubePlayerReturn {
  const { videoId, autoplay = true, controls = true } = options;

  const containerRef = useRef<HTMLDivElement | null>(null);
  const playerRef = useRef<YT.Player | null>(null);
  const uniqueId = useId();
  const playerIdRef = useRef<string>(`yt-player-${uniqueId.replace(/:/g, "")}`);
  const progressIntervalRef = useRef<number | null>(null);

  // Store callbacks in refs to avoid recreating player when they change
  const callbacksRef = useRef(options);
  useLayoutEffect(() => {
    callbacksRef.current = options;
  });

  const [isReady, setIsReady] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolumeState] = useState(100);
  const [isMuted, setIsMuted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Start progress tracking interval
  const startProgressTracking = () => {
    if (progressIntervalRef.current) return;

    progressIntervalRef.current = window.setInterval(() => {
      if (playerRef.current) {
        try {
          setCurrentTime(playerRef.current.getCurrentTime() || 0);
          const dur = playerRef.current.getDuration();
          if (dur > 0) {
            setDuration(dur);
          }
        } catch {
          // Player might be destroyed
        }
      }
    }, 250);
  };

  // Stop progress tracking interval
  const stopProgressTracking = () => {
    if (progressIntervalRef.current) {
      clearInterval(progressIntervalRef.current);
      progressIntervalRef.current = null;
    }
  };

  // Initialize player - only depends on videoId, autoplay, controls
  useEffect(() => {
    if (!videoId || !containerRef.current) return;

    let mounted = true;

    // Reset state for new video
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Resetting state synchronously on video change is intentional for this external integration
    setIsReady(false);
    setError(null);
    setCurrentTime(0);
    setDuration(0);

    const initPlayer = async () => {
      await loadYouTubeAPI();

      if (!mounted || !containerRef.current) return;

      // Create a div for the player inside the container
      const playerDiv = document.createElement("div");
      playerDiv.id = playerIdRef.current;
      containerRef.current.innerHTML = "";
      containerRef.current.appendChild(playerDiv);

      try {
        playerRef.current = new window.YT.Player(playerIdRef.current, {
          videoId,
          width: "100%",
          height: "100%",
          playerVars: {
            autoplay: autoplay ? 1 : 0,
            controls: controls ? 1 : 0,
            modestbranding: 1,
            rel: 0,
            playsinline: 1,
            enablejsapi: 1,
            origin: window.location.origin,
          },
          events: {
            onReady: (event) => {
              if (!mounted) return;
              setIsReady(true);
              setDuration(event.target.getDuration() || 0);
              setVolumeState(event.target.getVolume());
              setIsMuted(event.target.isMuted());
              callbacksRef.current.onReady?.();
            },
            onStateChange: (event) => {
              if (!mounted) return;
              const state = event.data;

              setIsPlaying(state === window.YT.PlayerState.PLAYING);

              if (state === window.YT.PlayerState.PLAYING) {
                startProgressTracking();
              } else {
                stopProgressTracking();
              }

              if (state === window.YT.PlayerState.ENDED) {
                callbacksRef.current.onEnd?.();
              }

              callbacksRef.current.onStateChange?.(state);
            },
            onError: (event) => {
              if (!mounted) return;
              const errorCode = event.data;
              let errorMessage = "Unable to play video.";

              switch (errorCode) {
                case window.YT.PlayerError.INVALID_PARAM:
                  errorMessage = "Invalid video ID.";
                  break;
                case window.YT.PlayerError.HTML5_ERROR:
                  errorMessage = "Video playback error.";
                  break;
                case window.YT.PlayerError.NOT_FOUND:
                  errorMessage = "Video not found.";
                  break;
                case window.YT.PlayerError.NOT_ALLOWED:
                case window.YT.PlayerError.NOT_ALLOWED_DISGUISE:
                  errorMessage = "Video is not available for embedding.";
                  break;
              }

              setError(errorMessage);
              callbacksRef.current.onError?.(errorCode);
            },
          },
        });
      } catch (e) {
        console.error("Failed to create YouTube player:", e);
        setError("Failed to load video player.");
      }
    };

    initPlayer();

    return () => {
      mounted = false;
      stopProgressTracking();
      if (playerRef.current) {
        try {
          playerRef.current.destroy();
        } catch {
          // Player might already be destroyed
        }
        playerRef.current = null;
      }
    };
  }, [videoId, autoplay, controls]);

  // Player control methods
  const play = () => {
    playerRef.current?.playVideo();
  };

  const pause = () => {
    playerRef.current?.pauseVideo();
  };

  const togglePlay = () => {
    if (isPlaying) {
      pause();
    } else {
      play();
    }
  };

  const seekTo = (seconds: number) => {
    playerRef.current?.seekTo(seconds, true);
    setCurrentTime(seconds);
  };

  const seekForward = (seconds: number) => {
    const newTime = Math.min(duration, currentTime + seconds);
    seekTo(newTime);
  };

  const seekBackward = (seconds: number) => {
    const newTime = Math.max(0, currentTime - seconds);
    seekTo(newTime);
  };

  const setVolume = (vol: number) => {
    const clampedVol = Math.max(0, Math.min(100, vol));
    playerRef.current?.setVolume(clampedVol);
    setVolumeState(clampedVol);
    if (clampedVol > 0 && isMuted) {
      playerRef.current?.unMute();
      setIsMuted(false);
    }
  };

  const mute = () => {
    playerRef.current?.mute();
    setIsMuted(true);
  };

  const unmute = () => {
    playerRef.current?.unMute();
    setIsMuted(false);
  };

  const toggleMute = () => {
    if (isMuted) {
      unmute();
    } else {
      mute();
    }
  };

  return {
    containerRef,
    isReady,
    isPlaying,
    currentTime,
    duration,
    volume,
    isMuted,
    error,
    play,
    pause,
    togglePlay,
    seekTo,
    seekForward,
    seekBackward,
    setVolume,
    mute,
    unmute,
    toggleMute,
  };
}
