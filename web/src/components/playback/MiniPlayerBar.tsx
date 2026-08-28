import { ChevronUp, Disc3, X } from "lucide-react";
import PlayerLikeButton from "@/components/playback/PlayerLikeButton";
import PlayerTransportControls from "@/components/playback/PlayerTransportControls";
import type { PlayerTransportProps } from "@/components/playback/PlayerTransportControls";
import ProgressBar from "@/components/playback/ProgressBar";
import VolumeControl from "@/components/playback/VolumeControl";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_ENTER_CLASS,
  PLAYER_ICON_BUTTON_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { TrackType } from "@/types";

type MiniPlayerBarProps = {
  track: TrackType;
  artist: string;
  albumCover: string | null;
  onExpand: () => void;
  onClose?: () => void;
  expandButtonRef: React.RefObject<HTMLButtonElement | null>;

  currentTime: number;
  duration: number;
  onSeek: (newTime: number) => void;
  audioRef: React.RefObject<HTMLAudioElement | null>;

  transport: Omit<PlayerTransportProps, "variant" | "playPauseButtonRef">;
};

// The docked bar shown while the fullscreen view is minimized. Carries
// data-audio-player so the global shortcuts treat its buttons as player
// chrome (Space pauses, Enter still activates the focused control).
export default function MiniPlayerBar({
  track,
  artist,
  albumCover,
  onExpand,
  onClose,
  expandButtonRef,
  currentTime,
  duration,
  onSeek,
  audioRef,
  transport,
}: MiniPlayerBarProps) {
  return (
    <div
      role="region"
      aria-label="Audio player"
      data-audio-player=""
      className={cn(
        MOTION_PLAYER_CHROME_ENTER_CLASS,
        "fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background/95 shadow-2xl shadow-black/50 backdrop-blur-lg",
      )}
    >
      <div className="mx-auto max-w-7xl px-4 py-3">
        <div className="flex items-center gap-4">
          <button
            ref={expandButtonRef}
            type="button"
            onClick={onExpand}
            className={cn(
              MOTION_PLAYER_CHROME_BUTTON_CLASS,
              FOCUS_VISIBLE_RING_CLASS,
              "flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left hover:opacity-80",
            )}
            aria-label={`Expand player. Now playing: ${track.title} by ${artist}`}
          >
            <div
              className="size-12 shrink-0 overflow-hidden rounded-lg bg-muted shadow-lg"
              aria-hidden="true"
            >
              {albumCover ? (
                <img
                  src={albumCover}
                  alt=""
                  className="size-full object-cover"
                />
              ) : (
                <div className="flex size-full items-center justify-center">
                  <Disc3 className="size-5 text-muted-foreground" aria-hidden="true" />
                </div>
              )}
            </div>

            <div className="min-w-0 flex-1" aria-hidden="true">
              <p className="truncate text-sm font-medium text-foreground">
                {track.title}
              </p>
              <p className="truncate text-xs text-muted-foreground">{artist}</p>
            </div>
          </button>

          <PlayerLikeButton
            trackId={track.id}
            trackTitle={track.title}
            variant="minimized"
          />

          <PlayerTransportControls variant="minimized" {...transport} />

          <ProgressBar
            currentTime={currentTime}
            duration={duration}
            onSeek={onSeek}
            variant="minimized"
            resetKey={track.id}
          />

          <div className="hidden sm:block">
            <VolumeControl mediaRef={audioRef} variant="minimized" />
          </div>

          <button
            type="button"
            onClick={onExpand}
            className={cn(
              PLAYER_ICON_BUTTON_CLASS,
              "hidden size-8 hover:bg-accent sm:flex",
            )}
            aria-label="Expand to fullscreen player"
          >
            <ChevronUp className="size-4" aria-hidden="true" />
          </button>

          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className={cn(PLAYER_ICON_BUTTON_CLASS, "size-8 hover:bg-accent")}
              aria-label="Stop playback and close player"
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          )}
        </div>

        <ProgressBar
          currentTime={currentTime}
          duration={duration}
          onSeek={onSeek}
          variant="mobile"
          resetKey={track.id}
        />
      </div>
    </div>
  );
}
