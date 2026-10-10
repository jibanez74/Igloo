import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Film } from "lucide-react";
import PosterCard from "@/components/shared/PosterCard";
import { watchProgressPercent } from "@/lib/format";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { unwrapInt, unwrapString } from "@/lib/nullable";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { LatestMovieType } from "@/types";

type MovieCardProps = {
  movie: LatestMovieType;
  watchProgress?: { progressSec: number; durationSec: number };
  /** The card's own menu, where a list offers actions on its movies. */
  actions?: ReactNode;
};

export default function MovieCard({
  movie,
  watchProgress,
  actions,
}: MovieCardProps) {
  const { id, title, poster_path, year } = movie;
  const queryClient = useQueryClient();

  const handlePrefetch = () =>
    queryClient.prefetchQuery(libraryMovieDetailsQueryOpts(id));

  const releaseYear = unwrapInt(year);
  const ariaTitle = releaseYear == null ? title : `${title} ${releaseYear}`;

  // A saved position makes the play action a resume, the same wording the
  // episode cards beside it in Continue Watching use.
  const hasProgress =
    watchProgress !== undefined && watchProgress.durationSec > 0;
  const detailsLabel = hasProgress
    ? `${ariaTitle}, ${watchProgressPercent(watchProgress.progressSec, watchProgress.durationSec)}% watched`
    : ariaTitle;

  const posterUrl = buildTmdbImageUrl(
    unwrapString(poster_path),
    TMDB_POSTER_SIZE,
  );

  return (
    <PosterCard
      detailsLink={{ to: "/movies/$id", params: { id: String(id) } }}
      playLink={{ to: "/movies/$id/play", params: { id: String(id) } }}
      posterUrl={posterUrl}
      fallbackIcon={Film}
      title={title}
      subtitle={releaseYear == null ? undefined : String(releaseYear)}
      detailsLabel={detailsLabel}
      playLabel={`${hasProgress ? "Resume" : "Play"} ${ariaTitle}`}
      watchProgress={watchProgress}
      actions={actions}
      onPrefetch={handlePrefetch}
    />
  );
}
