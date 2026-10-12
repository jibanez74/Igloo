import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { buttonVariants } from "@/components/ui/button";
import { setMediaWatched } from "@/lib/api";
import { episodeMediaRef } from "@/lib/media-ref";
import { showSeasonEpisodesQueryOpts } from "@/lib/query-opts";
import { showActionFailed } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import { refreshWatchQueries } from "@/lib/video-playback-exit";
import type { ShowSeasonEpisodesDataType } from "@/types";

type EpisodeRow = ShowSeasonEpisodesDataType["episodes"][number];
type EpisodeWatchState = Pick<EpisodeRow, "watched" | "progress_sec">;

type EpisodeWatchedToggleProps = {
  showId: number;
  seasonNumber: number;
  episodeId: number;
  watched: boolean;
  /** "S1 E3" — names the episode in the accessible label. */
  episodeCode: string;
};

/**
 * Marks one episode watched or unwatched. The season list is the source of
 * truth on the page, so the toggle updates its own row of that cached payload
 * optimistically (the row re-renders at once), restores only that row on
 * failure (another row's toggle may be in flight), and refreshes every watch
 * query that shows the episode once the server has answered.
 */
export default function EpisodeWatchedToggle({
  showId,
  seasonNumber,
  episodeId,
  watched,
  episodeCode,
}: EpisodeWatchedToggleProps) {
  const queryClient = useQueryClient();
  const seasonKey = showSeasonEpisodesQueryOpts(showId, seasonNumber).queryKey;

  // The cached row of this episode, read fresh each time so the other rows
  // keep whatever their own toggles wrote meanwhile.
  const episodeRow = () => {
    const season = queryClient.getQueryData(seasonKey);
    return season?.error === false
      ? season.data.episodes.find((episode) => episode.id === episodeId)
      : undefined;
  };

  const setEpisodeRow = (state: EpisodeWatchState) => {
    const season = queryClient.getQueryData(seasonKey);
    if (season?.error !== false) return;
    queryClient.setQueryData(seasonKey, {
      ...season,
      data: {
        ...season.data,
        episodes: season.data.episodes.map((episode) =>
          episode.id === episodeId ? { ...episode, ...state } : episode,
        ),
      },
    });
  };

  const setEpisodeWatched = (nextWatched: boolean) => {
    const row = episodeRow();
    if (!row) return;
    setEpisodeRow({
      watched: nextWatched,
      // Marking watched clears the position, as the server does.
      progress_sec: nextWatched ? { Float64: 0, Valid: false } : row.progress_sec,
    });
  };

  // With no cached row to restore, the season refetches instead.
  const rollback = (previous: EpisodeWatchState | undefined) => {
    if (previous) {
      setEpisodeRow(previous);
    } else {
      void queryClient.invalidateQueries({ queryKey: seasonKey });
    }
  };

  const mutation = useMutation({
    mutationFn: (nextWatched: boolean) =>
      setMediaWatched(episodeMediaRef(episodeId), nextWatched),
    onMutate: async (nextWatched: boolean) => {
      await queryClient.cancelQueries({ queryKey: seasonKey });
      const row = episodeRow();
      setEpisodeWatched(nextWatched);
      return {
        previous: row
          ? { watched: row.watched, progress_sec: row.progress_sec }
          : undefined,
      };
    },
    onError: (_error, _nextWatched, context) => {
      rollback(context?.previous);
      showActionFailed("update watched status");
    },
    onSuccess: (res, _nextWatched, context) => {
      if (res.error) {
        rollback(context?.previous);
        showActionFailed("update watched status", res);
        return;
      }

      setEpisodeWatched(res.data.watched);
      // The season rows, the episode's own progress entry and Home's
      // continue-watching row all show this episode's state.
      void refreshWatchQueries(queryClient, episodeMediaRef(episodeId), {
        refetchType: "active",
      });
    },
  });

  return (
    <button
      type="button"
      onClick={() => mutation.mutate(!watched)}
      disabled={mutation.isPending}
      className={cn(
        buttonVariants({ variant: "ghost", size: "icon" }),
        "min-h-10 min-w-10 touch-manipulation text-muted-foreground",
        watched && "text-success",
      )}
      aria-label={
        watched
          ? `Mark ${episodeCode} as unwatched`
          : `Mark ${episodeCode} as watched`
      }
      aria-pressed={watched}
    >
      {mutation.isPending ? (
        <Spinner className="size-4" aria-hidden="true" />
      ) : (
        <Check className="size-4" aria-hidden="true" />
      )}
    </button>
  );
}
