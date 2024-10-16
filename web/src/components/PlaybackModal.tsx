import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import type { Audio } from "@/types/Audio";
import type { Subtitles } from "@/types/Subtitles";

type ModalProps = {
  isOpen: boolean;
  onClose: () => void;
  audioTracks: Audio[];
  subtitleTracks: Subtitles[];
};

export default function PlaybackSettingsModal({
  isOpen,
  onClose,
  audioTracks = [],
  subtitleTracks = [],
}: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;

    if (isOpen && dialog && !dialog.open) {
      dialog.showModal();
    } else if (!isOpen && dialog && dialog.open) {
      dialog.close();
    }
  }, [isOpen]);

  useEffect(() => {
    const dialog = dialogRef.current;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    dialog?.addEventListener("keydown", handleKeyDown);

    return () => {
      dialog?.removeEventListener("keydown", handleKeyDown);
    };
  }, [onClose]);

  if (!isOpen) return null;

  return createPortal(
    <div className='fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center'>
      <dialog
        ref={dialogRef}
        className='bg-white p-6 rounded-lg shadow-lg w-96 max-w-full z-50'
      >
        <form method='dialog' className='flex flex-col space-y-4'>
          {/* Resolution Dropdown */}
          <label className='text-lg font-semibold text-dark'>
            Resolution
            <select className='mt-2 p-2 border border-gray-300 rounded-lg w-full'>
              <option value='1080p'>1080p</option>
              <option value='720p'>720p</option>
              <option value='480p'>480p</option>
              <option value='360p'>360p</option>
            </select>
          </label>

          {/* Audio Track Dropdown */}
          <label className='text-lg font-semibold text-dark'>
            Audio Track
            <select
              disabled={audioTracks.length === 0}
              className='mt-2 p-2 border border-gray-300 rounded-lg w-full'
            >
              {audioTracks.length > 0 &&
                audioTracks.map(t => (
                  <option key={t.ID} value={t.index}>
                    {`${t.language} - ${t.codec} ${t.channelLayout}`}
                  </option>
                ))}
            </select>
          </label>

          {/* Subtitles Dropdown */}
          <label className='text-lg font-semibold text-dark'>
            Subtitles
            <select
              disabled={subtitleTracks.length === 0}
              className='mt-2 p-2 border border-gray-300 rounded-lg w-full'
            >
              {subtitleTracks.length > 0 &&
                subtitleTracks.map(t => (
                  <option key={t.ID} value={t.index}>
                    {t.language}
                  </option>
                ))}
            </select>
          </label>

          {/* Buttons */}
          <div className='flex justify-between space-x-4'>
            <button
              type='button'
              onClick={onClose}
              className='bg-danger text-white px-4 py-2 rounded-lg hover:bg-danger-dark'
            >
              Cancel
            </button>
            <button
              type='submit'
              className='bg-success text-white px-4 py-2 rounded-lg hover:bg-success-dark'
            >
              Confirm
            </button>
          </div>
        </form>
      </dialog>
    </div>,
    document.body
  );
}
