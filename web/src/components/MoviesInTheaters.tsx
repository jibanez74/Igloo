import { useQuery } from "@tanstack/react-query";
import { inTheatersQueryOpts } from "@/lib/query-opts";
import { Film } from "lucide-react";
import HomeMediaSection from "@/components/HomeMediaSection";
import MovieCard from "@/components/InTheatersCard";
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

  const getAnnouncementMessage = () => {
    if (isPending) return undefined;
    if (hasError) return data.message || "Failed to load movies";
    if (movies.length === 0) return "No movies currently available";
    return undefined;
  };

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
      countLabel="movies"
      gridClassName="grid grid-cols-[repeat(auto-fit,minmax(min(8rem,100%),1fr))] gap-3 sm:gap-4 lg:grid-cols-[repeat(auto-fit,minmax(9rem,1fr))]"
      announcementMessage={getAnnouncementMessage()}
      getKey={(movie: TheaterMovieType) => String(movie.id)}
      renderItem={(movie: TheaterMovieType) => <MovieCard movie={movie} />}
    />
  );
}
