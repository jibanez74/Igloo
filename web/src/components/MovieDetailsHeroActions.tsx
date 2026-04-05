import { useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  Play,
  MoreVertical,
  Info,
  Radio,
  Settings2,
  Pencil,
  Trash2,
  Check,
} from "lucide-react";
import { toast } from "sonner";
import { Spinner } from "@/components/ui/spinner";
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
import { setMovieWatched } from "@/lib/api";
import { MOVIE_WATCH_PROGRESS_KEY } from "@/lib/constants";
import { movieWatchProgressQueryOpts } from "@/lib/query-opts";
import { showActionFailed } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type { ApiResponseType, MovieDetailsHeroActionsProps, MovieWatchProgressType } from "@/types";

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
  const queryClient = useQueryClient();
  const playButtonRef = useRef<HTMLAnchorElement | null>(null);
  const moreOptionsButtonRef = useRef<HTMLButtonElement | null>(null);
  const { data: watchProgressData, isLoading: watchProgressLoading } = useQuery(
    movieWatchProgressQueryOpts(movieId),
  );
  const isWatched =
    watchProgressData?.error === false
      ? Boolean(watchProgressData.data.watched)
      : false;

  const watchedMutation = useMutation({
    mutationFn: (nextWatched: boolean) => setMovieWatched(movieId, nextWatched),
    onMutate: async (nextWatched: boolean) => {
      await queryClient.cancelQueries({
        queryKey: [MOVIE_WATCH_PROGRESS_KEY, movieId],
      });
      const key = [MOVIE_WATCH_PROGRESS_KEY, movieId] as const;
      const previous =
        queryClient.getQueryData<ApiResponseType<MovieWatchProgressType>>(key);

      if (previous?.error === false) {
        queryClient.setQueryData<ApiResponseType<MovieWatchProgressType>>(key, {
          error: false,
          data: {
            ...previous.data,
            progress_sec: nextWatched ? 0 : previous.data.progress_sec,
            watched: nextWatched,
          },
        });
      }

      return { previous };
    },
    onError: (_err, _nextWatched, context) => {
      const key = [MOVIE_WATCH_PROGRESS_KEY, movieId] as const;
      if (context?.previous !== undefined) {
        queryClient.setQueryData(key, context.previous);
      } else {
        void queryClient.invalidateQueries({ queryKey: key });
      }
      showActionFailed(
        "update watched status",
        "Unable to update watched status. Please try again.",
      );
    },
    onSuccess: (res, nextWatched, context) => {
      const key = [MOVIE_WATCH_PROGRESS_KEY, movieId] as const;
      if (res.error) {
        if (context?.previous !== undefined) {
          queryClient.setQueryData(key, context.previous);
        }
        showActionFailed("update watched status", res.message);
        return;
      }

      const previous =
        queryClient.getQueryData<ApiResponseType<MovieWatchProgressType>>(key);
      if (previous?.error === false) {
        queryClient.setQueryData<ApiResponseType<MovieWatchProgressType>>(key, {
          error: false,
          data: {
            ...previous.data,
            progress_sec: nextWatched ? 0 : previous.data.progress_sec,
            watched: res.data.watched,
          },
        });
      }

      void queryClient.invalidateQueries({ queryKey: key });
    },
  });

  return (
    <div className="mt-6 flex flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start">
      <Link
        ref={playButtonRef}
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
      <button
        type="button"
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          watchedMutation.mutate(!isWatched);
        }}
        disabled={watchedMutation.isPending || watchProgressLoading}
        className={cn(
          buttonVariants({ variant: "outline", size: "lg" }),
          "min-h-11 touch-manipulation px-6 font-semibold",
        )}
        aria-label={isWatched ? "Mark movie as unwatched" : "Mark movie as watched"}
        aria-pressed={isWatched}
      >
        {watchedMutation.isPending ? (
          <Spinner className="size-4 text-emerald-400" aria-hidden="true" />
        ) : (
          <>
            <Check
              className={cn(
                "size-4",
                isWatched && "text-emerald-400",
              )}
              aria-hidden="true"
            />
            {isWatched ? "Watched" : "Watch"}
          </>
        )}
      </button>
      <MovieLikeButton movieId={movieId} variant="hero" />
      <DropdownMenu>
        <DropdownMenuTrigger
          ref={moreOptionsButtonRef}
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
        restoreFocusRef={playButtonRef}
      />

      {user?.is_admin && (
        <EditMovieDialog
          movieId={movieId}
          movie={movie}
          open={editOpen}
          onOpenChange={onEditOpenChange}
          restoreFocusRef={moreOptionsButtonRef}
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
