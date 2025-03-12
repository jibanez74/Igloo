import { createEffect, onCleanup, Show } from "solid-js";
import YoutubePlayer from "./YoutubePlayer";

type YoutubeModalProps = {
  isOpen: boolean;
  onClose: () => void;
  videoUrl: string | null;
};

export default function YoutubeModal(props: YoutubeModalProps) {
  let dialogRef: HTMLDialogElement | undefined;

  createEffect(() => {
    if (!dialogRef) return;

    if (props.isOpen && props.videoUrl) {
      dialogRef.showModal();
      document.body.style.overflow = "hidden";
    } else {
      dialogRef.close();
      document.body.style.overflow = "unset";
    }
  });

  createEffect(() => {
    if (!props.isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        props.onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    onCleanup(() => document.removeEventListener("keydown", handleKeyDown));
  });

  onCleanup(() => {
    document.body.style.overflow = "unset";
  });

  const handleBackdropClick = (e: MouseEvent) => {
    if (dialogRef && e.target === dialogRef) {
      props.onClose();
    }
  };

  return (
    <dialog
      ref={dialogRef}
      class="fixed m-auto inset-0 p-4 max-h-[90vh] max-w-[90vw] w-full md:w-[800px] bg-transparent 
             backdrop:bg-slate-900/90 backdrop:backdrop-blur-sm"
      onClick={handleBackdropClick}
      onClose={props.onClose}
    >
      <div class="bg-slate-900 rounded-lg shadow-xl shadow-black/20 overflow-hidden">
        <Show when={props.videoUrl}>
          {(url) => (
            <YoutubePlayer 
              url={url()} 
              autoplay={true}
              class="w-full"
              onEnded={props.onClose}
            />
          )}
        </Show>
      </div>
    </dialog>
  );
}
