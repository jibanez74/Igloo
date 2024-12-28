import { useState } from "react";
import Modal from "react-bootstrap/Modal";
import HlsPlayer from "./HlsPlayer";
import Message from "./Message";

type VideoPlayerProps = {
  show: boolean;
  onHide: () => void;
  url: string;
  title?: string;
  poster?: string;
  useHlsJs: boolean;
};

export default function VideoPlayer({
  show,
  onHide,
  url,
  title,
  poster,
  useHlsJs,
}: VideoPlayerProps) {
  const [error, setError] = useState<string | null>(null);

  const handleError = (error: Error) => {
    console.error("Video Player Error:", error);
    setError(error.message);
  };

  return (
    <Modal
      show={show}
      onHide={onHide}
      size='xl'
      centered
      fullscreen='lg-down'
      contentClassName='bg-dark'
    >
      <Modal.Header closeButton closeVariant='white' className='border-0' />

      <Modal.Body className='p-0'>
        {error ? (
          <div className='p-3'>
            <Message
              title='Video Playback Error'
              msg={error}
              variant='danger'
              duration={5000}
            />
          </div>
        ) : (
          <HlsPlayer
            url={url}
            title={title}
            poster={poster}
            useHlsJs={useHlsJs}
            onError={handleError}
            onEnded={onHide}
            onProgress={state => {
              // You can add progress tracking here if needed
              console.log("Progress:", state.played);
            }}
          />
        )}
      </Modal.Body>
    </Modal>
  );
}
