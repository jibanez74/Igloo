import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toggleLikeTrack } from "@/lib/api";
import { LIKED_TRACK_IDS_KEY, LIKED_TRACKS_KEY } from "@/lib/constants";
import { showActionFailed } from "@/lib/toast-helpers";
import type { ApiResponseType } from "@/types";

type LikedTrackIdsPayload = { liked_track_ids: number[] };

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
 * Shared like/unlike mutation for tracks. Optimistically toggles the track's
 * membership in the cached liked-ids list (the single source of liked state
 * for track rows and the audio player), rolls back on failure, and reconciles
 * with the server response on success.
 */
export function useTrackLikeToggle() {
  const queryClient = useQueryClient();
  const key = [LIKED_TRACK_IDS_KEY] as const;

  const rollback = (
    previous: ApiResponseType<LikedTrackIdsPayload> | undefined,
  ) => {
    if (previous !== undefined) {
      queryClient.setQueryData(key, previous);
    } else {
      void queryClient.invalidateQueries({ queryKey: key });
    }
  };

  return useMutation({
    mutationFn: (trackId: number) => toggleLikeTrack(trackId),
    onMutate: async (trackId: number) => {
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
      return { previous };
    },
    onError: (_err, _trackId, context) => {
      rollback(context?.previous);
      showActionFailed(
        "update like",
        "Unable to update liked status. Please try again.",
      );
    },
    onSuccess: (res, trackId, context) => {
      if (res.error) {
        rollback(context?.previous);
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
      }
      void queryClient.invalidateQueries({ queryKey: [LIKED_TRACKS_KEY] });
    },
  });
}
