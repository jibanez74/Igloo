import { useState } from "react";
import ReactPlayer from "react-player/file";
import Spinner from "react-bootstrap/Spinner";

type HlsPlayerProps = {
  url: string;
  title?: string;
  poster?: string;
  useHlsJs: boolean;
  onError?: (error: Error) => void;
  onProgress?: (state: {
    played: number;
    playedSeconds: number;
    loaded: number;
    loadedSeconds: number;
  }) => void;
  onDuration?: (duration: number) => void;
  onEnded?: () => void;
};

export default function HlsPlayer({
  url,
  title,
  poster,
  useHlsJs,
  onError,
  onProgress,
  onDuration,
  onEnded,
}: HlsPlayerProps) {
  const [isPlaying, setIsPlaying] = useState(false);
  const [isBuffering, setIsBuffering] = useState(false);

  return (
    <div className='player-wrapper'>
      <ReactPlayer
        url={url}
        playing={isPlaying}
        controls={true}
        width='100%'
        height='100%'
        style={{ position: "absolute", top: 0, left: 0 }}
        config={{
          forceHLS: useHlsJs,
          hlsOptions: useHlsJs
            ? {
                enableWorker: true,
                lowLatencyMode: true,
                backBufferLength: 90,
              }
            : undefined,
          attributes: {
            poster: poster,
            preload: "auto",
            controlsList: "nodownload",
            title: title,
          },
        }}
        onBuffer={() => setIsBuffering(true)}
        onBufferEnd={() => setIsBuffering(false)}
        onError={(error: Error) => {
          console.error("HLS Player Error:", error);
          onError?.(error);
        }}
        onPlay={() => setIsPlaying(true)}
        onPause={() => setIsPlaying(false)}
        onEnded={onEnded}
        onProgress={onProgress}
        onDuration={onDuration}
        progressInterval={1000}
      />
      {isBuffering && (
        <div className='buffering-overlay'>
          <Spinner animation='border' variant='light' role='status'>
            <span className='visually-hidden'>Loading...</span>
          </Spinner>
        </div>
      )}
      <style>
        {`
          .player-wrapper {
            position: relative;
            padding-top: 56.25%; /* 16:9 Aspect Ratio */
          }
          .buffering-overlay {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: rgba(0, 0, 0, 0.5);
            z-index: 1;
          }
        `}
      </style>
    </div>
  );
}
