import { useQueryClient } from "@tanstack/react-query";
import { Film } from "lucide-react";
import PosterCard from "@/components/shared/PosterCard";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { LatestMovieType } from "@/types";

type MovieCardProps = {
  movie: LatestMovieType;
  watchProgress?: { progressSec: number; durationSec: number };
};

export default function MovieCard({ movie, watchProgress }: MovieCardProps) {
  const { id, title, poster_path, year } = movie;
  const queryClient = useQueryClient();

  const handlePrefetch = () =>
    queryClient.prefetchQuery(libraryMovieDetailsQueryOpts(id));

  const ariaTitle = year.Valid ? `${title} ${year.Int64}` : title;

  const progressPct =
    watchProgress && watchProgress.durationSec > 0
      ? Math.min(
          100,
          Math.max(
            0,
            Math.round(
              (watchProgress.progressSec / watchProgress.durationSec) * 100,
            ),
          ),
        )
      : null;
  const detailsLabel =
    progressPct !== null ? `${ariaTitle}, ${progressPct}% watched` : ariaTitle;

  const posterUrl =
    poster_path.Valid && poster_path.String !== ""
      ? buildTmdbImageUrl(poster_path.String, TMDB_POSTER_SIZE)
      : "";

  return (
    <PosterCard
      detailsLink={{ to: "/movies/$id", params: { id: String(id) } }}
      playLink={{ to: "/movies/$id/play", params: { id: String(id) } }}
      posterUrl={posterUrl}
      fallbackIcon={Film}
      title={title}
      subtitle={year.Valid ? String(year.Int64) : undefined}
      detailsLabel={detailsLabel}
      playLabel={`Play ${ariaTitle}`}
      watchProgress={watchProgress}
      onPrefetch={handlePrefetch}
    />
  );
}
