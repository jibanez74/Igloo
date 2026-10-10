import { useRef, type RefObject } from "react";
import { MoreVertical, Trash2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type PlaylistMovieMenuProps = {
  movieTitle: string;
  onRemove: () => void;
  /** Held while this movie's removal is in flight, so it cannot fire twice. */
  disabled?: boolean;
  /**
   * Where focus goes when the menu closes with its trigger disabled or gone
   * with the card, so a keyboard user is not dropped at the top of the document.
   */
  fallbackFocusRef?: RefObject<HTMLElement | null>;
};

/**
 * The menu a movie carries on a playlist page it can be edited from. It
 * rides in PosterCard's actions corner, so its trigger is readable over any
 * poster and never part of the poster link.
 */
export default function PlaylistMovieMenu({
  movieTitle,
  onRemove,
  disabled = false,
  fallbackFocusRef,
}: PlaylistMovieMenuProps) {
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          ref={triggerRef}
          disabled={disabled}
          aria-label={`More actions for ${movieTitle}`}
          className={cn(
            "flex size-9 items-center justify-center rounded-full bg-black/60 text-white shadow-md outline-hidden hover:bg-black/80 disabled:opacity-50",
            FOCUS_VISIBLE_RING_CLASS,
            MOTION_MICRO_COLORS_CLASS,
          )}
        >
          <MoreVertical className="size-4" aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        // Removal disables the trigger while pending and can unmount the
        // card before the menu closes. Either way, restore to the heading
        // because the trigger cannot receive focus.
        onCloseAutoFocus={event => {
          event.preventDefault();
          const trigger = triggerRef.current;
          focusDialogRestoreTarget(
            trigger?.disabled ? null : trigger,
            fallbackFocusRef?.current,
          );
        }}
      >
        <DropdownMenuItem onSelect={onRemove} variant="destructive">
          <Trash2 className="size-4" aria-hidden="true" />
          Remove from Playlist
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
