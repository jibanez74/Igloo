import { useQuery } from "@tanstack/react-query";
import { Heart } from "lucide-react";
import { useTrackLikeToggle } from "@/hooks/useTrackLikeToggle";
import { PLAYER_ICON_BUTTON_CLASS } from "@/lib/constants";
import { likedTrackIdsQueryOpts } from "@/lib/query-opts";
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
  const { data } = useQuery(likedTrackIdsQueryOpts());
  const isLiked =
    data?.error === false && data.data.liked_track_ids.includes(trackId);

  const mutation = useTrackLikeToggle();

  return (
    <button
      type="button"
      onClick={() => {
        if (mutation.isPending) return;
        mutation.mutate(trackId);
      }}
      aria-disabled={mutation.isPending || undefined}
      aria-busy={mutation.isPending || undefined}
      className={cn(
        PLAYER_ICON_BUTTON_CLASS,
        variant === "expanded"
          ? "size-10 hover:bg-accent/50"
          : "size-10 shrink-0 hover:bg-accent",
        isLiked
          ? "text-destructive hover:text-destructive"
          : "hover:text-destructive",
      )}
      aria-label={
        isLiked
          ? `Remove ${trackTitle} from liked`
          : `Add ${trackTitle} to liked`
      }
      aria-pressed={isLiked}
    >
      <Heart
        className={cn(
          variant === "expanded" ? "size-5" : "size-4",
          isLiked && "fill-current",
        )}
        aria-hidden="true"
      />
    </button>
  );
}
