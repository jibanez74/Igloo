import { useState } from "react";
import Modal from "react-bootstrap/Modal";
import ReactPlayer from "react-player";

type VideoExtrasModalProps = {
  show: boolean;
  onHide: () => void;
  videoUrl: string | null;
  title: string;
};

export default function VideoExtrasModal({
  show,
  onHide,
  videoUrl,
  title,
}: VideoExtrasModalProps) {
  const [isPlaying, setIsPlaying] = useState(false);

  const handleClose = () => {
    setIsPlaying(false);
    onHide();
  };

  return (
    <>
      <Modal
        show={show}
        onHide={handleClose}
        size='xl'
        centered
        animation={true}
        dialogClassName='modal-90w'
      >
        <Modal.Header closeButton className='border-0 bg-dark'>
          <Modal.Title className='text-light'>{title}</Modal.Title>
        </Modal.Header>
        <Modal.Body className='p-0 bg-dark'>
          <div className='player-wrapper'>
            <ReactPlayer
              url={videoUrl || ""}
              playing={isPlaying}
              controls={true}
              width='100%'
              height='100%'
              style={{ position: "absolute", top: 0, left: 0 }}
              config={{
                youtube: {
                  playerVars: {
                    autoplay: 1,
                    modestbranding: 1,
                    rel: 0,
                    origin: window.location.origin,
                    enablejsapi: 1,
                  },
                },
              }}
              onEnded={() => setIsPlaying(false)}
              onError={e => console.error("Player Error:", e)}
            />
          </div>
        </Modal.Body>
      </Modal>

      <style>
        {`
          .modal-90w {
            width: 90%;
            max-width: 1200px;
          }
          .modal-content {
            background-color: transparent;
            border: none;
          }
          .player-wrapper {
            position: relative;
            padding-top: 56.25%; /* 16:9 Aspect Ratio */
          }
          .modal-header .btn-close {
            filter: invert(1);
          }
        `}
      </style>
    </>
  );
}
