import { createEffect, For } from "solid-js";
import { FiX } from "solid-icons/fi";

type Option = {
  value: number;
  label: string;
};

type PlaybackSettingsModalProps = {
  isOpen: boolean;
  onClose: () => void;
  audioOpts: Option[];
  subtitleOpts: Option[];
};

export default function PlaybackSettingsModal(props: PlaybackSettingsModalProps) {
  let dialogRef: HTMLDialogElement | undefined;
  let formRef: HTMLFormElement | undefined;

  createEffect(() => {
    if (!dialogRef) return;

    if (props.isOpen) {
      dialogRef.showModal();
    } else {
      dialogRef.close();
    }
  });

  const handleCancel = () => {
    formRef?.reset();
    props.onClose();
  };

  return (
    <dialog
      ref={dialogRef}
      class="fixed inset-0 bg-transparent p-0 m-0 h-full w-full outline-none"
      onClick={(e) => {
        if (e.target === dialogRef) props.onClose();
      }}
    >
      <div
        class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 
                    w-[600px] max-w-[90vw] bg-slate-900 rounded-xl shadow-2xl"
      >
        {/* Header */}
        <div class="flex items-center justify-between p-6 border-b border-sky-200/10">
          <h2 class="text-xl font-medium text-white">Playback Settings</h2>
          <button
            type="button"
            onClick={handleCancel}
            class="text-sky-200 hover:text-sky-100 transition-colors"
          >
            <FiX class="w-6 h-6" aria-hidden="true" />
          </button>
        </div>

        {/* Content */}
        <div class="p-6">
          <form ref={formRef} class="space-y-6">
            {/* Audio Track */}
            <div>
              <label
                for="audioTrack"
                class="block text-base font-medium text-sky-200 mb-2"
              >
                Audio Track
              </label>
              <select
                id="audioTrack"
                name="audioTrack"
                class="w-full bg-slate-800 border border-sky-200/10 rounded-lg px-4 py-3 text-base text-white 
                         focus:outline-none focus:ring-2 focus:ring-sky-500/40"
                value="english"
              >
                <For each={props.audioOpts}>
                  {(opt) => (
                    <option value={opt.value}>{opt.label}</option>
                  )}
                </For>
              </select>
            </div>

            {/* Subtitles */}
            <div>
              <label
                for="subtitles"
                class="block text-base font-medium text-sky-200 mb-2"
              >
                Subtitles
              </label>
              <select
                id="subtitles"
                name="subtitles"
                class="w-full bg-slate-800 border border-sky-200/10 rounded-lg px-4 py-3 text-base text-white 
                         focus:outline-none focus:ring-2 focus:ring-sky-500/40"
                value="none"
              >
                <option value="none">None</option>
                <For each={props.subtitleOpts}>
                  {(opt) => (
                    <option value={opt.value}>{opt.label}</option>
                  )}
                </For>
              </select>
            </div>

            {/* Resolution */}
            <div>
              <label
                for="resolution"
                class="block text-base font-medium text-sky-200 mb-2"
              >
                Resolution
              </label>
              <select
                id="resolution"
                name="resolution"
                class="w-full bg-slate-800 border border-sky-200/10 rounded-lg px-4 py-3 text-base text-white 
                         focus:outline-none focus:ring-2 focus:ring-sky-500/40"
                value="1080p"
              >
                <option value="2160p">4K (2160p)</option>
                <option value="1080p">Full HD (1080p)</option>
                <option value="720p">HD (720p)</option>
                <option value="480p">SD (480p)</option>
              </select>
            </div>

            {/* Buttons */}
            <div class="pt-4 flex gap-4">
              <button
                type="submit"
                class="flex-1 px-6 py-3 bg-sky-500 hover:bg-sky-400 text-white text-base font-medium 
                         rounded-lg transition-colors"
              >
                Apply Settings
              </button>
              <button
                type="button"
                onClick={handleCancel}
                class="flex-1 px-6 py-3 bg-slate-800 hover:bg-slate-700 text-sky-200 hover:text-sky-100 
                         text-base font-medium rounded-lg transition-colors"
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
