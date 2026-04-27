import { useQuery } from "@tanstack/react-query";
import { latestMoviesQueryOpts } from "@/lib/query-opts";
import { Film } from "lucide-react";
import HomeMediaSection from "@/components/HomeMediaSection";
import MovieCard from "@/components/MovieCard";

export default function LatestMovies() {
  const { data, isPending } = useQuery(latestMoviesQueryOpts());

  const movies = data && !data.error ? (data.data?.movies ?? []) : [];
  const hasError = data && data.error;
  const errorMessage = hasError
    ? data.message || "Failed to load movies. Please try again later."
    : undefined;

  const getAnnouncementMessage = () => {
    if (isPending) return undefined;
    if (hasError) return data.message || "Failed to load movies";
    if (movies.length === 0) return "No movies in your library";
    return undefined;
  };

  return (
    <HomeMediaSection
      title="Recently Added Movies"
      headingId="recent-movies"
      items={movies}
      isPending={isPending}
      errorMessage={errorMessage}
      loadingLabel="Loading movies..."
      emptyTitle="No Movies Yet"
      emptyDescription="Your movie library is empty. Add a movies folder in settings and run a scan to get started."
      emptyIcon={Film}
      countLabel="movies"
      gridClassName="grid grid-cols-[repeat(auto-fit,minmax(min(7.5rem,100%),1fr))] gap-3 sm:gap-4"
      announcementMessage={getAnnouncementMessage()}
      getKey={movie => String(movie.id)}
      renderItem={movie => <MovieCard movie={movie} />}
    />
  );
}
