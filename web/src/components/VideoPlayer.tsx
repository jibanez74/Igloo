import { useEffect, useRef } from "react";
import Hls from "hls.js";
import type { RefObject } from "react";

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
  subtitleTrack?: SubtitleTrackInfo | null;
  startSec?: number;
  onSessionLost?: (currentTime: number) => void;
};

function isHLSUrl(url: string): boolean {
  return url.endsWith(".m3u8") || url.includes(".m3u8?");
}

const supportsNativeHLS = (() => {
  if (typeof document === "undefined") return false;
  const v = document.createElement("video");
  return (
    v.canPlayType("application/vnd.apple.mpegurl") !== "" ||
    v.canPlayType("application/x-mpegURL") !== ""
  );
})();

export default function VideoPlayer({
  videoRef,
  src,
  title,
  isFullscreen = false,
  onError,
  subtitleTrack = null,
  startSec = 0,
  onSessionLost,
}: VideoPlayerProps) {
  const hlsRef = useRef<Hls | null>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;

    if (isHLSUrl(src) && Hls.isSupported() && !supportsNativeHLS) {
      const hls = new Hls({
        xhrSetup(xhr) {
          xhr.withCredentials = true;
        },
        manifestLoadingTimeOut: 120_000,
        levelLoadingTimeOut: 120_000,
        fragLoadingTimeOut: 120_000,
        backBufferLength: 30,
      });
      hlsRef.current = hls;

      hls.loadSource(src);
      hls.attachMedia(video);

      if (startSec > 0) {
        hls.once(Hls.Events.MANIFEST_PARSED, () => {
          video.currentTime = startSec;
        });
      }

      let mediaRecoveryAttempted = false;
      hls.on(Hls.Events.ERROR, (_, data) => {
        if (
          data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR &&
          data.response?.code === 404 &&
          onSessionLost
        ) {
          onSessionLost(video.currentTime);
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
          onError(`Network error loading stream (${detail}).`);
        } else if (data.type === "mediaError") {
          onError(`The browser could not decode this stream (${detail}).`);
        } else {
          onError(`Stream error: ${detail}`);
        }
      });

      return () => {
        hls.destroy();
        hlsRef.current = null;
      };
    }

    // Native HLS (Safari) or direct stream
    video.src = src;
    if (startSec > 0) {
      video.addEventListener(
        "loadedmetadata",
        () => {
          video.currentTime = startSec;
        },
        { once: true },
      );
    }
    return () => {
      video.removeAttribute("src");
      video.load();
    };
  }, [src, videoRef, onError, startSec, onSessionLost]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const existing = video.querySelector("track[data-subtitle]");
    if (existing) {
      video.removeChild(existing);
    }

    if (!subtitleTrack) return;

    const track = document.createElement("track");
    track.kind = "subtitles";
    track.src = subtitleTrack.url;
    track.srclang = subtitleTrack.srclang;
    track.label = subtitleTrack.label;
    track.default = true;
    track.setAttribute("data-subtitle", "");
    video.appendChild(track);
    track.track.mode = "showing";

    return () => {
      if (video.contains(track)) {
        video.removeChild(track);
      }
    };
  }, [subtitleTrack, videoRef]);

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
        />
      </div>
    </div>
  );
}
