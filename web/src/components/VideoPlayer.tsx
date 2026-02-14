import type { RefObject } from "react";

type MovieVideoProps = {
  videoRef: RefObject<HTMLVideoElement | null>;
  src: string;
  title: string;
  isFullscreen?: boolean;
  onTimeUpdate: () => void;
  onDurationChange: () => void;
  onPlay: () => void;
  onPause: () => void;
  onError: () => void;
};

export default function VideoPlayer({
  videoRef,
  src,
  title,
  isFullscreen = false,
  onTimeUpdate,
  onDurationChange,
  onPlay,
  onPause,
  onError,
}: MovieVideoProps) {
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
          src={src}
          className={`size-full bg-black object-contain ${isFullscreen ? "rounded-none" : "rounded-lg"}`}
          playsInline
          aria-label={`Video player for ${title}`}
          onTimeUpdate={onTimeUpdate}
          onDurationChange={onDurationChange}
          onPlay={onPlay}
          onPause={onPause}
          onWaiting={() => {}}
          onError={onError}
        />
      </div>
    </div>
  );
}
