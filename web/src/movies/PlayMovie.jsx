import { Suspense, useState, useRef } from "react";
import { Await, defer, useLoaderData } from "react-router-dom";
import queryClient from "../utils/queryClient";
import { getMovieByID } from "./httpMovie";
import Spinner from "../shared/Spinner";
import {
  FaPlay,
  FaPause,
  FaVolumeUp,
  FaVolumeMute,
  FaExpand,
  FaCompress,
} from "react-icons/fa";

export default function PlayMovie() {
  const { data } = useLoaderData();
  const [isPlaying, setIsPlaying] = useState(false);
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const videoRef = useRef(null);
  const containerRef = useRef(null);

  const togglePlay = () => {
    if (videoRef.current.paused) {
      videoRef.current.play();
    } else {
      videoRef.current.pause();
    }
    setIsPlaying(!isPlaying);
  };

  const toggleMute = () => {
    videoRef.current.muted = !videoRef.current.muted;
    setIsMuted(!isMuted);
  };

  const handleVolumeChange = e => {
    const newVolume = parseFloat(e.target.value);
    videoRef.current.volume = newVolume;
    setVolume(newVolume);
    setIsMuted(newVolume === 0);
  };

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen();
      setIsFullscreen(true);
    } else {
      document.exitFullscreen();
      setIsFullscreen(false);
    }
  };

  const handleTimeUpdate = () => {
    setCurrentTime(videoRef.current.currentTime);
  };

  const handleDurationChange = () => {
    setDuration(videoRef.current.duration);
  };

  const formatTime = time => {
    const minutes = Math.floor(time / 60);
    const seconds = Math.floor(time % 60);
    return `${minutes}:${seconds < 10 ? "0" : ""}${seconds}`;
  };

  return (
    <div className='container mx-auto' ref={containerRef}>
      <Suspense fallback={<Spinner />}>
        <Await resolve={data}>
          {data => {
            let url = `/api/v1/movie/stream/${data._id}?transcode=yes&vcodec=h264_nvenc&channels=2&acodec=aac&abitrate=320k`;

            if (data.contentType === "video/mp4") {
              url = `/api/v1/movie/stream/${data._id}?transcode=no`;
            }

            alert(url);

            return (
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
                  Your browser does not support the video tag.
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
                        min='0'
                        max='1'
                        step='0.1'
                        value={volume}
                        onChange={handleVolumeChange}
                        className='w-20 mr-4 accent-white'
                        aria-label='Volume'
                        aria-valuemin='0'
                        aria-valuemax='100'
                        aria-valuenow={volume * 100}
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
                      aria-valuemin='0'
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
            );
          }}
        </Await>
      </Suspense>
    </div>
  );
}

async function getMovie(id) {
  return queryClient.fetchQuery({
    queryKey: ["movie", id],
    queryFn: () => getMovieByID(id),
  });
}

export async function loader({ params }) {
  return defer({
    data: await getMovie(params.id),
  });
}
