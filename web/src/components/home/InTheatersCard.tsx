import { Film } from "lucide-react";
import PosterCard from "@/components/shared/PosterCard";
import TmdbScoreBadge from "@/components/shared/TmdbScoreBadge";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { catalogYear } from "@/lib/format";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { TheaterMovieType } from "@/types";

type InTheatersCardProps = {
  movie: TheaterMovieType;
};

// An unreleased title is not in the library, so there is nothing to play and
// no detail query to prefetch: the card is the poster, its TMDB score and a
// link to the TMDB-backed details page.
export default function InTheatersCard({ movie }: InTheatersCardProps) {
  const { id, title, poster_path, vote_average, release_date } = movie;

  const posterUrl = buildTmdbImageUrl(poster_path, TMDB_POSTER_SIZE);

  const rating = vote_average ? vote_average.toFixed(1) : null;
  const year = catalogYear(release_date);

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
      detailsLabel={`${title}${year ? `, ${year}` : ""}${rating ? `, TMDB user score ${rating} out of 10` : ""}`}
      badge={
        rating && (
          // The link already speaks the score; the darker fill keeps the
          // badge legible over raw poster art, which has no backdrop dim.
          <TmdbScoreBadge
            score={vote_average}
            size="sm"
            aria-hidden="true"
            className="absolute top-2 right-2 bg-black/60 shadow-lg backdrop-blur-sm"
          />
        )
      }
    />
  );
}
