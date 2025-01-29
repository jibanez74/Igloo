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
      className='fixed m-auto inset-0 p-4 max-h-[90vh] max-w-[90vw] w-full md:w-[800px] bg-transparent 
                 backdrop:bg-slate-900/90 backdrop:backdrop-blur-sm'
      onClick={handleBackdropClick}
      onClose={onClose}
    >
      <div className='bg-slate-900 rounded-lg shadow-xl shadow-black/20 overflow-hidden'>
        <YoutubePlayer url={videoUrl} playing={isOpen} onEnded={onClose} />
      </div>
    </dialog>
  );
}
