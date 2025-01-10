import { useState } from "react";
import { useNavigate } from "react-router-dom";
import HlsPlayer from "../shared/HlsPlayer";

interface PlayMovieProps {
  movieId: number;
  onClose?: () => void;
}

export default function PlayMovie({ movieId, onClose }: PlayMovieProps) {
  const [error, setError] = useState<string>("");
  const navigate = useNavigate();

  const handleError = (error: Error) => {
    console.error("Playback error:", error);
    setError("Failed to play video. Please try again.");
  };

  const handleClose = () => {
    onClose?.();
    navigate(`/movie/${movieId}`);
  };

  return (
    <div className='video-player-container'>
      <HlsPlayer movieId={movieId} onError={handleError} />
      {error && (
        <div className='error-message'>
          {error}
          <button onClick={handleClose}>Close</button>
        </div>
      )}
      <style jsx>{`
        .video-player-container {
          position: fixed;
          top: 0;
          left: 0;
          width: 100%;
          height: 100%;
          background: black;
          z-index: 1000;
        }
        .error-message {
          position: absolute;
          top: 50%;
          left: 50%;
          transform: translate(-50%, -50%);
          background: rgba(0, 0, 0, 0.8);
          color: white;
          padding: 1rem;
          border-radius: 4px;
          text-align: center;
        }
        button {
          margin-top: 1rem;
          padding: 0.5rem 1rem;
          background: #007bff;
          border: none;
          border-radius: 4px;
          color: white;
          cursor: pointer;
        }
        button:hover {
          background: #0056b3;
        }
      `}</style>
    </div>
  );
}
