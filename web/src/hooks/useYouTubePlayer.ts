import { useRef, useState, useEffect, useId, useEffectEvent } from "react";

const YOUTUBE_IFRAME_API_SRC = "https://www.youtube.com/iframe_api";
const YOUTUBE_API_LOAD_TIMEOUT_MS = 15000;

let apiLoaded = false;
let apiLoading = false;
let apiLoadError: Error | null = null;
const apiReadyResolvers: Array<() => void> = [];
const apiReadyRejectors: Array<(error: Error) => void> = [];

function resolveApiLoad() {
  apiLoaded = true;
  apiLoading = false;
  apiLoadError = null;

  for (const resolve of apiReadyResolvers.splice(0)) {
    resolve();
  }
  apiReadyRejectors.length = 0;
}

function rejectApiLoad(error: Error) {
  apiLoaded = false;
  apiLoading = false;
  apiLoadError = error;

  for (const reject of apiReadyRejectors.splice(0)) {
    reject(error);
  }
  apiReadyResolvers.length = 0;
}

function loadYouTubeAPI(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (apiLoaded && window.YT?.Player) {
      resolve();
      return;
    }

    if (apiLoadError) {
      reject(apiLoadError);
      return;
    }

    if (apiLoading) {
      apiReadyResolvers.push(resolve);
      apiReadyRejectors.push(reject);
      return;
    }

    apiLoading = true;
    apiReadyResolvers.push(resolve);
    apiReadyRejectors.push(reject);

    const timeoutId = window.setTimeout(() => {
      rejectApiLoad(new Error("Timed out loading the YouTube player API."));
    }, YOUTUBE_API_LOAD_TIMEOUT_MS);

    window.onYouTubeIframeAPIReady = () => {
      window.clearTimeout(timeoutId);
      resolveApiLoad();
    };

    const handleScriptError = () => {
      window.clearTimeout(timeoutId);
      rejectApiLoad(new Error("Failed to load the YouTube player API."));
    };

    const existingScript = document.querySelector<HTMLScriptElement>(
      `script[src="${YOUTUBE_IFRAME_API_SRC}"]`,
    );

    if (existingScript) {
      existingScript.addEventListener("error", handleScriptError, {
        once: true,
      });
      return;
    }

    const script = document.createElement("script");
    script.src = YOUTUBE_IFRAME_API_SRC;
    script.async = true;
    script.addEventListener("error", handleScriptError, { once: true });
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

function getCurrentPlayerTime(player: YT.Player | null) {
  if (!player) return 0;

  try {
    return player.getCurrentTime() || 0;
  } catch {
    return 0;
  }
}

function getCurrentPlayerDuration(player: YT.Player | null) {
  if (!player) return 0;

  try {
    return player.getDuration() || 0;
  } catch {
    return 0;
  }
}

function getPlayerState(player: YT.Player | null) {
  if (!player) return null;

  try {
    return player.getPlayerState();
  } catch {
    return null;
  }
}

function getPlayerMuted(player: YT.Player | null) {
  if (!player) return false;

  try {
    return player.isMuted();
  } catch {
    return false;
  }
}

export function useYouTubePlayer(
  options: UseYouTubePlayerOptions,
): UseYouTubePlayerReturn {
  const { videoId, autoplay = true, controls = true } = options;

  const containerRef = useRef<HTMLDivElement | null>(null);
  const playerRef = useRef<YT.Player | null>(null);
  const uniqueId = useId();
  const playerIdRef = useRef(`yt-player-${uniqueId.replace(/:/g, "")}`);
  const progressIntervalRef = useRef<number | null>(null);

  const [isReady, setIsReady] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolumeState] = useState(100);
  const [isMuted, setIsMuted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const stopProgressTracking = useEffectEvent(() => {
    if (progressIntervalRef.current) {
      window.clearInterval(progressIntervalRef.current);
      progressIntervalRef.current = null;
    }
  });

  const syncProgressState = useEffectEvent(() => {
    const player = playerRef.current;
    if (!player) {
      return;
    }

    const nextTime = getCurrentPlayerTime(player);
    const nextDuration = getCurrentPlayerDuration(player);

    setCurrentTime(nextTime);
    if (nextDuration > 0) {
      setDuration(nextDuration);
    }
  });

  const startProgressTracking = useEffectEvent(() => {
    if (progressIntervalRef.current) {
      return;
    }

    progressIntervalRef.current = window.setInterval(() => {
      syncProgressState();
    }, 250);
  });

  const handlePlayerReady = useEffectEvent((event: YT.PlayerEvent) => {
    setIsReady(true);
    setError(null);
    setDuration(event.target.getDuration() || 0);
    setVolumeState(event.target.getVolume());
    setIsMuted(event.target.isMuted());
    options.onReady?.();
  });

  const handlePlayerStateChange = useEffectEvent(
    (event: YT.OnStateChangeEvent) => {
      const state = event.data;

      setIsPlaying(state === window.YT.PlayerState.PLAYING);

      if (state === window.YT.PlayerState.PLAYING) {
        startProgressTracking();
        syncProgressState();
      } else {
        stopProgressTracking();
      }

      if (state === window.YT.PlayerState.ENDED) {
        options.onEnd?.();
      }

      options.onStateChange?.(state);
    },
  );

  const handlePlayerError = useEffectEvent((event: YT.OnErrorEvent) => {
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

    stopProgressTracking();
    setIsPlaying(false);
    setError(errorMessage);
    options.onError?.(errorCode);
  });

  useEffect(() => {
    if (!videoId || !containerRef.current) {
      stopProgressTracking();
      return;
    }

    let mounted = true;

    // eslint-disable-next-line react-hooks/set-state-in-effect -- Resetting state synchronously on video change is intentional for this external integration
    setIsReady(false);
    setIsPlaying(false);
    setError(null);
    setCurrentTime(0);
    setDuration(0);

    const initPlayer = async () => {
      try {
        await loadYouTubeAPI();
      } catch (apiError) {
        if (!mounted) {
          return;
        }

        console.error("Failed to load YouTube API:", apiError);
        setError("Failed to load the YouTube player.");
        return;
      }

      if (!mounted || !containerRef.current) return;

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
            onReady: event => {
              if (!mounted) return;
              handlePlayerReady(event);
            },
            onStateChange: event => {
              if (!mounted) return;
              handlePlayerStateChange(event);
            },
            onError: event => {
              if (!mounted) return;
              handlePlayerError(event);
            },
          },
        });
      } catch (creationError) {
        console.error("Failed to create YouTube player:", creationError);
        setError("Failed to load video player.");
      }
    };

    void initPlayer();

    return () => {
      mounted = false;
      stopProgressTracking();
      setIsPlaying(false);

      if (playerRef.current) {
        try {
          playerRef.current.destroy();
        } catch {
          // Player might already be destroyed.
        }
        playerRef.current = null;
      }
    };
  }, [videoId, autoplay, controls]);

  const play = () => {
    playerRef.current?.playVideo();
  };

  const pause = () => {
    playerRef.current?.pauseVideo();
  };

  const togglePlay = () => {
    const player = playerRef.current;
    const playerState = getPlayerState(player);

    if (playerState === window.YT.PlayerState.PLAYING) {
      player?.pauseVideo();
      return;
    }

    player?.playVideo();
  };

  const seekTo = (seconds: number) => {
    const player = playerRef.current;
    if (!player) {
      return;
    }

    player.seekTo(seconds, true);
    setCurrentTime(seconds);
  };

  const seekForward = (seconds: number) => {
    const player = playerRef.current;
    const now = getCurrentPlayerTime(player);
    const totalDuration = Math.max(duration, getCurrentPlayerDuration(player));
    const newTime = Math.min(totalDuration, now + seconds);
    seekTo(newTime);
  };

  const seekBackward = (seconds: number) => {
    const now = getCurrentPlayerTime(playerRef.current);
    const newTime = Math.max(0, now - seconds);
    seekTo(newTime);
  };

  const setVolume = (vol: number) => {
    const clampedVol = Math.max(0, Math.min(100, vol));
    const player = playerRef.current;

    player?.setVolume(clampedVol);
    setVolumeState(clampedVol);

    if (clampedVol > 0 && getPlayerMuted(player)) {
      player?.unMute();
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
    const player = playerRef.current;

    if (getPlayerMuted(player)) {
      player?.unMute();
      setIsMuted(false);
      return;
    }

    player?.mute();
    setIsMuted(true);
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
