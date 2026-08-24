import { useQuery } from "@tanstack/react-query";
import { continueWatchingQueryOpts } from "@/lib/query-opts";
import { Play } from "lucide-react";
import HomeMediaSection from "@/components/home/HomeMediaSection";
import MovieCard from "@/components/movies/MovieCard";
import { HOME_POSTER_GRID_CLASS } from "@/lib/constants";

export default function ContinueWatching() {
  const { data, isPending } = useQuery(continueWatchingQueryOpts());

  const movies = data && !data.error ? (data.data?.movies ?? []) : [];
  const hasError = data && data.error;
  const errorMessage = hasError
    ? data.message || "Failed to load your in-progress movies."
    : undefined;

  // The home route loader awaits this query, so the section renders populated
  // or not at all — never a loading block that collapses (layout shift).
  if (isPending) return null;
  if (!errorMessage && movies.length === 0) return null;

  return (
    <HomeMediaSection
      title="Continue Watching"
      headingId="continue-watching"
      items={movies}
      errorMessage={errorMessage}
      emptyTitle="Nothing In Progress"
      emptyDescription="Movies you start watching will appear here."
      emptyIcon={Play}
      countNoun="movie"
      gridClassName={HOME_POSTER_GRID_CLASS}
      getKey={movie => String(movie.id)}
      renderItem={movie => (
        <MovieCard
          movie={movie}
          watchProgress={{
            progressSec: movie.progress_sec,
            durationSec: movie.duration_sec,
          }}
        />
      )}
    />
  );
}
