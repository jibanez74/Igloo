import { useState, useRef } from "react";
import { createFileRoute, useSearch } from "@tanstack/react-router";
import { z } from "zod";
import ReactPlayer from "react-player";
import {
  FaPlay,
  FaPause,
  FaVolumeUp,
  FaVolumeMute,
  FaExpand,
  FaCompress,
} from "react-icons/fa";

const searchSchema = z.object({
  filePath: z.string(),
  directPlay: z.boolean().optional(),
  videoCodec: z.string().optional(),
  videoBitRate: z.string().optional(),
  videoHeight: z.number(),
  audioCodec: z.string().optional(),
  audioBitRate: z.string().optional(),
  audioChannels: z.string().optional(),
  container: z.string().optional(),
});

export const Route = createFileRoute("/movies/play")({
  component: PlayMoviePage,
  validateSearch: searchSchema.parse,
});

function PlayMoviePage() {
  const search = useSearch({ from: "/movies/play" });
  const [isPlaying, setIsPlaying] = useState(false);
  const [isMuted, setIsMuted] = useState(false);
  const [volume, setVolume] = useState(0.5);
  const [played, setPlayed] = useState(0);
  const [seeking, setSeeking] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const playerRef = useRef<ReactPlayer>(null);
  const playerContainerRef = useRef<HTMLDivElement>(null);

  const togglePlay = () => setIsPlaying(!isPlaying);
  const toggleMute = () => setIsMuted(!isMuted);

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setVolume(parseFloat(e.target.value));
  };

  const handleSeekChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setPlayed(parseFloat(e.target.value));
  };

  const handleSeekMouseDown = () => {
    setSeeking(true);
  };

  const handleSeekMouseUp = (e: React.MouseEvent<HTMLInputElement>) => {
    setSeeking(false);
    playerRef.current?.seekTo(parseFloat((e.target as HTMLInputElement).value));
  };

  const handleProgress = (state: { played: number }) => {
    if (!seeking) {
      setPlayed(state.played);
    }
  };

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      playerContainerRef.current?.requestFullscreen();
      setIsFullscreen(true);
    } else {
      document.exitFullscreen();
      setIsFullscreen(false);
    }
  };

  const videoUrl = `/api/v1/streaming?filePath=${encodeURIComponent(search.filePath)}&directPlay=${search.directPlay}&videoCodec=${search.videoCodec}&videoBitRate=${search.videoBitRate}&videoHeight=${search.videoHeight}&audioCodec=${search.audioCodec}&audioBitRate=${search.audioBitRate}&audioChannels=${search.audioChannels}&container=${search.container}`;

  return (
    <div className='container mx-auto px-4 py-8'>
      <div ref={playerContainerRef} className='relative'>
        <ReactPlayer
          ref={playerRef}
          url={videoUrl}
          width='100%'
          height='auto'
          playing={isPlaying}
          volume={volume}
          muted={isMuted}
          onProgress={handleProgress}
          className='react-player'
        />
        <div className='absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black to-transparent p-4'>
          <div className='flex items-center justify-between mb-2'>
            <button
              onClick={togglePlay}
              className='text-white hover:text-secondary'
            >
              {isPlaying ? <FaPause /> : <FaPlay />}
            </button>
            <input
              type='range'
              min={0}
              max={1}
              step='any'
              value={played}
              onMouseDown={handleSeekMouseDown}
              onChange={handleSeekChange}
              onMouseUp={handleSeekMouseUp}
              className='w-full mx-4 accent-secondary'
            />
            <div className='flex items-center'>
              <button
                onClick={toggleMute}
                className='text-white hover:text-secondary mr-2'
              >
                {isMuted ? <FaVolumeMute /> : <FaVolumeUp />}
              </button>
              <input
                type='range'
                min={0}
                max={1}
                step='any'
                value={volume}
                onChange={handleVolumeChange}
                className='w-20 accent-secondary'
              />
              <button
                onClick={toggleFullscreen}
                className='text-white hover:text-secondary ml-2'
              >
                {isFullscreen ? <FaCompress /> : <FaExpand />}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
