  import { useEffect, useRef } from "react";
import { FiX } from "react-icons/fi";

type opts = {
  value: number;
  label: string;
};

type PlaybackSettingsModalProps = {
  isOpen: boolean;
  onClose: () => void;
  audioOpts: opts[];
  subtitleOpts: opts[];
};

export default function PlaybackSettingsModal({
  isOpen,
  onClose,
  audioOpts,
  subtitleOpts,
}: PlaybackSettingsModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const formRef = useRef<HTMLFormElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    if (isOpen) {
      dialog.showModal();
    } else {
      dialog.close();
    }
  }, [isOpen]);

  const handleCancel = () => {
    formRef.current?.reset();
    onClose();
  };

  return (
    <dialog
      ref={dialogRef}
      className='fixed inset-0 bg-transparent p-0 m-0 h-full w-full outline-none'
      onClick={e => {
        if (e.target === dialogRef.current) onClose();
      }}
    >
      <div
        className='absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 
                    w-[600px] max-w-[90vw] bg-slate-900 rounded-xl shadow-2xl'
      >
        {/* Header */}
        <div className='flex items-center justify-between p-6 border-b border-sky-200/10'>
          <h2 className='text-xl font-medium text-white'>Playback Settings</h2>
          <button
            type='button'
            onClick={handleCancel}
            className='text-sky-200 hover:text-sky-100 transition-colors'
          >
            <FiX className='w-6 h-6' aria-hidden='true' />
          </button>
        </div>

        {/* Content */}
        <div className='p-6'>
          <form ref={formRef} className='space-y-6'>
            {/* Audio Track */}
            <div>
              <label
                htmlFor='audioTrack'
                className='block text-base font-medium text-sky-200 mb-2'
              >
                Audio Track
              </label>
              <select
                id='audioTrack'
                name='audioTrack'
                className='w-full bg-slate-800 border border-sky-200/10 rounded-lg px-4 py-3 text-base text-white 
                         focus:outline-none focus:ring-2 focus:ring-sky-500/40'
                defaultValue='english'
              >
                {audioOpts.map(o => (
                  <option value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>

            {/* Subtitles */}
            <div>
              <label
                htmlFor='subtitles'
                className='block text-base font-medium text-sky-200 mb-2'
              >
                Subtitles
              </label>
              <select
                id='subtitles'
                name='subtitles'
                className='w-full bg-slate-800 border border-sky-200/10 rounded-lg px-4 py-3 text-base text-white 
                         focus:outline-none focus:ring-2 focus:ring-sky-500/40'
                defaultValue='none'
              >
                <option value='none'>None</option>

                {subtitleOpts.map(o => (
                  <option value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>

            {/* Resolution */}
            <div>
              <label
                htmlFor='resolution'
                className='block text-base font-medium text-sky-200 mb-2'
              >
                Resolution
              </label>
              <select
                id='resolution'
                name='resolution'
                className='w-full bg-slate-800 border border-sky-200/10 rounded-lg px-4 py-3 text-base text-white 
                         focus:outline-none focus:ring-2 focus:ring-sky-500/40'
                defaultValue='1080p'
              >
                <option value='2160p'>4K (2160p)</option>
                <option value='1080p'>Full HD (1080p)</option>
                <option value='720p'>HD (720p)</option>
                <option value='480p'>SD (480p)</option>
              </select>
            </div>

            {/* Buttons */}
            <div className='pt-4 flex gap-4'>
              <button
                type='submit'
                className='flex-1 px-6 py-3 bg-sky-500 hover:bg-sky-400 text-white text-base font-medium 
                         rounded-lg transition-colors'
              >
                Apply Settings
              </button>
              <button
                type='button'
                onClick={handleCancel}
                className='flex-1 px-6 py-3 bg-slate-800 hover:bg-slate-700 text-sky-200 hover:text-sky-100 
                         text-base font-medium rounded-lg transition-colors'
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </dialog>
  );
}
