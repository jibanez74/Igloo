import ReactPlayer from "react-player/youtube";

type YoutubePlayerProps = {
  url: string | null;
  playing?: boolean;
  onPlay?: () => void;
  onPause?: () => void;
  onEnded?: () => void;
  onError?: (error: Error) => void;
};

export default function YoutubePlayer({
  url,
  playing = false,
  onPlay,
  onPause,
  onEnded,
  onError,
}: YoutubePlayerProps) {
  return (
    <div className='w-100'>
      <div className='ratio ratio-16x9'>
        <ReactPlayer
          url={url || ""}
          playing={playing}
          controls={true}
          width='100%'
          height='100%'
          config={{
            playerVars: {
              autoplay: 1,
              modestbranding: 1,
              rel: 0,
              origin: window.location.origin,
              enablejsapi: 1,
              playsinline: 1,
            },
          }}
          onPlay={onPlay}
          onPause={onPause}
          onEnded={onEnded}
          onError={onError}
        />
      </div>
    </div>
  );
}
