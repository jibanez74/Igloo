import PropTypes from "prop-types";
import { useEffect, useRef } from "react";
import videojs from "video.js";
import "video.js/dist/video-js.css";

export default function VideoPlayer({ options }) {
  const videoRef = useRef(null);
  const playerRef = useRef(null);

  useEffect(() => {
    if (videoRef.current && !playerRef.current) {
      const player = videojs(videoRef.current, options, () => {
        console.log("Player is ready");
      });
      playerRef.current = player;
    }

    return () => {
      if (playerRef.current) {
        playerRef.current.dispose();
        playerRef.current = null;
      }
    };
  }, [options]);

  return (
    <div>
      <div data-vjs-player>
        <video ref={videoRef} className='video-js vjs-big-play-centered' />
      </div>
    </div>
  );
}
const defaultOptions = {
  controls: true,
  autoplay: true,
};

export default function VideoPlayer({ options }) {
  const videoRef = useRef(null);
  const playerRef = useRef(null);

  const mergedOptions = { ...defaultOptions, ...options };

  useEffect(() => {
    if (videoRef.current && !playerRef.current) {
      const player = videojs(videoRef.current, mergedOptions, () => {
        console.log("Player is ready");
      });
      playerRef.current = player;
    }

    return () => {
      if (playerRef.current) {
        playerRef.current.dispose();
        playerRef.current = null;
      }
    };
  }, [mergedOptions]);

  return (
    <div>
      <div data-vjs-player>
        <video ref={videoRef} className='video-js vjs-big-play-centered' />
      </div>
    </div>
  );
}

VideoPlayer.propTypes = {
  options: PropTypes.shape({
    autoplay: PropTypes.bool,
    controls: PropTypes.bool,
    responsive: PropTypes.bool,
    fluid: PropTypes.bool,
    sources: PropTypes.arrayOf(
      PropTypes.shape({
        src: PropTypes.string.isRequired, // Source URL is required
        type: PropTypes.string.isRequired, // Type (e.g., 'application/x-mpegURL', 'video/mp4') is required
      })
    ).isRequired, // Sources is an array of objects, and it is required
  }).isRequired, // Options itself is required
};
