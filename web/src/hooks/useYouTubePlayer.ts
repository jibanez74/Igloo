import {
  useRef,
  useState,
  useEffect,
  useId,
  useEffectEvent,
  useCallback,
} from "react";
import type {
  UseYouTubePlayerOptions,
  UseYouTubePlayerReturn,
} from "@/types";

const YOUTUBE_IFRAME_API_SRC = "https://www.youtube.com/iframe_api";
const YOUTUBE_API_LOAD_TIMEOUT_MS = 15000;
const YOUTUBE_PLAYER_READY_TIMEOUT_MS = 12000;
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

type YouTubePlayerState = {
  playerIdentity: string;
  isReady: boolean;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isMuted: boolean;
  error: string | null;
};

function getYouTubePlayerIdentity(
  videoId: string | null,
  autoplay: boolean,
  controls: boolean,
  reloadKey: number,
) {
  return `${videoId ?? "none"}:${autoplay ? "autoplay" : "manual"}:${controls ? "controls" : "chromeless"}:${reloadKey}`;
}

function createInitialPlayerState(playerIdentity: string): YouTubePlayerState {
  return {
    playerIdentity,
    isReady: false,
    isPlaying: false,
    currentTime: 0,
    duration: 0,
    volume: 100,
    isMuted: false,
    error: null,
  };
}

