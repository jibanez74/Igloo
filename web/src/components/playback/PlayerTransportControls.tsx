import { Pause, Play, SkipBack, SkipForward } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import {
  PLAYER_ICON_BUTTON_CLASS,
  PLAYER_PRIMARY_BUTTON_CLASS,
  PLAYER_TRANSPORT_INERT_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type TransportVariant = "expanded" | "minimized";

export type PlayerTransportProps = {
  variant: TransportVariant;
  isPlaying: boolean;
  isLoading: boolean;
  hasNext: boolean;
  previousAriaLabel: string;
  nextAriaLabel: string;
  playPauseAriaLabel: string;
  onPrevious: () => void;
  onTogglePlay: () => void;
  onNext: () => void;

  // The fullscreen dialog focuses this on open (see AudioPlayer).
  playPauseButtonRef?: React.RefObject<HTMLButtonElement | null>;
};

// Variant-specific sizing; the semantics are identical in both chromes.
const variantStyles: Record<
  TransportVariant,
  {
    group: string;
    skipButton: string;
    skipIcon: string;
    playButton: string;
    playIcon: string;
  }
> = {
  expanded: {
    group: "flex items-center gap-6",
    skipButton: "size-14 hover:bg-accent/50",
    skipIcon: "size-6",
    playButton: "size-20 shadow-xl shadow-primary/30",
    playIcon: "size-8",
  },
  minimized: {
    group: "flex items-center gap-2",
    skipButton: "size-10 hover:bg-accent",
    skipIcon: "size-4",
    playButton: "size-12 shadow-lg shadow-primary/20",
    playIcon: "size-5",
  },
};

// Previous / play-pause / next. Neither skip button is ever `disabled` —
// that drops it from the VoiceOver focus order — so "no next track" is
// carried by aria-disabled plus the inert styling (design-system §1.7).
export default function PlayerTransportControls({
  variant,
  isPlaying,
  isLoading,
  hasNext,
  previousAriaLabel,
  nextAriaLabel,
  playPauseAriaLabel,
  onPrevious,
  onTogglePlay,
  onNext,
  playPauseButtonRef,
}: PlayerTransportProps) {
  const styles = variantStyles[variant];

  return (
    <div className={styles.group} role="group" aria-label="Playback controls">
      <button
        type="button"
        onClick={onPrevious}
        className={cn(PLAYER_ICON_BUTTON_CLASS, styles.skipButton)}
        aria-label={previousAriaLabel}
      >
        <SkipBack className={styles.skipIcon} aria-hidden="true" />
      </button>

      <button
        type="button"
        ref={playPauseButtonRef}
        onClick={onTogglePlay}
        aria-disabled={isLoading || undefined}
        aria-busy={isLoading || undefined}
        className={cn(PLAYER_PRIMARY_BUTTON_CLASS, styles.playButton)}
        aria-label={isLoading ? "Loading" : playPauseAriaLabel}
      >
        {isLoading ? (
          <Spinner className={styles.playIcon} />
        ) : isPlaying ? (
          <Pause className={cn(styles.playIcon, "fill-current")} aria-hidden="true" />
        ) : (
          <Play className={cn(styles.playIcon, "fill-current")} aria-hidden="true" />
        )}
      </button>

      <button
        type="button"
        onClick={onNext}
        aria-disabled={!hasNext}
        className={cn(
          PLAYER_ICON_BUTTON_CLASS,
          PLAYER_TRANSPORT_INERT_CLASS,
          styles.skipButton,
        )}
        aria-label={nextAriaLabel}
      >
        <SkipForward className={styles.skipIcon} aria-hidden="true" />
      </button>
    </div>
  );
}
