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
export function useTrackLikeToggle(trackId: number) {
  const queryClient = useQueryClient();
  const key = [LIKED_TRACK_IDS_KEY] as const;
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
      queryClient.getQueryData<ApiResponseType<LikedTrackIdsPayload>>(key);
    if (previousIsLiked === undefined || current?.error !== false) {
      void queryClient.invalidateQueries({ queryKey: key });
      return;
    }

    queryClient.setQueryData(
      key,
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
      await queryClient.cancelQueries({ queryKey: key });
      const previous =
        queryClient.getQueryData<ApiResponseType<LikedTrackIdsPayload>>(key);
      if (previous?.error === false) {
        const ids = previous.data.liked_track_ids;
        queryClient.setQueryData(
          key,
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
        queryClient.getQueryData<ApiResponseType<LikedTrackIdsPayload>>(key);
      if (current?.error === false) {
        queryClient.setQueryData(
          key,
          likedIdsResponse(
            setTrackLiked(
              current.data.liked_track_ids,
              trackId,
              res.data.is_liked,
            ),
          ),
        );
      } else {
        void queryClient.invalidateQueries({ queryKey: key });
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
