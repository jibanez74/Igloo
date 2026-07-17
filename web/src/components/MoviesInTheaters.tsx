import { useQuery } from "@tanstack/react-query";
import { inTheatersQueryOpts } from "@/lib/query-opts";
import { Film } from "lucide-react";
import HomeMediaSection from "@/components/HomeMediaSection";
import InTheatersCard from "@/components/InTheatersCard";
import { HOME_POSTER_GRID_CLASS } from "@/lib/constants";
import type { TheaterMovieType } from "@/types";

export default function MoviesInTheaters() {
  const { data, isPending } = useQuery(inTheatersQueryOpts());

  let movies: TheaterMovieType[] = [];
  if (data && !data.error) {
    movies = [...data.data.movies].sort((a, b) => {
      const dateA = new Date(a.release_date).getTime();
      const dateB = new Date(b.release_date).getTime();

      return dateB - dateA;
    });
  }

  const hasError = data && data.error;
  const errorMessage = hasError
    ? data.message || "Failed to load movies. Please try again later."
    : undefined;

  return (
    <HomeMediaSection
      title="Now Playing in Theaters"
      headingId="movies-in-theaters"
      items={movies}
      isPending={isPending}
      errorMessage={errorMessage}
      loadingLabel="Loading movies..."
      emptyTitle="No Movies Available"
      emptyDescription="Unable to fetch movies currently playing in theaters. Check back later."
      emptyIcon={Film}
      countNoun="movie"
      gridClassName={HOME_POSTER_GRID_CLASS}
      getKey={(movie: TheaterMovieType) => String(movie.id)}
      renderItem={(movie: TheaterMovieType) => <InTheatersCard movie={movie} />}
    />
  );
}
