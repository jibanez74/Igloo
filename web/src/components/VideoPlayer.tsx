import { useEffect, useEffectEvent, useRef } from "react";
import type Hls from "hls.js";
import type { ErrorData } from "hls.js";
import {
  HLS_JS_BACK_BUFFER_LENGTH_SEC,
  HLS_JS_LOAD_TIMEOUT_MS,
} from "@/lib/constants";
import { supportsNativeHLS } from "@/lib/playback";
import type { VideoPlayerProps } from "@/types";

function loadHlsLight() {
  return import("hls.js/light");
}

export default function VideoPlayer({
  videoRef,
  src,
  title,
  isFullscreen = false,
  onError,
  onPlay,
  onPause,
  onEnded,
  onTimeUpdate,
  onDurationChange,
  onNativeError,
  subtitleTrack = null,
  startSec = 0,
  onStartApplied,
  onSessionLost,
}: VideoPlayerProps) {
  const hlsRef = useRef<Hls | null>(null);

  const reportError = useEffectEvent((message: string) => {
    onError(message);
  });

  const handleStartApplied = useEffectEvent((time: number) => {
    onStartApplied?.(time);
  });

  const applyStartTime = useEffectEvent((video: HTMLVideoElement) => {
    const duration = Number.isFinite(video.duration) ? video.duration : 0;
    const nextTime = duration > 0 ? Math.min(startSec, duration) : startSec;
    video.currentTime = nextTime;
    handleStartApplied(nextTime);
  });

  const handleHlsError = useEffectEvent(
    (
      video: HTMLVideoElement,
      data: ErrorData,
      sessionLostDetail: string,
    ) => {
      // Rate limiting and the max-attempt budget live in useHlsSessionRecovery
      // (the onSessionLost consumer); this component only reports the event.
      if (
        data.details === sessionLostDetail &&
        data.response?.code === 404 &&
        onSessionLost
      ) {
        onSessionLost(video.currentTime);
        return;
      }

      const detail = data.details ?? "unknown error";
      if (data.type === "networkError") {
        reportError(`Network error loading stream (${detail}).`);
      } else if (data.type === "mediaError") {
        reportError(`The browser could not decode this stream (${detail}).`);
      } else {
        reportError(`Stream error: ${detail}`);
      }
    },
  );

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;

    if (
      (src.endsWith(".m3u8") || src.includes(".m3u8?")) &&
      !supportsNativeHLS
    ) {
      let cancelled = false;
      let disposeHls: (() => void) | null = null;

      void (async () => {
        try {
          const { default: Hls } = await loadHlsLight();
          if (cancelled || !Hls.isSupported()) return;

          const hls = new Hls({
            xhrSetup(xhr: XMLHttpRequest) {
              xhr.withCredentials = true;
            },
            manifestLoadingTimeOut: HLS_JS_LOAD_TIMEOUT_MS,
            levelLoadingTimeOut: HLS_JS_LOAD_TIMEOUT_MS,
            fragLoadingTimeOut: HLS_JS_LOAD_TIMEOUT_MS,
            backBufferLength: HLS_JS_BACK_BUFFER_LENGTH_SEC,
            startPosition: startSec > 0 ? startSec : -1,
          });
          const sessionLostDetail = Hls.ErrorDetails.FRAG_LOAD_ERROR;
          hlsRef.current = hls;
          disposeHls = () => {
            hls.destroy();
            hlsRef.current = null;
          };

          hls.loadSource(src);
          hls.attachMedia(video);

          if (startSec > 0) {
            hls.once(Hls.Events.MANIFEST_PARSED, () => {
              handleStartApplied(startSec);
            });
          }

          let mediaRecoveryAttempted = false;
          hls.on(Hls.Events.ERROR, (_event, data: ErrorData) => {
            const isSessionLostError =
              data.details === sessionLostDetail &&
              data.response?.code === 404;

            if (isSessionLostError) {
              handleHlsError(video, data, sessionLostDetail);
              return;
            }

            if (!data.fatal) return;

            if (data.type === "mediaError" && !mediaRecoveryAttempted) {
              mediaRecoveryAttempted = true;
              hls.recoverMediaError();
              return;
            }

            handleHlsError(video, data, sessionLostDetail);
          });
        } catch {
          if (!cancelled) {
            reportError("Failed to load the video playback engine.");
          }
        }
      })();

      return () => {
        cancelled = true;
        disposeHls?.();
      };
    }

    video.src = src;
    return () => {
      video.removeAttribute("src");
      video.load();
    };
  }, [src, startSec, videoRef]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || startSec <= 0) return;
    // HLS.js owns the initial seek via its startPosition config and fires
    // onStartApplied via MANIFEST_PARSED; don't compete with it.
    if (hlsRef.current) return;

    if (video.readyState >= 1) {
      applyStartTime(video);
      return;
    }

    const handleLoadedMetadata = () => {
      applyStartTime(video);
    };

    video.addEventListener("loadedmetadata", handleLoadedMetadata, { once: true });
    return () => {
      video.removeEventListener("loadedmetadata", handleLoadedMetadata);
    };
  }, [startSec, src, videoRef]);

  // The subtitleTrack object gets a new reference on every parent render;
  // key on the URL which uniquely identifies the active subtitle so the
  // effect only re-runs when the subtitle actually changes.
  const subtitleUrl = subtitleTrack?.url ?? null;
  const subtitleTrackRef = useRef(subtitleTrack);
  useEffect(() => {
    subtitleTrackRef.current = subtitleTrack;
  }, [subtitleTrack]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const existing = video.querySelector("track[data-subtitle]");
    if (existing) {
      video.removeChild(existing);
    }

    const sub = subtitleTrackRef.current;
    if (!sub) return;

    const track = document.createElement("track");
    track.kind = "subtitles";
    track.src = sub.url;
    track.srclang = sub.srclang;
    track.label = sub.label;
    track.setAttribute("data-subtitle", "");
    video.appendChild(track);
    track.track.mode = "showing";

    return () => {
      if (video.contains(track)) {
        video.removeChild(track);
      }
    };
  }, [subtitleUrl, videoRef]);

  return (
    <div
      className={
        isFullscreen
          ? "relative flex min-h-0 w-full flex-1 items-center justify-center bg-black"
          : "relative flex min-h-0 w-full flex-1 items-center justify-center p-4"
      }
    >
      <div
        className={
          isFullscreen
            ? "size-full min-h-0 min-w-0"
            : "aspect-video w-full max-w-6xl"
        }
      >
        <video
          ref={videoRef}
          className={`size-full bg-black object-contain ${isFullscreen ? "rounded-none" : "rounded-lg"}`}
          playsInline
          aria-label={`Video player for ${title}`}
          onPlay={onPlay}
          onPause={onPause}
          onEnded={onEnded}
          onTimeUpdate={
            onTimeUpdate
              ? (e) => onTimeUpdate(e.currentTarget.currentTime)
              : undefined
          }
          onDurationChange={
            onDurationChange
              ? (e) => onDurationChange(e.currentTarget.duration)
              : undefined
          }
          onError={(e) => {
            const errorCode = e.currentTarget.error?.code;

            onNativeError?.(errorCode);

            switch (errorCode) {
              case MediaError.MEDIA_ERR_ABORTED:
                onError(
                  "Playback was interrupted before the stream finished loading.",
                );
                return;
              case MediaError.MEDIA_ERR_NETWORK:
                onError("A network error interrupted video playback.");
                return;
              case MediaError.MEDIA_ERR_DECODE:
                onError("The browser could not decode this video stream.");
                return;
              case MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED:
                onError(
                  "This video format or stream is not supported by the browser.",
                );
                return;
              default:
                onError(
                  "Playback failed — the browser could not play this stream.",
                );
            }
          }}
        />
      </div>
    </div>
  );
}
