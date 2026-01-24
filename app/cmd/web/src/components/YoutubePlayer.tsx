import { useEffect } from "react";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { useYouTubePlayer } from "@/hooks/useYouTubePlayer";

type YoutubePlayerProps = {
  videoKey: string;
  title?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export default function YoutubePlayer({
  videoKey,
  title = "Video",
  open,
  onOpenChange,
}: YoutubePlayerProps) {
  const { containerRef, isReady, error } = useYouTubePlayer({
    videoId: open ? videoKey : null,
    autoplay: true,
    controls: true,
    onEnd: () => onOpenChange(false),
  });

  useEffect(() => {
    if (open && isReady && containerRef.current) {
      const timer = setTimeout(() => {
        const iframe = containerRef.current?.querySelector("iframe");

        if (iframe) {
          iframe.focus();
        }
      }, 300);

      return () => clearTimeout(timer);
    }
  }, [open, isReady, containerRef]);

  // Prevent Space and Enter from bubbling up and closing the dialog
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key !== "Escape") {
      e.stopPropagation();
    }
  };

  // Prevent dialog from closing when clicking inside the player
  const handleInteractOutside = (e: Event) => {
    e.preventDefault();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className='w-full max-w-4xl overflow-hidden border-cyan-500/30 bg-slate-950 p-0 shadow-2xl shadow-cyan-500/10'
        showCloseButton={true}
        onInteractOutside={handleInteractOutside}
      >
        <DialogTitle className='sr-only'>{title}</DialogTitle>
        <DialogDescription className='sr-only'>
          Playing {title}. Click on the video to give it focus, then use YouTube
          keyboard shortcuts: Space or K to play/pause, J to rewind 10 seconds,
          L to forward 10 seconds, arrow keys to seek 5 seconds, M to mute, F
          for fullscreen. Tab to the Close button and press Escape to close this
          dialog.
        </DialogDescription>

        <div className='relative aspect-video w-full' onKeyDown={handleKeyDown}>
          {error ? (
            <div className='flex h-full w-full items-center justify-center bg-slate-900'>
              <p className='text-slate-400'>{error}</p>
            </div>
          ) : (
            <>
              <div ref={containerRef} className='h-full w-full' />
              {/* Loading overlay while player initializes */}
              {!isReady && (
                <div className='absolute inset-0 flex items-center justify-center bg-slate-900'>
                  <Spinner className="size-10 text-amber-400" />
                </div>
              )}
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
