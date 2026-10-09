import type { Ref } from "react";
import { Heart, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { MOTION_LOADING_STATE_CLASS } from "@/lib/constants";
import { pluralize } from "@/lib/format";
import { cn } from "@/lib/utils";

type PlaylistsTabToolbarProps = {
  /** The loaded playlist count; left out while loading and on error. */
  count?: number;
  isLoading: boolean;
  likedLabel: string;
  likedAriaLabel?: string;
  createAriaLabel?: string;
  onShowLiked: () => void;
  onCreate: () => void;
  likedButtonRef?: Ref<HTMLButtonElement>;
  createButtonRef?: Ref<HTMLButtonElement>;
};

/**
 * The music and movie Playlists tabs' toolbar (design-system §3.2): the count
 * on the left, the liked view and "New playlist" on the right. It renders in
 * every state so both buttons stay reachable while the list loads or fails.
 */
export default function PlaylistsTabToolbar({
  count,
  isLoading,
  likedLabel,
  likedAriaLabel,
  createAriaLabel,
  onShowLiked,
  onCreate,
  likedButtonRef,
  createButtonRef,
}: PlaylistsTabToolbarProps) {
  return (
    <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      {isLoading ? (
        <div className="flex h-5 items-center">
          <div className={cn("h-4 w-24 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
        </div>
      ) : (
        count !== undefined && (
          <span className="text-sm text-muted-foreground">
            {pluralize(count, "playlist")}
          </span>
        )
      )}
      <div className="flex flex-wrap gap-2 sm:ml-auto">
        <Button
          ref={likedButtonRef}
          variant="outline"
          onClick={onShowLiked}
          className="min-h-10 rounded-full"
          aria-label={likedAriaLabel}
        >
          <Heart className="size-4 shrink-0" aria-hidden="true" />
          {likedLabel}
        </Button>
        <Button
          ref={createButtonRef}
          variant="accent-pill"
          onClick={onCreate}
          className="min-h-10"
          aria-label={createAriaLabel}
        >
          <Plus className="size-4 shrink-0" aria-hidden="true" />
          New playlist
        </Button>
      </div>
    </div>
  );
}
