import { useRef, useEffect } from "react";
import YoutubePlayer from "./YoutubePlayer";

type YoutubeModalProps = {
  isOpen: boolean;
  onClose: () => void;
  videoUrl: string | null;
};

export default function YoutubeModal({
  isOpen,
  onClose,
  videoUrl,
}: YoutubeModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    if (isOpen) {
      dialog.showModal();
      // Prevent body scroll when modal is open
      document.body.style.overflow = "hidden";
    } else {
      dialog.close();
      // Restore body scroll when modal is closed
      document.body.style.overflow = "unset";
    }

    return () => {
      document.body.style.overflow = "unset";
    };
  }, [isOpen]);

  const handleBackdropClick = (e: React.MouseEvent<HTMLDialogElement>) => {
    const dialog = dialogRef.current;
    if (dialog && e.target === dialog) {
      onClose();
    }
  };

  return (
    <dialog
      ref={dialogRef}
      className='fixed inset-0 p-0 m-0 max-w-4xl w-[92%] h-fit bg-transparent backdrop:bg-slate-900/90 backdrop:backdrop-blur-sm'
      onClick={handleBackdropClick}
      onClose={onClose}
    >
      <div className='bg-slate-900 rounded-lg shadow-xl shadow-black/20'>
        <YoutubePlayer url={videoUrl} playing={isOpen} onEnded={onClose} />
      </div>
    </dialog>
  );
}
