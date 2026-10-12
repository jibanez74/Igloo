import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Heart } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { buttonVariants } from "@/components/ui/button";
import { showActionFailed } from "@/lib/toast-helpers";
import { toggleLikeMovie } from "@/lib/api";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  MOVIES_LIKED_KEY,
} from "@/lib/constants";
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
  /**
   * `card` — compact circle for poster grids.
   * `hero` — outline button with label (movie details action row).
   */
  variant?: "card" | "hero";
  className?: string;
};

export default function MovieLikeButton({
  movieId,
  variant = "card",
  className = "",
}: MovieLikeButtonProps) {
  const queryClient = useQueryClient();
  const likeQuery = movieLikeStatusQueryOpts(movieId);
  const likeKey = likeQuery.queryKey;
  const { data, isLoading } = useQuery(likeQuery);
  const isLiked =
    data?.error === false ? Boolean(data.data.is_liked) : false;

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
      if (context?.previous !== undefined) {
        queryClient.setQueryData(likeKey, context.previous);
      } else {
        queryClient.setQueryData(likeKey, likeStatusResponse(false));
      }
      showActionFailed(
        "update like",
        "Unable to update liked status. Please try again.",
      );
    },
    onSuccess: (res, _vars, context) => {
      if (res.error) {
        if (context?.previous !== undefined) {
          queryClient.setQueryData(likeKey, context.previous);
        } else {
          queryClient.setQueryData(likeKey, likeStatusResponse(false));
        }
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

  if (variant === "hero") {
    return (
      <button
        type="button"
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          mutation.mutate();
        }}
        disabled={mutation.isPending || isLoading}
        className={cn(
          buttonVariants({ variant: "outline", size: "lg" }),
          "min-h-11 touch-manipulation px-3 font-semibold sm:px-6",
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
          <Spinner className="size-4 text-primary!" aria-hidden="true" />
        )}
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        mutation.mutate();
      }}
      disabled={mutation.isPending || isLoading}
      className={cn(
        MOTION_MICRO_COLORS_CLASS,
        FOCUS_VISIBLE_RING_CLASS,
        "flex size-9 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-sm hover:bg-black/70 disabled:opacity-60",
        className,
      )}
      aria-label={label}
      aria-pressed={isLiked}
    >
      {mutation.isPending ? (
        <Spinner className="size-4 text-primary" />
      ) : (
        <Heart
          className={`size-5 ${isLiked ? "fill-primary text-primary" : "text-white"}`}
          aria-hidden="true"
        />
      )}
    </button>
  );
}
