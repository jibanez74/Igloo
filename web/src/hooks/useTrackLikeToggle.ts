import {
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { toggleLikeTrack } from "@/lib/api";
import { LIKED_TRACK_IDS_KEY, LIKED_TRACKS_KEY } from "@/lib/constants";
import { likedTrackIdsQueryOpts } from "@/lib/query-opts";
import { showActionFailed } from "@/lib/toast-helpers";
import type { ApiResponseType } from "@/types";

type LikedTrackIdsPayload = { liked_track_ids: number[] };
const TRACK_LIKE_MUTATION_KEY = "track-like-toggle";
const LIKED_IDS_QUERY_KEY = [LIKED_TRACK_IDS_KEY] as const;

const likedIdsResponse = (
  ids: number[],
): ApiResponseType<LikedTrackIdsPayload> => ({
  error: false,
  data: { liked_track_ids: ids },
});

function setTrackLiked(ids: number[], trackId: number, isLiked: boolean) {
  const without = ids.filter(id => id !== trackId);
  return isLiked ? [...without, trackId] : without;
}

/**
 * Shared like/unlike controller for a track. The liked-ids cache is the single
 * source of liked state for track rows and the audio player. Mutations are
 * keyed per track so duplicate controls share their pending state.
 */
function useTrackLikeToggle(trackId: number) {
  const queryClient = useQueryClient();
  const mutationKey = [TRACK_LIKE_MUTATION_KEY, trackId] as const;
  const likedIdsQuery = useQuery({
    ...likedTrackIdsQueryOpts(),
    select: response =>
      response.error
        ? undefined
        : response.data.liked_track_ids.includes(trackId),
  });
  const isLiked = likedIdsQuery.data;
  const isReady = typeof isLiked === "boolean";
  const pendingCount = useIsMutating({ mutationKey, exact: true });

  const rollback = (previousIsLiked: boolean | undefined) => {
    const current =
      queryClient.getQueryData<ApiResponseType<LikedTrackIdsPayload>>(LIKED_IDS_QUERY_KEY);
    if (previousIsLiked === undefined || current?.error !== false) {
      void queryClient.invalidateQueries({ queryKey: LIKED_IDS_QUERY_KEY });
      return;
    }

    queryClient.setQueryData(
      LIKED_IDS_QUERY_KEY,
      likedIdsResponse(
        setTrackLiked(
          current.data.liked_track_ids,
          trackId,
          previousIsLiked,
        ),
      ),
    );
  };

  const mutation = useMutation({
    mutationKey,
    mutationFn: () => toggleLikeTrack(trackId),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: LIKED_IDS_QUERY_KEY });
      const previous =
        queryClient.getQueryData<ApiResponseType<LikedTrackIdsPayload>>(LIKED_IDS_QUERY_KEY);
      if (previous?.error === false) {
        const ids = previous.data.liked_track_ids;
        queryClient.setQueryData(
          LIKED_IDS_QUERY_KEY,
          likedIdsResponse(
            setTrackLiked(ids, trackId, !ids.includes(trackId)),
          ),
        );
      }
      return {
        previousIsLiked:
          previous?.error === false
            ? previous.data.liked_track_ids.includes(trackId)
            : undefined,
      };
    },
    onError: (_err, _variables, context) => {
      rollback(context?.previousIsLiked);
      showActionFailed(
        "update like",
        "Unable to update liked status. Please try again.",
      );
    },
    onSuccess: (res, _variables, context) => {
      if (res.error) {
        rollback(context?.previousIsLiked);
        showActionFailed("update like", res.message);
        return;
      }
      const current =
        queryClient.getQueryData<ApiResponseType<LikedTrackIdsPayload>>(LIKED_IDS_QUERY_KEY);
      if (current?.error === false) {
        queryClient.setQueryData(
          LIKED_IDS_QUERY_KEY,
          likedIdsResponse(
            setTrackLiked(
              current.data.liked_track_ids,
              trackId,
              res.data.is_liked,
            ),
          ),
        );
      } else {
        void queryClient.invalidateQueries({ queryKey: LIKED_IDS_QUERY_KEY });
      }
      void queryClient.invalidateQueries({ queryKey: [LIKED_TRACKS_KEY] });
    },
  });

  const toggle = () => {
    if (
      !isReady ||
      queryClient.isMutating({ mutationKey, exact: true }) > 0
    ) {
      return;
    }
    mutation.mutate();
  };

  return {
    isLiked,
    isReady,
    isStatusPending: likedIdsQuery.isPending,
    isPending: pendingCount > 0,
    toggle,
  };
}

/**
 * Render-ready state for a like button. Every like control must derive its
 * disabled/aria state from here so duplicate controls stay consistent — use
 * aria-disabled (never native disabled) to keep the button in the focus order
 * while pending.
 */
export function useLikeButtonState(trackId: number, trackTitle: string) {
  const { isLiked, isReady, isStatusPending, isPending, toggle } =
    useTrackLikeToggle(trackId);
  const isDisabled = !isReady || isPending;
  const ariaLabel = isReady
    ? isLiked
      ? `Remove ${trackTitle} from liked`
      : `Add ${trackTitle} to liked`
    : isStatusPending
      ? `Loading liked status for ${trackTitle}`
      : `Liked status unavailable for ${trackTitle}`;

  return {
    isLiked,
    isDisabled,
    ariaLabel,
    ariaPressed: isReady ? isLiked : undefined,
    ariaBusy: isStatusPending || isPending || undefined,
    toggle: () => {
      if (isDisabled) return;
      toggle();
    },
  };
}
