import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Heart } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { showActionFailed } from "@/lib/toast-helpers";
import { toggleLikeMovie } from "@/lib/api";
import { MOVIES_LIKED_KEY } from "@/lib/constants";
import { movieLikeStatusQueryOpts } from "@/lib/query-opts";
import { cn } from "@/lib/utils";
import type { ApiResponseType } from "@/types";

type LikeStatusPayload = { is_liked: boolean };

const likeStatusResponse = (
  isLiked: boolean,
): ApiResponseType<LikeStatusPayload> => ({
  error: false,
  data: { is_liked: isLiked },
});

type MovieLikeButtonProps = {
  movieId: number;
  className?: string;
};

export default function MovieLikeButton({
  movieId,
  className = "",
}: MovieLikeButtonProps) {
  const queryClient = useQueryClient();
  const likeQuery = movieLikeStatusQueryOpts(movieId);
  const likeKey = likeQuery.queryKey;
  const { data, isLoading } = useQuery(likeQuery);
  const isLiked =
    data?.error === false ? Boolean(data.data.is_liked) : false;

  // Undoes the optimistic flip. With nothing cached before it, the real state
  // is unknown, so it refetches rather than claiming "not liked".
  const rollback = (previous: typeof data) => {
    if (previous === undefined) {
      void queryClient.invalidateQueries({ queryKey: likeKey });
      return;
    }
    queryClient.setQueryData(likeKey, previous);
  };

  const mutation = useMutation({
    mutationFn: () => toggleLikeMovie(movieId),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: likeKey });
      const previous = queryClient.getQueryData(likeKey);
      const nextLiked = previous?.error === false
        ? !previous.data.is_liked
        : true;
      queryClient.setQueryData(likeKey, likeStatusResponse(nextLiked));
      return { previous };
    },
    onError: (_err, _vars, context) => {
      rollback(context?.previous);
      showActionFailed(
        "update like",
        "Unable to update liked status. Please try again.",
      );
    },
    onSuccess: (res, _vars, context) => {
      if (res.error) {
        rollback(context?.previous);
        showActionFailed("update like", res);
        return;
      }
      queryClient.setQueryData(likeKey, likeStatusResponse(res.data.is_liked));
      // Keep the movie grid mounted — do not invalidate MOVIES_LIBRARY_KEY (refetch was
      // stealing focus / scrolling). Liked tab list only refetches when that query is active.
      void queryClient.invalidateQueries({ queryKey: [MOVIES_LIKED_KEY] });
    },
  });

  const label = isLiked ? "Unlike this movie" : "Like this movie";

  return (
    <Button
      type="button"
      variant="outline"
      size="xl"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        mutation.mutate();
      }}
      disabled={mutation.isPending || isLoading}
      className={cn(
        "touch-manipulation px-3 font-semibold sm:px-6",
        className,
      )}
      aria-label={label}
      aria-pressed={isLiked}
    >
      <Heart
        className={cn(
          "size-4",
          isLiked && "fill-primary text-primary!",
        )}
        aria-hidden="true"
      />
      {isLiked ? "Liked" : "Like"}
      {mutation.isPending && (
        <Spinner tone="primary" aria-hidden="true" />
      )}
    </Button>
  );
}
