import { Link } from "@tanstack/react-router";
import { Film, Star } from "lucide-react";
import {
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_SURFACE_CLASS,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { TheaterMovieType } from "@/types";

type MovieCardProps = {
  movie: TheaterMovieType;
};

export default function MovieCard({ movie }: MovieCardProps) {
  const { id, title, poster_path, vote_average, release_date } = movie;

  const posterUrl = poster_path
    ? buildTmdbImageUrl(poster_path, TMDB_POSTER_SIZE)
    : "";

  const rating = vote_average ? vote_average.toFixed(1) : null;
  const year = release_date ? new Date(release_date).getFullYear() : null;

  const getRatingColor = (score: number) => {
    if (score >= 7) return "bg-aurora text-aurora-foreground"; // Strong
    if (score >= 5) return "bg-aurora/80 text-aurora-foreground"; // Mid
    return "bg-muted text-foreground"; // Low
  };

  return (
    <article
      className={cn(
        CARD_INTERACTIVE_SURFACE_CLASS,
        CARD_SURFACE_CLASS,
      )}
    >
      <Link
        to="/movies/in-theaters/$id"
        params={{ id: id.toString() }}
        className="block rounded-xl outline-hidden focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
        aria-label={`${title}${year ? `, ${year}` : ""}${rating ? `, rated ${rating} out of 10` : ""}`}
      >
        {/* Poster with 2:3 aspect ratio (standard movie poster) */}
        <div className="relative aspect-2/3 bg-muted">
          {posterUrl ? (
            <img
              src={posterUrl}
              alt=""
              width="500"
              height="750"
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

          {/* Rating badge */}
          {rating && (
            <div
              className={`absolute top-2 right-2 rounded-md px-2 py-0.5 text-xs font-bold shadow-lg ${getRatingColor(vote_average)}`}
              aria-hidden="true"
            >
              <Star className="mr-1 size-2.5 fill-current" aria-hidden="true" />
              {rating}
            </div>
          )}

          {/* Gradient overlay for text readability */}
          <div className="absolute inset-x-0 bottom-0 h-28 bg-linear-to-t from-black/90 via-black/50 to-transparent" />
        </div>

        {/* Movie info */}
        <div className="absolute inset-x-0 bottom-0 p-3">
          <h3 className="line-clamp-2 text-sm/tight font-semibold text-white drop-shadow-lg">
            {title}
          </h3>
          {year && (
            <p className="mt-0.5 text-xs text-white/80 drop-shadow-lg">
              {year}
            </p>
          )}
        </div>
      </Link>
    </article>
  );
}
