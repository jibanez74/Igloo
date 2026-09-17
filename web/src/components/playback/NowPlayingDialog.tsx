import { ChevronDown, Disc3, X } from "lucide-react";
import {
  Dialog,
  DialogDescription,
  DialogFullscreenContent,
  DialogTitle,
} from "@/components/ui/dialog";
import PlayerLikeButton from "@/components/playback/PlayerLikeButton";
import PlayerTransportControls from "@/components/playback/PlayerTransportControls";
import type { PlayerTransportProps } from "@/components/playback/PlayerTransportControls";
import ProgressBar from "@/components/playback/ProgressBar";
import VolumeControl from "@/components/playback/VolumeControl";
import {
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  PLAYER_ICON_BUTTON_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { TrackType } from "@/types";

type NowPlayingDialogProps = {
  track: TrackType;
  artist: string;
  albumTitle: string;
  albumCover: string | null;
  isExpanded: boolean;
  onMinimize: () => void;
  onClose?: () => void;

  currentTime: number;
  duration: number;
  onSeek: (newTime: number) => void;
  audioRef: React.RefObject<HTMLAudioElement | null>;

  // "Track N of M", already adjusted for tracks trimmed off an endless queue.
  trackPosition: number;
  trackTotal: number;

  transport: Omit<PlayerTransportProps, "variant">;
};

// The fullscreen "Now Playing" view. Carries data-audio-player so the global
// shortcuts treat it as the player's own chrome rather than a foreign dialog.
export default function NowPlayingDialog({
  track,
  artist,
  albumTitle,
  albumCover,
  isExpanded,
  onMinimize,
  onClose,
  currentTime,
  duration,
  onSeek,
  audioRef,
  trackPosition,
  trackTotal,
  transport,
}: NowPlayingDialogProps) {
  return (
    <Dialog
      open={isExpanded}
      onOpenChange={open => {
        if (!open) {
          onMinimize();
        }
      }}
    >
      {isExpanded && (
        <DialogFullscreenContent
          data-audio-player=""
          className={cn(
            MOTION_MEDIA_OVERLAY_ENTER_CLASS,
            "flex flex-col bg-linear-to-b from-background via-muted to-background",
          )}
          onOpenAutoFocus={event => {
            event.preventDefault();
            transport.playPauseButtonRef?.current?.focus({
              preventScroll: true,
            });
          }}
          onCloseAutoFocus={event => {
            event.preventDefault();
          }}
        >
          <DialogTitle className="sr-only">
            Now playing: {track.title} by {artist}
          </DialogTitle>
          <DialogDescription className="sr-only">
            Press Escape to minimize.
          </DialogDescription>

          <header className="flex items-center justify-between px-6 py-4">
            <button
              type="button"
              onClick={onMinimize}
              className={cn(
                PLAYER_ICON_BUTTON_CLASS,
                "size-10 hover:bg-accent/50",
              )}
              aria-label="Minimize player (Escape)"
            >
              <ChevronDown className="size-5" aria-hidden="true" />
            </button>
            <div className="text-center" id="player-header">
              <p className="text-xs tracking-widest text-muted-foreground uppercase">
                Now Playing
              </p>
              <p className="mt-0.5 text-sm text-muted-foreground">{albumTitle}</p>
            </div>
            {onClose ? (
              <button
                type="button"
                onClick={onClose}
                className={cn(
                  PLAYER_ICON_BUTTON_CLASS,
                  "size-10 hover:bg-accent/50",
                )}
                aria-label="Stop playback and close player"
              >
                <X className="size-5" aria-hidden="true" />
              </button>
            ) : (
              <div className="size-10" aria-hidden="true" />
            )}
          </header>

          <main className="flex flex-1 flex-col items-center justify-center px-8 pb-8">
            <div className="mb-8 size-72 overflow-hidden rounded-2xl shadow-2xl shadow-black/50 sm:size-80 md:size-96">
              {albumCover ? (
                <img
                  src={albumCover}
                  alt={`Album cover for ${albumTitle}`}
                  className="size-full object-cover"
                />
              ) : (
                <div
                  className="flex size-full items-center justify-center bg-muted"
                  role="img"
                  aria-label="No album cover available"
                >
                  <Disc3 className="size-24 text-muted-foreground" aria-hidden="true" />
                </div>
              )}
            </div>

            <div className="mb-8 flex max-w-md items-center gap-3">
              <div className="size-10 shrink-0" aria-hidden="true" />
              <div className="min-w-0 text-center">
                <h1
                  id="track-title"
                  className="truncate text-2xl font-bold text-foreground sm:text-3xl"
                >
                  {track.title}
                </h1>
                <p className="mt-1 truncate text-lg text-primary">{artist}</p>
              </div>
              <PlayerLikeButton
                trackId={track.id}
                trackTitle={track.title}
                variant="expanded"
              />
            </div>

            <ProgressBar
              currentTime={currentTime}
              duration={duration}
              onSeek={onSeek}
              variant="expanded"
              resetKey={track.id}
            />

            <PlayerTransportControls variant="expanded" {...transport} />

            <div className="mt-6">
              <VolumeControl mediaRef={audioRef} variant="expanded" />
            </div>

            <p className="mt-4 text-sm text-muted-foreground">
              Track {trackPosition} of {trackTotal}
            </p>
          </main>
        </DialogFullscreenContent>
      )}
    </Dialog>
  );
}
