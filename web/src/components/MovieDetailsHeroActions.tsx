import { Link } from "@tanstack/react-router";
import {
  Play,
  MoreVertical,
  Info,
  Radio,
  Settings2,
  Pencil,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import PlaybackSettingsDialog from "@/components/PlaybackSettingsDialog";
import TechnicalDetailsDialog from "@/components/TechnicalDetailsDialog";
import EditMovieDialog from "@/components/EditMovieDialog";
import DeleteMovieDialog from "@/components/DeleteMovieDialog";
import MovieLikeButton from "@/components/MovieLikeButton";
import { cn } from "@/lib/utils";
import type { MovieDetailsHeroActionsProps } from "@/types";

export default function MovieDetailsHeroActions({
  movieId,
  movie,
  movieTitle,
  user,
  playbackSettings,
  onPlaybackSettingsChange,
  playbackSettingsOpen,
  onPlaybackSettingsOpenChange,
  technicalDetailsOpen,
  onTechnicalDetailsOpenChange,
  editOpen,
  onEditOpenChange,
  deleteOpen,
  onDeleteOpenChange,
}: MovieDetailsHeroActionsProps) {
  return (
    <div className="mt-6 flex flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start">
      <Link
        to="/movies/$id/play"
        params={{ id: String(movieId) }}
        search={{
          mode: playbackSettings.mode,
          audio_track: playbackSettings.audioTrack,
          subtitle_track: playbackSettings.subtitleTrack ?? undefined,
        }}
        className={cn(
          buttonVariants({ variant: "accent", size: "lg" }),
          "min-h-11 min-w-34 touch-manipulation sm:min-w-0",
        )}
      >
        <Play className="size-4 fill-current" aria-hidden="true" />
        Play
      </Link>
      <MovieLikeButton movieId={movieId} variant="hero" />
      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(
            buttonVariants({ variant: "outline", size: "lg" }),
            "min-h-11 touch-manipulation",
          )}
          aria-label="More options"
        >
          <MoreVertical className="size-4" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem onSelect={() => onPlaybackSettingsOpenChange(true)}>
            <Settings2 className="size-4" aria-hidden="true" />
            Playback Settings
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => toast.info("Coming soon")}>
            <Radio className="size-4" aria-hidden="true" />
            Watch Together
          </DropdownMenuItem>
          {user?.is_admin && (
            <DropdownMenuItem onSelect={() => onEditOpenChange(true)}>
              <Pencil className="size-4" aria-hidden="true" />
              Edit
            </DropdownMenuItem>
          )}
          <DropdownMenuItem
            onSelect={() => onTechnicalDetailsOpenChange(true)}
          >
            <Info className="size-4" aria-hidden="true" />
            Technical Details
          </DropdownMenuItem>
          {user?.is_admin && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={() => onDeleteOpenChange(true)}
                className="text-red-400 focus:text-red-300"
              >
                <Trash2 className="size-4" aria-hidden="true" />
                Delete
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      <PlaybackSettingsDialog
        movieId={movieId}
        open={playbackSettingsOpen}
        onOpenChange={onPlaybackSettingsOpenChange}
        settings={playbackSettings}
        onSave={onPlaybackSettingsChange}
      />

      {user?.is_admin && (
        <EditMovieDialog
          movieId={movieId}
          movie={movie}
          open={editOpen}
          onOpenChange={onEditOpenChange}
        />
      )}

      <TechnicalDetailsDialog
        movieId={movieId}
        open={technicalDetailsOpen}
        onOpenChange={onTechnicalDetailsOpenChange}
      />

      {user?.is_admin && (
        <DeleteMovieDialog
          movieId={movieId}
          movieTitle={movieTitle}
          open={deleteOpen}
          onOpenChange={onDeleteOpenChange}
        />
      )}
    </div>
  );
}
