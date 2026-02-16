import type { RefObject } from "react";

type MovieVideoProps = {
  videoRef: RefObject<HTMLVideoElement | null>;
  src: string;
  title: string;
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
  onTimeUpdate,
  onDurationChange,
  onPlay,
  onPause,
  onError,
}: MovieVideoProps) {
  return (
    <div className="relative flex flex-1 items-center justify-center p-4">
      <div className="aspect-video w-full max-w-6xl">
        <video
          ref={videoRef}
          src={src}
          className="size-full rounded-lg bg-black"
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
