import { Heart } from "lucide-react";
import { useTrackLikeToggle } from "@/hooks/useTrackLikeToggle";
import { PLAYER_ICON_BUTTON_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type PlayerLikeButtonProps = {
  trackId: number;
  trackTitle: string;
  variant: "expanded" | "minimized";
};

export default function PlayerLikeButton({
  trackId,
  trackTitle,
  variant,
}: PlayerLikeButtonProps) {
  const likeToggle = useTrackLikeToggle(trackId);
  const isDisabled = !likeToggle.isReady || likeToggle.isPending;
  const ariaLabel = likeToggle.isReady
    ? likeToggle.isLiked
      ? `Remove ${trackTitle} from liked`
      : `Add ${trackTitle} to liked`
    : likeToggle.isStatusPending
      ? `Loading liked status for ${trackTitle}`
      : `Liked status unavailable for ${trackTitle}`;

  return (
    <button
      type="button"
      onClick={() => {
        if (isDisabled) return;
        likeToggle.toggle();
      }}
      aria-disabled={isDisabled || undefined}
      aria-busy={
        likeToggle.isStatusPending || likeToggle.isPending || undefined
      }
      className={cn(
        PLAYER_ICON_BUTTON_CLASS,
        variant === "expanded"
          ? "size-10 hover:bg-accent/50"
          : "size-10 shrink-0 hover:bg-accent",
        likeToggle.isLiked
          ? "text-destructive hover:text-destructive"
          : "hover:text-destructive",
        isDisabled && "cursor-not-allowed opacity-50",
      )}
      aria-label={ariaLabel}
      aria-pressed={likeToggle.isReady ? likeToggle.isLiked : undefined}
    >
      <Heart
        className={cn(
          variant === "expanded" ? "size-5" : "size-4",
          likeToggle.isLiked && "fill-current",
        )}
        aria-hidden="true"
      />
    </button>
  );
}
