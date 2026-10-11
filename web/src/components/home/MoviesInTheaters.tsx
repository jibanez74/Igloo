import { useQuery } from "@tanstack/react-query";
import { inTheatersQueryOpts } from "@/lib/query-opts";
import { Film } from "lucide-react";
import HomeMediaSection from "@/components/home/HomeMediaSection";
import InTheatersCard from "@/components/home/InTheatersCard";
import { HOME_POSTER_GRID_CLASS } from "@/lib/constants";
import { parseCatalogDate } from "@/lib/format";
import type { TheaterMovieType } from "@/types";
import { apiErrorMessage } from "@/lib/is-api-failure";

export default function MoviesInTheaters() {
  const { data, isPending, refetch } = useQuery(inTheatersQueryOpts());

  let movies: TheaterMovieType[] = [];
  if (data && !data.error) {
    movies = data.data.movies.toSorted(
      (a, b) =>
        parseCatalogDate(b.release_date).getTime() -
        parseCatalogDate(a.release_date).getTime(),
    );
  }

  const errorMessage = data?.error
    ? apiErrorMessage(data, "Couldn’t load movies in theaters.")
    : undefined;

  return (
    <HomeMediaSection
      title="Now Playing in Theaters"
      headingId="movies-in-theaters"
      items={movies}
      isPending={isPending}
      errorMessage={errorMessage}
      onRetry={() => void refetch()}
      loadingLabel="Loading movies..."
      emptyTitle="No Movies Available"
      emptyDescription="No movies are listed as playing in theaters right now. Check back later."
      emptyIcon={Film}
      countNoun="movie"
      gridClassName={HOME_POSTER_GRID_CLASS}
      getKey={(movie: TheaterMovieType) => String(movie.id)}
      renderItem={(movie: TheaterMovieType) => <InTheatersCard movie={movie} />}
    />
  );
}
