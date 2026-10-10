import { useQuery } from "@tanstack/react-query";
import { latestMoviesQueryOpts } from "@/lib/query-opts";
import { Film } from "lucide-react";
import HomeMediaSection from "@/components/home/HomeMediaSection";
import MovieCard from "@/components/movies/MovieCard";
import { HOME_POSTER_GRID_CLASS } from "@/lib/constants";
import { apiErrorMessage } from "@/lib/is-api-failure";

export default function LatestMovies() {
  const { data, isPending, refetch } = useQuery(latestMoviesQueryOpts());

  const movies = data && !data.error ? (data.data?.movies ?? []) : [];
  const errorMessage = data?.error
    ? apiErrorMessage(data, "Couldn’t load movies.")
    : undefined;

  return (
    <HomeMediaSection
      title="Recently Added Movies"
      headingId="recent-movies"
      items={movies}
      isPending={isPending}
      errorMessage={errorMessage}
      onRetry={() => void refetch()}
      loadingLabel="Loading movies..."
      emptyTitle="No Movies Yet"
      emptyDescription="Your movie library is empty. Add a movies folder in settings and run a scan to get started."
      emptyIcon={Film}
      countNoun="movie"
      gridClassName={HOME_POSTER_GRID_CLASS}
      getKey={movie => String(movie.id)}
      renderItem={movie => <MovieCard movie={movie} />}
    />
  );
}
