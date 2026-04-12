import { useRef, useState, useEffect, useId, useEffectEvent } from "react";

const YOUTUBE_IFRAME_API_SRC = "https://www.youtube.com/iframe_api";
const YOUTUBE_API_LOAD_TIMEOUT_MS = 15000;
const YOUTUBE_IFRAME_API_ERROR_ATTR = "data-yt-api-load-error";

let apiLoading = false;
let apiLoadError: Error | null = null;
let apiLoadTimeoutId: number | null = null;
let cleanupApiScriptListeners: (() => void) | null = null;
const apiReadyResolvers: Array<() => void> = [];
const apiReadyRejectors: Array<(error: Error) => void> = [];

function isYouTubeApiReady() {
  return typeof window !== "undefined" && Boolean(window.YT?.Player);
}

function clearApiLoadTimeout() {
  if (apiLoadTimeoutId !== null) {
    window.clearTimeout(apiLoadTimeoutId);
    apiLoadTimeoutId = null;
  }
}

function cleanupPendingApiLoad() {
  clearApiLoadTimeout();
  cleanupApiScriptListeners?.();
  cleanupApiScriptListeners = null;
}

function resolveApiLoad() {
  cleanupPendingApiLoad();
  apiLoading = false;
  apiLoadError = null;

  const resolvers = apiReadyResolvers.splice(0);
  apiReadyRejectors.length = 0;

  for (const resolve of resolvers) {
    resolve();
  }
}

function rejectApiLoad(error: Error) {
  cleanupPendingApiLoad();
  apiLoading = false;
  apiLoadError = error;

  const rejectors = apiReadyRejectors.splice(0);
  apiReadyResolvers.length = 0;

  for (const reject of rejectors) {
    reject(error);
  }
}

function markYouTubeApiScriptFailed(script: HTMLScriptElement | null) {
  script?.setAttribute(YOUTUBE_IFRAME_API_ERROR_ATTR, "true");
}

function shouldReplaceYouTubeApiScript(script: HTMLScriptElement) {
  const scriptReadyState =
    "readyState" in script ? script.readyState : undefined;

  return (
    script.getAttribute(YOUTUBE_IFRAME_API_ERROR_ATTR) === "true" ||
    (scriptReadyState === "complete" && !isYouTubeApiReady()) ||
    (scriptReadyState === "loaded" && !isYouTubeApiReady())
  );
}

function loadYouTubeAPI(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (isYouTubeApiReady()) {
      resolveApiLoad();
      resolve();
      return;
    }

    if (apiLoading) {
      apiReadyResolvers.push(resolve);
      apiReadyRejectors.push(reject);
      return;
    }

    if (apiLoadError) {
      apiLoadError = null;
    }

    apiLoading = true;
    apiReadyResolvers.push(resolve);
    apiReadyRejectors.push(reject);

    cleanupPendingApiLoad();

    let activeScript = document.querySelector<HTMLScriptElement>(
      `script[src="${YOUTUBE_IFRAME_API_SRC}"]`,
    );

    apiLoadTimeoutId = window.setTimeout(() => {
      markYouTubeApiScriptFailed(activeScript);
      rejectApiLoad(new Error("Timed out loading the YouTube player API."));
    }, YOUTUBE_API_LOAD_TIMEOUT_MS);

    window.onYouTubeIframeAPIReady = () => {
      resolveApiLoad();
    };

    const handleScriptError = () => {
      markYouTubeApiScriptFailed(activeScript);
      rejectApiLoad(new Error("Failed to load the YouTube player API."));
    };

    const handleScriptLoad = () => {
      if (isYouTubeApiReady()) {
        resolveApiLoad();
      }
    };

    if (activeScript && shouldReplaceYouTubeApiScript(activeScript)) {
      cleanupPendingApiLoad();
      activeScript.remove();
      activeScript = null;

      apiLoadTimeoutId = window.setTimeout(() => {
        rejectApiLoad(new Error("Timed out loading the YouTube player API."));
      }, YOUTUBE_API_LOAD_TIMEOUT_MS);
    }

    if (apiLoadError) {
      apiLoadError = null;
    }

    if (!activeScript) {
      activeScript = document.createElement("script");
      activeScript.src = YOUTUBE_IFRAME_API_SRC;
      activeScript.async = true;
      document.body.appendChild(activeScript);
    }

    activeScript.removeAttribute(YOUTUBE_IFRAME_API_ERROR_ATTR);
    activeScript.addEventListener("load", handleScriptLoad, { once: true });
    activeScript.addEventListener("error", handleScriptError, { once: true });

    cleanupApiScriptListeners = () => {
      activeScript?.removeEventListener("load", handleScriptLoad);
      activeScript?.removeEventListener("error", handleScriptError);
    };
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
    const playerStateEnum = window.YT?.PlayerState;
    if (!player || !playerStateEnum) {
      return;
    }

    const playerState = getPlayerState(player);
    if (playerState === null) {
      return;
    }

    if (playerState === playerStateEnum.PLAYING) {
      player.pauseVideo();
      return;
    }

    player.playVideo();
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
    if (!player) {
      return;
    }

    player.setVolume(clampedVol);
    setVolumeState(clampedVol);

    if (clampedVol > 0 && getPlayerMuted(player)) {
      player.unMute();
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
    if (!player) {
      return;
    }

    if (getPlayerMuted(player)) {
      player.unMute();
      setIsMuted(false);
      return;
    }

    player.mute();
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
