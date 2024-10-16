import { useState, useRef } from "react";
import { createFileRoute, useSearch } from "@tanstack/react-router";
import z from "zod";
import { v4 } from "uuid";
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
  const [volume, setVolume] = useState(0.5);
  const [isMuted, setIsMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);

  const videoRef = useRef<HTMLVideoElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVolume = parseFloat(e.target.value);
    if (videoRef.current) {
      videoRef.current.volume = newVolume;
    }
    setVolume(newVolume);
    setIsMuted(newVolume === 0);
  };

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      containerRef.current?.requestFullscreen();
      setIsFullscreen(true);
    } else {
      document.exitFullscreen();
      setIsFullscreen(false);
    }
  };

  const togglePlay = () => {
    if (videoRef.current) {
      if (videoRef.current.paused) {
        videoRef.current.play();
      } else {
        videoRef.current.pause();
      }
      setIsPlaying(!isPlaying);
    }
  };

  const toggleMute = () => {
    if (videoRef.current) {
      videoRef.current.muted = !videoRef.current.muted;
      setIsMuted(!videoRef.current.muted);
    }
  };

  const handleTimeUpdate = () => {
    if (videoRef.current) {
      setCurrentTime(videoRef.current.currentTime);
    }
  };

  const handleDurationChange = () => {
    if (videoRef.current) {
      setDuration(videoRef.current.duration);
    }
  };

  const formatTime = (time: number) => {
    const minutes = Math.floor(time / 60);
    const seconds = Math.floor(time % 60);
    return `${minutes}:${seconds < 10 ? "0" : ""}${seconds}`;
  };

  const url = search.directPlay ? `/api/v1/streaming/video?contentType=${encodeURIComponent("video/mp4")}&filePath=${encodeURIComponent(search.filePath)}`
  : `/api/v1/streaming/video/transcode?`

  return (
    <div className='container' ref={containerRef}>
      <div
        className='relative'
        onMouseEnter={() => setShowControls(true)}
        onMouseLeave={() => isPlaying && setShowControls(false)}
      >
        <video
          ref={videoRef}
          className='w-full h-auto'
          onPlay={() => setIsPlaying(true)}
          onPause={() => setIsPlaying(false)}
          onTimeUpdate={handleTimeUpdate}
          onDurationChange={handleDurationChange}
        >
          <source src={url} type='video/mp4' />
        </video>
        <div
          className={`absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black to-transparent p-4 transition-opacity duration-300 ${
            showControls ? "opacity-100" : "opacity-0"
          }`}
        >
          <div className='flex items-center justify-between'>
            <button
              onClick={togglePlay}
              className='text-white text-2xl focus:outline-none'
              aria-label={isPlaying ? "Pause" : "Play"}
            >
              {isPlaying ? <FaPause /> : <FaPlay />}
            </button>
            <div className='flex items-center'>
              <button
                onClick={toggleMute}
                className='text-white text-xl mr-2 focus:outline-none'
                aria-label={isMuted ? "Unmute" : "Mute"}
              >
                {isMuted ? <FaVolumeMute /> : <FaVolumeUp />}
              </button>
              <input
                type='range'
                min={0}
                max={1}
                step={0.1}
                value={volume}
                onChange={handleVolumeChange}
                className='w-20 mr-4 accent-white'
                aria-label='Volume'
                aria-valuemin={0} // Change to a number
                aria-valuemax={1} // Change to a number
                aria-valuenow={volume * 100} // Change to a number
              />
              <button
                onClick={toggleFullscreen}
                className='text-white text-xl focus:outline-none'
                aria-label={
                  isFullscreen ? "Exit fullscreen" : "Enter fullscreen"
                }
              >
                {isFullscreen ? <FaCompress /> : <FaExpand />}
              </button>
            </div>
          </div>
          <div className='w-full bg-gray-200 rounded-full h-1.5 mt-2'>
            <div
              className='bg-white h-1.5 rounded-full'
              style={{
                width: `${(currentTime / duration) * 100 || 0}%`,
              }}
              role='progressbar'
              aria-label='Video progress'
              aria-valuemin={0}
              aria-valuemax={duration}
              aria-valuenow={currentTime}
            ></div>
          </div>
          <div className='text-white text-sm mt-1'>
            <span aria-live='polite'>
              {formatTime(currentTime)} / {formatTime(duration)}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
