import ReactPlayer from "react-player/file";
import { useState } from "react";

interface HlsPlayerProps {
  movieId: number;
  onPlay?: () => void;
  onPause?: () => void;
  onError?: (error: Error) => void;
  onBuffer?: () => void;
  onBufferEnd?: () => void;
}

export default function HlsPlayer({
  movieId,
  onPlay,
  onPause,
  onError,
  onBuffer,
  onBufferEnd,
}: HlsPlayerProps) {
  const [playing, setPlaying] = useState(false);
  const [buffering, setBuffering] = useState(false);

  const handlePlay = () => {
    setPlaying(true);
    onPlay?.();
  };

  const handlePause = () => {
    setPlaying(false);
    onPause?.();
  };

  const handleBuffer = () => {
    setBuffering(true);
    onBuffer?.();
  };

  const handleBufferEnd = () => {
    setBuffering(false);
    onBufferEnd?.();
  };

  const handleError = (error: Error) => {
    console.error("HLS Player error:", error);
    onError?.(error);
  };

  return (
    <div className='player-wrapper'>
      <ReactPlayer
        url={`/api/v1/movies/${movieId}/stream/master.m3u8`}
        className='react-player'
        width='100%'
        height='100%'
        playing={playing}
        controls={true}
        onPlay={handlePlay}
        onPause={handlePause}
        onError={handleError}
        onBuffer={handleBuffer}
        onBufferEnd={handleBufferEnd}
        config={{
          file: {
            forceHLS: true,
            hlsOptions: {
              maxBufferLength: 30,
              maxMaxBufferLength: 60,
            },
          },
        }}
      />
      <style jsx>{`
        .player-wrapper {
          position: relative;
          padding-top: 56.25%; /* 16:9 aspect ratio */
        }
        .react-player {
          position: absolute;
          top: 0;
          left: 0;
        }
      `}</style>
    </div>
  );
}
