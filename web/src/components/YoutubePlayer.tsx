import { useRef, useEffect } from "react";
import ReactPlayer from "react-player";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";

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
  const videoUrl = `https://www.youtube.com/watch?v=${videoKey}`;
  const containerRef = useRef<HTMLDivElement>(null);

  // Focus the YouTube iframe when dialog opens so native keyboard controls work
  useEffect(() => {
    if (open && containerRef.current) {
      // Small delay to ensure the iframe is rendered
      const timer = setTimeout(() => {
        const iframe = containerRef.current?.querySelector("iframe");
        if (iframe) {
          iframe.focus();
        }
      }, 300);
      return () => clearTimeout(timer);
    }
  }, [open]);

  // Prevent Space and Enter from bubbling up and closing the dialog
  // when focus is on the container (not the iframe)
  const handleKeyDown = (e: React.KeyboardEvent) => {
    // Only allow Escape to bubble up for closing the dialog
    if (e.key !== "Escape") {
      e.stopPropagation();
    }
  };

  // Prevent dialog from closing when clicking inside the player
  const handleInteractOutside = (e: Event) => {
    e.preventDefault();
  };

  // Close the dialog when the video ends
  const handleEnded = () => {
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className='max-w-4xl w-full bg-slate-950 border-cyan-500/30 p-0 overflow-hidden shadow-2xl shadow-cyan-500/10'
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

        <div
          ref={containerRef}
          className='aspect-video w-full'
          onKeyDown={handleKeyDown}
        >
          <ReactPlayer
            src={videoUrl}
            playing={open}
            controls
            width='100%'
            height='100%'
            onEnded={handleEnded}
          />
        </div>
      </DialogContent>
    </Dialog>
  );
}
