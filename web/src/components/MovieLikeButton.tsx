import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Heart } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { buttonVariants } from "@/components/ui/button";
import { showActionFailed } from "@/lib/toast-helpers";
import { toggleLikeMovie } from "@/lib/api";
import { MOVIES_LIKED_KEY, MOVIE_LIKE_STATUS_KEY } from "@/lib/constants";
import { movieLikeStatusQueryOpts } from "@/lib/query-opts";
import { cn } from "@/lib/utils";
import type { ApiResponseType } from "@/types";

type LikeStatusPayload = { is_liked: boolean };

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
  const { data, isLoading } = useQuery(movieLikeStatusQueryOpts(movieId));
  const isLiked =
    data?.error === false ? Boolean(data.data.is_liked) : false;

  const mutation = useMutation({
    mutationFn: () => toggleLikeMovie(movieId),
    onMutate: async () => {
      await queryClient.cancelQueries({
        queryKey: [MOVIE_LIKE_STATUS_KEY, movieId],
      });
      const key = [MOVIE_LIKE_STATUS_KEY, movieId] as const;
      const previous = queryClient.getQueryData<ApiResponseType<LikeStatusPayload>>(
        key,
      );
      if (previous?.error === false) {
        queryClient.setQueryData<ApiResponseType<LikeStatusPayload>>(key, {
          error: false,
          data: { is_liked: !previous.data.is_liked },
        });
      }
      return { previous };
    },
    onError: (_err, _vars, context) => {
      const key = [MOVIE_LIKE_STATUS_KEY, movieId];
      if (context?.previous !== undefined) {
        queryClient.setQueryData(key, context.previous);
      } else {
        void queryClient.invalidateQueries({ queryKey: key });
      }
      showActionFailed(
        "update like",
        "Unable to update liked status. Please try again.",
      );
    },
    onSuccess: (res, _vars, context) => {
      const key = [MOVIE_LIKE_STATUS_KEY, movieId] as const;
      if (res.error) {
        if (context?.previous !== undefined) {
          queryClient.setQueryData(key, context.previous);
        }
        showActionFailed("update like", res.message);
        return;
      }
      queryClient.setQueryData<ApiResponseType<LikeStatusPayload>>(key, {
        error: false,
        data: { is_liked: res.data.is_liked },
      });
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
          "min-h-11 touch-manipulation px-6 font-semibold",
          className,
        )}
        aria-label={label}
        aria-pressed={isLiked}
      >
        {mutation.isPending ? (
          <Spinner className="size-4 text-amber-400" aria-hidden="true" />
        ) : (
          <>
            <Heart
              className={cn(
                "size-4",
                isLiked && "fill-amber-400 text-amber-400",
              )}
              aria-hidden="true"
            />
            {isLiked ? "Liked" : "Like"}
          </>
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
        "flex size-9 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-sm transition-colors hover:bg-black/70 focus:ring-2 focus:ring-amber-400 focus:outline-none disabled:opacity-60",
        className,
      )}
      aria-label={label}
      aria-pressed={isLiked}
    >
      {mutation.isPending ? (
        <Spinner className="size-4 text-amber-400" />
      ) : (
        <Heart
          className={`size-5 ${isLiked ? "fill-amber-400 text-amber-400" : "text-white"}`}
          aria-hidden="true"
        />
      )}
    </button>
  );
}
