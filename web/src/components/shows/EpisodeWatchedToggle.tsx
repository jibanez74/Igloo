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
import type { ApiResponseType, ShowSeasonEpisodesDataType } from "@/types";

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
 * truth on the page, so the toggle updates that cached payload optimistically
 * (the row re-renders at once), restores it on failure, and refreshes every
 * watch query that shows the episode once the server has answered.
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

  const setSeasonWatched = (nextWatched: boolean) => {
    queryClient.setQueryData<ApiResponseType<ShowSeasonEpisodesDataType>>(
      seasonKey,
      (previous) => {
        if (!previous || previous.error) return previous;
        return {
          ...previous,
          data: {
            ...previous.data,
            episodes: previous.data.episodes.map((episode) =>
              episode.id === episodeId
                ? {
                    ...episode,
                    watched: nextWatched,
                    // Marking watched clears the position, as the server does.
                    progress_sec: nextWatched
                      ? { Float64: 0, Valid: false }
                      : episode.progress_sec,
                  }
                : episode,
            ),
          },
        };
      },
    );
  };

  const mutation = useMutation({
    mutationFn: (nextWatched: boolean) =>
      setMediaWatched(episodeMediaRef(episodeId), nextWatched),
    onMutate: async (nextWatched: boolean) => {
      await queryClient.cancelQueries({ queryKey: seasonKey });
      const previous =
        queryClient.getQueryData<ApiResponseType<ShowSeasonEpisodesDataType>>(
          seasonKey,
        );
      setSeasonWatched(nextWatched);
      return { previous };
    },
    onSuccess: (res, _nextWatched, context) => {
      if (res.error) {
        queryClient.setQueryData(seasonKey, context?.previous);
        showActionFailed("update watched status", res);
        return;
      }

      setSeasonWatched(res.data.watched);
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
