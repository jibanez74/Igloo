import { useState } from "react";
import Modal from "react-bootstrap/Modal";
import YoutubePlayer from "./YoutubePlayer";

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
    <Modal
      show={show}
      onHide={handleClose}
      size='xl'
      centered
      animation={true}
      dialogClassName='modal-90w'
      contentClassName='bg-transparent border-0'
    >
      <Modal.Header
        closeButton
        className='border-0 bg-dark'
        closeVariant='white'
      />

      <Modal.Body className='p-0 bg-dark'>
        <YoutubePlayer
          url={videoUrl}
          playing={isPlaying}
          onPlay={() => setIsPlaying(true)}
          onPause={() => setIsPlaying(false)}
          onEnded={() => setIsPlaying(false)}
          onError={e => console.error("Player Error:", e)}
        />
      </Modal.Body>
    </Modal>
  );
}
