import { Link } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import { Film, Play } from "lucide-react";
import {
  CARD_ACTION_REVEAL_CLASS,
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_OVERLAY_REVEAL_CLASS,
  CARD_SURFACE_CLASS,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { LatestMovieType } from "@/types";

type MovieCardProps = {
  movie: LatestMovieType;
};

export default function MovieCard({ movie }: MovieCardProps) {
  const { id, title, poster_path, year } = movie;
  const queryClient = useQueryClient();

  const handlePrefetch = () =>
    queryClient.prefetchQuery(libraryMovieDetailsQueryOpts(id));

  const ariaTitle = year.Valid ? `${title} ${year.Int64}` : title;

  const posterUrl =
    poster_path.Valid && poster_path.String !== ""
      ? buildTmdbImageUrl(poster_path.String, TMDB_POSTER_SIZE)
      : "";

  return (
    <article
      className={cn(
        CARD_INTERACTIVE_SURFACE_CLASS,
        CARD_SURFACE_CLASS,
        "min-w-0 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background",
      )}
      onMouseEnter={handlePrefetch}
      onFocus={handlePrefetch}
    >
      <div className="relative">
        <Link
          to="/movies/$id"
          params={{ id: String(id) }}
          className="block rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
          aria-label={ariaTitle}
        >
          {/* Poster with 2:3 aspect ratio (standard movie poster) */}
          <div className="relative aspect-2/3 bg-muted">
            {posterUrl ? (
              <img
                src={posterUrl}
                alt=""
                width={500}
                height={750}
                loading="lazy"
                decoding="async"
                fetchPriority="low"
                className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
              />
            ) : (
              <div className="flex size-full items-center justify-center">
                <Film className="size-10 text-muted-foreground" aria-hidden="true" />
              </div>
            )}
            {/* Overlay - appears on hover/focus */}
            <div
              className={cn(
                CARD_OVERLAY_REVEAL_CLASS,
                "absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100",
              )}
              aria-hidden="true"
            />
            {/* Gradient overlay for text readability */}
            <div className="absolute inset-x-0 bottom-0 h-28 bg-linear-to-t from-black/90 via-black/50 to-transparent" />
          </div>
          {/* Movie info */}
          <div className="absolute inset-x-0 bottom-0 p-3">
            <h3 className="line-clamp-2 text-sm/tight font-semibold text-white drop-shadow-lg">
              {title}
            </h3>
            {year.Valid && (
              <p className="mt-0.5 text-xs text-white/80 drop-shadow-lg">
                {year.Int64}
              </p>
            )}
          </div>
        </Link>
      </div>

      {/* Play button - goes to play page without opening details */}
      <Link
        to="/movies/$id/play"
        params={{ id: String(id) }}
        className={cn(
          CARD_ACTION_REVEAL_CLASS,
          "absolute top-1/2 left-1/2 z-10 flex size-14 -translate-1/2 scale-90 items-center justify-center rounded-full bg-primary text-primary-foreground opacity-0 shadow-lg shadow-black/30 group-focus-within:scale-100 group-focus-within:opacity-100 group-hover:scale-100 group-hover:opacity-100 hover:bg-primary/90 focus:scale-100 focus:opacity-100 focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
        )}
        aria-label={`Play ${ariaTitle}`}
      >
        <Play className="size-7 fill-current" aria-hidden="true" />
      </Link>
    </article>
  );
}