export function useYouTubePlayer(
  options: UseYouTubePlayerOptions,
): UseYouTubePlayerReturn {
  const { videoId, autoplay = true, controls = true } = options;

  const containerRef = useRef<HTMLDivElement | null>(null);
  // Boolean state mirrors whether the container node is attached, so the init
  // effect re-runs the moment the DOM node mounts (the node lives in a ref so we
  // can mutate it directly; state only exists to re-trigger the effect).
  const [containerReady, setContainerReady] = useState(false);
  const setContainerRef = useCallback((node: HTMLDivElement | null) => {
    containerRef.current = node;
    setContainerReady(node != null);
  }, []);
  const playerRef = useRef<YT.Player | null>(null);
  const uniqueId = useId();
  const playerIdRef = useRef(`yt-player-${uniqueId.replace(/:/g, "")}`);
  const progressIntervalRef = useRef<number | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const playerIdentity = getYouTubePlayerIdentity(
    videoId,
    autoplay,
    controls,
    reloadKey,
  );
  const [playerState, setPlayerState] = useState(() =>
    createInitialPlayerState(playerIdentity),
  );

  let currentPlayerState = playerState;
  if (playerState.playerIdentity !== playerIdentity) {
    currentPlayerState = createInitialPlayerState(playerIdentity);
    setPlayerState(currentPlayerState);
  }

  const {
    isReady,
    isPlaying,
    currentTime,
    duration,
    volume,
    isMuted,
    error,
  } = currentPlayerState;

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

    setPlayerState(previous => ({
      ...previous,
      currentTime: nextTime,
      duration: nextDuration > 0 ? nextDuration : previous.duration,
    }));
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
    setPlayerState(previous => ({
      ...previous,
      isReady: true,
      error: null,
      duration: event.target.getDuration() || 0,
      volume: event.target.getVolume(),
      isMuted: event.target.isMuted(),
    }));
    options.onReady?.();
  });

  const handlePlayerStateChange = useEffectEvent(
    (event: YT.OnStateChangeEvent) => {
      const state = event.data;

      setPlayerState(previous => ({
        ...previous,
        isPlaying: state === window.YT.PlayerState.PLAYING,
      }));

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

  const clearPlayerRef = useEffectEvent(() => {
    playerRef.current = null;
  });

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
    setPlayerState(previous => ({
      ...previous,
      isPlaying: false,
      error: errorMessage,
    }));
    options.onError?.(errorCode);
  });

  useEffect(() => {
    if (!videoId || !containerReady || !containerRef.current) {
      stopProgressTracking();
      return;
    }

    let mounted = true;
    let createdPlayer: YT.Player | null = null;
    let readyTimeoutId: number | null = null;

    const clearReadyTimeout = () => {
      if (readyTimeoutId !== null) {
        window.clearTimeout(readyTimeoutId);
        readyTimeoutId = null;
      }
    };

    const initPlayer = async () => {
      try {
        await loadYouTubeAPI();
      } catch (apiError) {
        if (!mounted) {
          return;
        }

        console.error("Failed to load YouTube API:", apiError);
        setPlayerState(previous => ({
          ...previous,
          error: "Failed to load the YouTube player.",
        }));
        return;
      }

      if (!mounted || !containerRef.current) return;

      // Include reloadKey so a retry mounts a fresh element for the new player.
      const elementId = `${playerIdRef.current}-${reloadKey}`;
      const playerDiv = document.createElement("div");
      playerDiv.id = elementId;
      containerRef.current.innerHTML = "";
      containerRef.current.appendChild(playerDiv);

      try {
        const player = new window.YT.Player(elementId, {
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
              clearReadyTimeout();
              handlePlayerReady(event);
            },
            onStateChange: event => {
              if (!mounted) return;
              handlePlayerStateChange(event);
            },
            onError: event => {
              if (!mounted) return;
              clearReadyTimeout();
              handlePlayerError(event);
            },
          },
        });

        createdPlayer = player;
        playerRef.current = player;

        // Watchdog: if the embed never reaches `onReady` (e.g. the YouTube
        // iframe is blocked by an ad blocker or a network issue), surface an
        // actionable error instead of an indefinite loading spinner.
        readyTimeoutId = window.setTimeout(() => {
          if (!mounted) return;
          setPlayerState(previous =>
            previous.isReady || previous.error
              ? previous
              : {
                  ...previous,
                  error:
                    "The video player took too long to load. It may be blocked by an ad blocker or browser extension.",
                },
          );
        }, YOUTUBE_PLAYER_READY_TIMEOUT_MS);
      } catch (creationError) {
        console.error("Failed to create YouTube player:", creationError);
        setPlayerState(previous => ({
          ...previous,
          error: "Failed to load video player.",
        }));
      }
    };

    void initPlayer();

    return () => {
      mounted = false;
      clearReadyTimeout();
      stopProgressTracking();

      if (createdPlayer) {
        try {
          createdPlayer.destroy();
        } catch {
          // Player might already be destroyed.
        }
      }

      clearPlayerRef();
    };
  }, [videoId, autoplay, controls, containerReady, reloadKey]);

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

    const nextTime = Number.isFinite(seconds) ? seconds : 0;
    const playerDuration = getCurrentPlayerDuration(player);
    const clampedTime =
      playerDuration > 0 ? Math.min(playerDuration, Math.max(0, nextTime)) : Math.max(0, nextTime);

    player.seekTo(clampedTime, true);
    setPlayerState(previous => ({
      ...previous,
      currentTime: clampedTime,
    }));
  };

  const seekForward = (seconds: number) => {
    const player = playerRef.current;
    if (!player) {
      return;
    }

    const now = getCurrentPlayerTime(player);
    const totalDuration = Math.max(duration, getCurrentPlayerDuration(player));
    const newTime = Math.min(totalDuration, now + seconds);
    seekTo(newTime);
  };

  const seekBackward = (seconds: number) => {
    const player = playerRef.current;
    if (!player) {
      return;
    }

    const now = getCurrentPlayerTime(player);
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
    let nextIsMuted = currentPlayerState.isMuted;

    if (clampedVol > 0 && getPlayerMuted(player)) {
      player.unMute();
      nextIsMuted = false;
    }

    setPlayerState(previous => ({
      ...previous,
      volume: clampedVol,
      isMuted: nextIsMuted,
    }));
  };

  const mute = () => {
    const player = playerRef.current;
    if (!player) {
      return;
    }

    player.mute();
    setPlayerState(previous => ({
      ...previous,
      isMuted: true,
    }));
  };

  const unmute = () => {
    const player = playerRef.current;
    if (!player) {
      return;
    }

    player.unMute();
    setPlayerState(previous => ({
      ...previous,
      isMuted: false,
    }));
  };

  const toggleMute = () => {
    const player = playerRef.current;
    if (!player) {
      return;
    }

    if (getPlayerMuted(player)) {
      player.unMute();
      setPlayerState(previous => ({
        ...previous,
        isMuted: false,
      }));
      return;
    }

    player.mute();
    setPlayerState(previous => ({
      ...previous,
      isMuted: true,
    }));
  };

  const retry = () => {
    setReloadKey(previous => previous + 1);
  };

  return {
    containerRef: setContainerRef,
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
    retry,
  };
}
