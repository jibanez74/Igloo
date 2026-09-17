import { Heart } from "lucide-react";
import { useLikeButtonState } from "@/hooks/useTrackLikeToggle";
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
  const likeButton = useLikeButtonState(trackId, trackTitle);

  return (
    <button
      type="button"
      onClick={likeButton.toggle}
      aria-disabled={likeButton.isDisabled || undefined}
      aria-busy={likeButton.ariaBusy}
      className={cn(
        PLAYER_ICON_BUTTON_CLASS,
        variant === "expanded"
          ? "size-10 shrink-0 hover:bg-accent/50"
          : "size-10 shrink-0 hover:bg-accent",
        likeButton.isLiked
          ? "text-destructive hover:text-destructive"
          : "hover:text-destructive",
        likeButton.isDisabled && "cursor-not-allowed opacity-50",
      )}
      aria-label={likeButton.ariaLabel}
      aria-pressed={likeButton.ariaPressed}
    >
      <Heart
        className={cn(
          variant === "expanded" ? "size-5" : "size-4",
          likeButton.isLiked && "fill-current",
        )}
        aria-hidden="true"
      />
    </button>
  );
}
