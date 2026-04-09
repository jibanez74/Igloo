import { useEffect, useRef } from "react";
import Hls from "hls.js";
import type { RefObject } from "react";
import {
  HLS_JS_BACK_BUFFER_LENGTH_SEC,
  HLS_JS_LOAD_TIMEOUT_MS,
  HLS_SESSION_LOST_MAX_ATTEMPTS,
  HLS_SESSION_LOST_MIN_INTERVAL_MS,
} from "@/lib/constants";
import {
  hlsStreamRecoveryKey,
  supportsNativeHLS,
} from "@/lib/playback";

type SubtitleTrackInfo = {
  url: string;
  label: string;
  srclang: string;
};

type VideoPlayerProps = {
  videoRef: RefObject<HTMLVideoElement | null>;
  src: string;
  title: string;
  isFullscreen?: boolean;
  onError: (message: string) => void;
  onPlay?: () => void;
  onPause?: () => void;
  onEnded?: () => void;
  onTimeUpdate?: (time: number) => void;
  onDurationChange?: (duration: number) => void;
  onNativeError?: (code: number | null | undefined) => void;
  subtitleTrack?: SubtitleTrackInfo | null;
  startSec?: number;
  onStartApplied?: (time: number) => void;
  onSessionLost?: (currentTime: number) => void;
};

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
  const sessionRecoveryKeyRef = useRef<string>("");
  const sessionLostAttemptsRef = useRef(0);
  const lastSessionLostAtRef = useRef(0);

  // Stable refs for callbacks so the main setup effect only re-runs when the
  // stream URL or start position actually changes, not on every parent render.
  const onErrorRef = useRef(onError);
  const onSessionLostRef = useRef(onSessionLost);
  const onStartAppliedRef = useRef(onStartApplied);
  useEffect(() => {
    onErrorRef.current = onError;
    onSessionLostRef.current = onSessionLost;
    onStartAppliedRef.current = onStartApplied;
  }, [onError, onSessionLost, onStartApplied]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;

    if (
      (src.endsWith(".m3u8") || src.includes(".m3u8?")) &&
      Hls.isSupported() &&
      !supportsNativeHLS
    ) {
      const recoveryKey = hlsStreamRecoveryKey(src);
      if (sessionRecoveryKeyRef.current !== recoveryKey) {
        sessionRecoveryKeyRef.current = recoveryKey;
        sessionLostAttemptsRef.current = 0;
        lastSessionLostAtRef.current = 0;
      }

      const hls = new Hls({
        xhrSetup(xhr) {
          xhr.withCredentials = true;
        },
        manifestLoadingTimeOut: HLS_JS_LOAD_TIMEOUT_MS,
        levelLoadingTimeOut: HLS_JS_LOAD_TIMEOUT_MS,
        fragLoadingTimeOut: HLS_JS_LOAD_TIMEOUT_MS,
        backBufferLength: HLS_JS_BACK_BUFFER_LENGTH_SEC,
        startPosition: startSec > 0 ? startSec : -1,
      });
      hlsRef.current = hls;

      hls.loadSource(src);
      hls.attachMedia(video);

      if (startSec > 0) {
        hls.once(Hls.Events.MANIFEST_PARSED, () => {
          onStartAppliedRef.current?.(startSec);
        });
      }

      let mediaRecoveryAttempted = false;
      hls.on(Hls.Events.ERROR, (_, data) => {
        if (
          data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR &&
          data.response?.code === 404 &&
          onSessionLostRef.current
        ) {
          const now = Date.now();
          if (sessionLostAttemptsRef.current >= HLS_SESSION_LOST_MAX_ATTEMPTS) {
            onErrorRef.current(
              "Playback session could not be recovered. Try reloading the page or choosing another quality.",
            );
            return;
          }
          const tooSoon =
            sessionLostAttemptsRef.current > 0 &&
            now - lastSessionLostAtRef.current <
              HLS_SESSION_LOST_MIN_INTERVAL_MS;
          if (tooSoon) {
            return;
          }
          sessionLostAttemptsRef.current += 1;
          lastSessionLostAtRef.current = now;
          onSessionLostRef.current(video.currentTime);
          return;
        }

        if (!data.fatal) return;

        if (data.type === "mediaError" && !mediaRecoveryAttempted) {
          mediaRecoveryAttempted = true;
          hls.recoverMediaError();
          return;
        }

        const detail = data.details ?? "unknown error";
        if (data.type === "networkError") {
          onErrorRef.current(`Network error loading stream (${detail}).`);
        } else if (data.type === "mediaError") {
          onErrorRef.current(
            `The browser could not decode this stream (${detail}).`,
          );
        } else {
          onErrorRef.current(`Stream error: ${detail}`);
        }
      });

      return () => {
        hls.destroy();
        hlsRef.current = null;
      };
    }

    video.src = src;
    return () => {
      video.removeAttribute("src");
      video.load();
    };
  }, [src, videoRef]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || startSec <= 0) return;
    // HLS.js owns the initial seek via its startPosition config and fires
    // onStartApplied via MANIFEST_PARSED; don't compete with it.
    if (hlsRef.current) return;

    const applyStart = () => {
      const duration = Number.isFinite(video.duration) ? video.duration : 0;
      const nextTime =
        duration > 0 ? Math.min(startSec, duration) : startSec;
      video.currentTime = nextTime;
      onStartAppliedRef.current?.(nextTime);
    };

    if (video.readyState >= 1) {
      applyStart();
      return;
    }

    video.addEventListener("loadedmetadata", applyStart, { once: true });
    return () => {
      video.removeEventListener("loadedmetadata", applyStart);
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
    track.default = true;
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
          onError={
            onNativeError
              ? (e) => onNativeError(e.currentTarget.error?.code)
              : undefined
          }
        />
      </div>
    </div>
  );
}
