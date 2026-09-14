import { Film, Star } from "lucide-react";
import PosterCard from "@/components/shared/PosterCard";
import { Badge } from "@/components/ui/badge";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { parseCatalogDate } from "@/lib/format";
import { criticRatingClass } from "@/lib/rating";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { TheaterMovieType } from "@/types";

type InTheatersCardProps = {
  movie: TheaterMovieType;
};

// An unreleased title is not in the library, so there is nothing to play and
// no detail query to prefetch: the card is the poster, the critic rating and
// a link to the TMDB-backed details page.
export default function InTheatersCard({ movie }: InTheatersCardProps) {
  const { id, title, poster_path, vote_average, release_date } = movie;

  const posterUrl = poster_path
    ? buildTmdbImageUrl(poster_path, TMDB_POSTER_SIZE)
    : "";

  const rating = vote_average ? vote_average.toFixed(1) : null;
  const year = release_date
    ? parseCatalogDate(release_date).getFullYear()
    : null;

  return (
    <PosterCard
      detailsLink={{
        to: "/movies/in-theaters/$id",
        params: { id: id.toString() },
      }}
      posterUrl={posterUrl}
      fallbackIcon={Film}
      title={title}
      subtitle={year ? String(year) : undefined}
      detailsLabel={`${title}${year ? `, ${year}` : ""}${rating ? `, rated ${rating} out of 10` : ""}`}
      badge={
        rating && (
          <Badge
            className={cn(
              "absolute top-2 right-2 rounded-md px-2 font-bold shadow-lg",
              criticRatingClass(vote_average),
            )}
            aria-hidden="true"
          >
            <Star className="size-2.5 fill-current" aria-hidden="true" />
            {rating}
          </Badge>
        )
      }
    />
  );
}
