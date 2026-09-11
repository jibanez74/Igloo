import { Link } from "@tanstack/react-router";
import { Tv } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  CARD_FOCUS_WITHIN_RING_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_SURFACE_CLASS,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { LatestShowType } from "@/types";

type ShowCardProps = {
  show: LatestShowType;
};

// Shows have no play route and no detail query yet, so this card carries no play
// action and no prefetch - like InTheatersCard and MusicianCard.
export default function ShowCard({ show }: ShowCardProps) {
  const { id, name, poster_path, premiere_year } = show;

  const posterUrl =
    poster_path.Valid && poster_path.String !== ""
      ? buildTmdbImageUrl(poster_path.String, TMDB_POSTER_SIZE)
      : "";
  const { showPoster, onError } = usePosterFallback(posterUrl);

  const cardAriaLabel = premiere_year.Valid
    ? `${name} ${premiere_year.Int64}`
    : name;

  return (
    <article
      className={cn(CARD_SURFACE_CLASS, "min-w-0", CARD_FOCUS_WITHIN_RING_CLASS)}
    >
      <Link
        to="/tv-shows/$id"
        params={{ id: String(id) }}
        className="block rounded-xl outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
        aria-label={cardAriaLabel}
      >
        {/* Poster with 2:3 aspect ratio (standard show poster) */}
        <div className="relative aspect-2/3 bg-muted">
          {showPoster ? (
            <img
              src={posterUrl}
              alt=""
              width={500}
              height={750}
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
              onError={onError}
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <Tv className="size-10 text-muted-foreground" aria-hidden="true" />
            </div>
          )}

          {/* Gradient overlay for text readability */}
          <div className="absolute inset-x-0 bottom-0 h-28 bg-linear-to-t from-black/90 via-black/50 to-transparent" />
        </div>

        {/* Show info */}
        <div className="absolute inset-x-0 bottom-0 p-3">
          <h3 className="line-clamp-2 text-sm/tight font-semibold text-white drop-shadow-lg">
            {name}
          </h3>
          {premiere_year.Valid && (
            <p className="mt-0.5 text-xs text-white/80 drop-shadow-lg">
              {premiere_year.Int64}
            </p>
          )}
        </div>
      </Link>
    </article>
  );
}
