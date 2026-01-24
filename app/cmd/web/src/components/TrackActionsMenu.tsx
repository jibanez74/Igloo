import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { MoreVertical, Plus, Disc3, User, Trash2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import AddToPlaylistDialog from "@/components/AddToPlaylistDialog";

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

  const hasAlbum = albumId != null && albumId > 0;
  const hasMusician = musicianId != null && musicianId > 0;
  const hasPlaylistActions = canRemoveFromPlaylist && onRemoveFromPlaylist;

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            className="flex size-8 shrink-0 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-700 hover:text-white"
            aria-label="More actions"
          >
            <MoreVertical className="size-4" aria-hidden="true" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          className="border-slate-700 bg-slate-800 text-white"
        >
          {/* Add to Playlist - always show */}
          <DropdownMenuItem
            onClick={() => setShowAddToPlaylist(true)}
            className="cursor-pointer hover:bg-slate-700"
          >
            <Plus className="mr-2 size-4 text-amber-400" aria-hidden="true" />
            Add to Playlist
          </DropdownMenuItem>

          {(hasAlbum || hasMusician) && (
            <DropdownMenuSeparator className="bg-slate-700" />
          )}

          {hasAlbum && (
            <DropdownMenuItem
              asChild
              className="cursor-pointer hover:bg-slate-700"
            >
              <Link to="/music/album/$id" params={{ id: albumId.toString() }}>
                <Disc3 className="mr-2 size-4 text-amber-400" aria-hidden="true" />
                Go to Album
              </Link>
            </DropdownMenuItem>
          )}
          {hasMusician && (
            <DropdownMenuItem
              asChild
              className="cursor-pointer hover:bg-slate-700"
            >
              <Link
                to="/music/musician/$id"
                params={{ id: musicianId.toString() }}
              >
                <User className="mr-2 size-4 text-amber-400" aria-hidden="true" />
                Go to Artist
              </Link>
            </DropdownMenuItem>
          )}
          {hasPlaylistActions && (
            <>
              <DropdownMenuSeparator className="bg-slate-700" />
              <DropdownMenuItem
                onClick={onRemoveFromPlaylist}
                className="cursor-pointer text-red-400 hover:bg-slate-700 hover:text-red-300 focus:text-red-300"
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
      />
    </>
  );
}
