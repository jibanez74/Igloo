import PropTypes from "prop-types";
import { useEffect, useRef, useState } from "react";
import shaka from "shaka-player";

export default function VideoPlayer({ src, onError }) {
  const videoRef = useRef(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const video = videoRef.current;
    const player = new shaka.Player(video);

    const onErrorEvent = event => {
      onError(event.detail);
    };

    player.addEventListener("error", onErrorEvent);

    setIsLoading(true);
    player
      .load(src)
      .then(() => {
        console.log("The video has now been loaded!");
        setIsLoading(false);
      })
      .catch(onError);

    return () => {
      player.removeEventListener("error", onErrorEvent);
      player.destroy();
    };
  }, [src, onError]);

  return (
    <div style={{ position: "relative", width: "100%", height: "100%" }}>
      {isLoading && (
        <div
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          Loading...
        </div>
      )}
      <video
        ref={videoRef}
        style={{ width: "100%", height: "100%", objectFit: "contain" }}
        controls
        aria-label='Video player'
      />
    </div>
  );
}

VideoPlayer.propTypes = {
  src: PropTypes.string.isRequired,
  onError: PropTypes.func,
};
