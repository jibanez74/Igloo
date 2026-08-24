import { useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import { MoreVertical, Plus, Disc3, User, Trash2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import AddToPlaylistDialog from "@/components/music/AddToPlaylistDialog";
import { MOTION_TRACK_MENU_TRIGGER_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type TrackActionsMenuProps = {
  // Required for add to playlist
  trackId: number;
  trackTitle: string;
  // Navigation
  albumId?: number | null;
  albumTitle?: string;
  musicianId?: number | null;
  musicianName?: string;
  // Playlist actions
  canRemoveFromPlaylist?: boolean;
  onRemoveFromPlaylist?: () => void;
};

export default function TrackActionsMenu({
  trackId,
  trackTitle,
  albumId,
  musicianId,
  canRemoveFromPlaylist = false,
  onRemoveFromPlaylist,
}: TrackActionsMenuProps) {
  const [showAddToPlaylist, setShowAddToPlaylist] = useState(false);
  const actionsButtonRef = useRef<HTMLButtonElement | null>(null);

  const hasAlbum = albumId != null && albumId > 0;
  const hasMusician = musicianId != null && musicianId > 0;
  const hasPlaylistActions = canRemoveFromPlaylist && onRemoveFromPlaylist;

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            ref={actionsButtonRef}
            className={cn(
              "flex size-8 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground",
              MOTION_TRACK_MENU_TRIGGER_CLASS,
            )}
            aria-label={`More actions for ${trackTitle}`}
          >
            <MoreVertical className="size-4" aria-hidden="true" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          className="border-border bg-popover text-popover-foreground"
        >
          {/* Add to Playlist - always show */}
          <DropdownMenuItem
            onClick={() => setShowAddToPlaylist(true)}
            className="cursor-pointer hover:bg-accent"
          >
            <Plus className="mr-2 size-4 text-primary" aria-hidden="true" />
            Add to Playlist
          </DropdownMenuItem>

          {(hasAlbum || hasMusician) && (
            <DropdownMenuSeparator className="bg-border" />
          )}

          {hasAlbum && (
            <DropdownMenuItem
              asChild
              className="cursor-pointer hover:bg-accent"
            >
              <Link to="/music/album/$id" params={{ id: albumId.toString() }}>
                <Disc3 className="mr-2 size-4 text-primary" aria-hidden="true" />
                Go to Album
              </Link>
            </DropdownMenuItem>
          )}
          {hasMusician && (
            <DropdownMenuItem
              asChild
              className="cursor-pointer hover:bg-accent"
            >
              <Link
                to="/music/musician/$id"
                params={{ id: musicianId.toString() }}
              >
                <User className="mr-2 size-4 text-primary" aria-hidden="true" />
                Go to Artist
              </Link>
            </DropdownMenuItem>
          )}
          {hasPlaylistActions && (
            <>
              <DropdownMenuSeparator className="bg-border" />
              <DropdownMenuItem
                onClick={onRemoveFromPlaylist}
                variant="destructive"
                className="cursor-pointer"
              >
                <Trash2 className="mr-2 size-4" aria-hidden="true" />
                Remove from Playlist
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      <AddToPlaylistDialog
        open={showAddToPlaylist}
        onOpenChange={setShowAddToPlaylist}
        trackId={trackId}
        trackTitle={trackTitle}
        restoreFocusRef={actionsButtonRef}
      />
    </>
  );
}
