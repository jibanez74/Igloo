import { Link } from "@tanstack/react-router";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type TrackActionsMenuProps = {
  albumId?: number | null;
  albumTitle?: string;
  musicianId?: number | null;
  musicianName?: string;
};

export default function TrackActionsMenu({
  albumId,
  musicianId,
}: TrackActionsMenuProps) {
  const hasAlbum = albumId != null && albumId > 0;
  const hasMusician = musicianId != null && musicianId > 0;

  // Don't render if no actions available
  if (!hasAlbum && !hasMusician) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          className="flex size-8 shrink-0 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-700 hover:text-white"
          aria-label="More actions"
        >
          <i className="fa-solid fa-ellipsis-vertical" aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="border-slate-700 bg-slate-800 text-white"
      >
        {hasAlbum && (
          <DropdownMenuItem
            asChild
            className="cursor-pointer hover:bg-slate-700"
          >
            <Link
              to="/music/album/$id"
              params={{ id: albumId.toString() }}
            >
              <i
                className="fa-solid fa-compact-disc mr-2 text-amber-400"
                aria-hidden="true"
              />
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
              <i
                className="fa-solid fa-user mr-2 text-amber-400"
                aria-hidden="true"
              />
              Go to Artist
            </Link>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
