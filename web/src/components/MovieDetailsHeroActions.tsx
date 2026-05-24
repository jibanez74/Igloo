import { lazy, Suspense, useRef, useState } from "react";
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
import { Spinner } from "@/components/ui/spinner";
import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import MovieLikeButton from "@/components/MovieLikeButton";
import { setMovieWatched } from "@/lib/api";
import { MOVIE_WATCH_PROGRESS_KEY } from "@/lib/constants";
import { movieWatchProgressQueryOpts } from "@/lib/query-opts";
import { showActionFailed } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type { ApiResponseType, MovieDetailsHeroActionsProps, MovieWatchProgressType } from "@/types";

const loadPlaybackSettingsDialog = () => import("@/components/PlaybackSettingsDialog");
const loadTechnicalDetailsDialog = () => import("@/components/TechnicalDetailsDialog");
const loadEditMovieDialog = () => import("@/components/EditMovieDialog");
const loadDeleteMovieDialog = () => import("@/components/DeleteMovieDialog");
const loadCreateWatchRoomDialog = () => import("@/components/CreateWatchRoomDialog");

const PlaybackSettingsDialog = lazy(loadPlaybackSettingsDialog);
const TechnicalDetailsDialog = lazy(loadTechnicalDetailsDialog);
const EditMovieDialog = lazy(loadEditMovieDialog);
const DeleteMovieDialog = lazy(loadDeleteMovieDialog);
const CreateWatchRoomDialog = lazy(loadCreateWatchRoomDialog);

const emptyWatchProgress = (): MovieWatchProgressType => ({
  progress_sec: null,
  duration_sec: null,
  watched: false,
  updated_at: null,
});

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
  const [playbackFormResetKey, setPlaybackFormResetKey] = useState(0);
  const [createWatchRoomOpen, setCreateWatchRoomOpen] = useState(false);

  const preloadMoreOptionsDialogs = () => {
    void loadPlaybackSettingsDialog();
    void loadTechnicalDetailsDialog();
    void loadCreateWatchRoomDialog();
    if (user?.is_admin) {
      void loadEditMovieDialog();
      void loadDeleteMovieDialog();
    }
  };

  const handlePlaybackSettingsOpenChange = (next: boolean) => {
    if (next) {
      setPlaybackFormResetKey(k => k + 1);
    }
    onPlaybackSettingsOpenChange(next);
  };
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
      const currentProgress = previous?.error === false
        ? previous.data
        : emptyWatchProgress();

      queryClient.setQueryData<ApiResponseType<MovieWatchProgressType>>(key, {
        error: false,
        data: {
          ...currentProgress,
          progress_sec: nextWatched ? 0 : currentProgress.progress_sec,
          watched: nextWatched,
        },
      });

      return { previous };
    },
    onError: (_err, _nextWatched, context) => {
      const key = [MOVIE_WATCH_PROGRESS_KEY, movieId] as const;
      if (context?.previous !== undefined) {
        queryClient.setQueryData(key, context.previous);
      } else {
        queryClient.setQueryData<ApiResponseType<MovieWatchProgressType>>(key, {
          error: false,
          data: emptyWatchProgress(),
        });
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
        } else {
          queryClient.setQueryData<ApiResponseType<MovieWatchProgressType>>(
            key,
            {
              error: false,
              data: emptyWatchProgress(),
            },
          );
        }
        showActionFailed("update watched status", res.message);
        return;
      }

      const previous =
        queryClient.getQueryData<ApiResponseType<MovieWatchProgressType>>(key);
      const currentProgress = previous?.error === false
        ? previous.data
        : emptyWatchProgress();
      queryClient.setQueryData<ApiResponseType<MovieWatchProgressType>>(key, {
        error: false,
        data: {
          ...currentProgress,
          progress_sec: nextWatched ? 0 : currentProgress.progress_sec,
          watched: res.data.watched,
        },
      });

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
        <Check
          className={cn(
            "size-4",
            isWatched && "text-emerald-400!",
          )}
          aria-hidden="true"
        />
        {isWatched ? "Watched" : "Watch"}
        {watchedMutation.isPending && (
          <Spinner className="size-4 text-emerald-400!" aria-hidden="true" />
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
          onFocus={preloadMoreOptionsDialogs}
          onPointerDown={preloadMoreOptionsDialogs}
          onPointerEnter={preloadMoreOptionsDialogs}
        >
          <MoreVertical className="size-4" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem
            onSelect={() => handlePlaybackSettingsOpenChange(true)}
          >
            <Settings2 className="size-4" aria-hidden="true" />
            Playback Settings
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setCreateWatchRoomOpen(true)}>
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

      {playbackSettingsOpen && (
        <Suspense fallback={null}>
          <PlaybackSettingsDialog
            movieId={movieId}
            open={playbackSettingsOpen}
            onOpenChange={handlePlaybackSettingsOpenChange}
            settings={playbackSettings}
            onSave={onPlaybackSettingsChange}
            restoreFocusRef={playButtonRef}
            formResetKey={playbackFormResetKey}
          />
        </Suspense>
      )}

      {createWatchRoomOpen && (
        <Suspense fallback={null}>
          <CreateWatchRoomDialog
            movieId={movieId}
            movieTitle={movieTitle}
            playbackSettings={playbackSettings}
            open={createWatchRoomOpen}
            onOpenChange={setCreateWatchRoomOpen}
            restoreFocusRef={moreOptionsButtonRef}
          />
        </Suspense>
      )}

      {user?.is_admin && editOpen && (
        <Suspense fallback={null}>
          <EditMovieDialog
            movieId={movieId}
            movie={movie}
            open={editOpen}
            onOpenChange={onEditOpenChange}
            restoreFocusRef={moreOptionsButtonRef}
          />
        </Suspense>
      )}

      {technicalDetailsOpen && (
        <Suspense fallback={null}>
          <TechnicalDetailsDialog
            movieId={movieId}
            open={technicalDetailsOpen}
            onOpenChange={onTechnicalDetailsOpenChange}
          />
        </Suspense>
      )}

      {user?.is_admin && deleteOpen && (
        <Suspense fallback={null}>
          <DeleteMovieDialog
            movieId={movieId}
            movieTitle={movieTitle}
            open={deleteOpen}
            onOpenChange={onDeleteOpenChange}
          />
        </Suspense>
      )}
    </div>
  );
}
