import { useEffect, useCallback, useMemo, useRef } from "react";
import Hls from "hls.js";
import type { RefObject } from "react";

type VideoPlayerProps = {
  videoRef: RefObject<HTMLVideoElement | null>;
  src: string;
  title: string;
  isFullscreen?: boolean;
  onError: () => void;
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
}: VideoPlayerProps) {
  const hlsRef = useRef<Hls | null>(null);

  const stableOnError = useCallback(onError, []);

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
      });
      hlsRef.current = hls;

      hls.loadSource(src);
      hls.attachMedia(video);

      hls.on(Hls.Events.ERROR, (_, data) => {
        if (data.fatal) stableOnError();
      });

      return () => {
        hls.destroy();
        hlsRef.current = null;
      };
    }

    // Native HLS (Safari) or direct stream
    video.src = src;
    return () => {
      video.removeAttribute("src");
      video.load();
    };
  }, [src, videoRef, stableOnError]);

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
