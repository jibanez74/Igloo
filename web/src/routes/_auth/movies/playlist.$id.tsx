import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ListVideo } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import MovieCard from "@/components/movies/MovieCard";
import LibraryPagination from "@/components/shared/LibraryPagination";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import {
  moviePlaylistDetailsQueryOpts,
  moviePlaylistMoviesQueryOpts,
} from "@/lib/query-opts";
import {
  MOTION_MICRO_COLORS_CLASS,
  MOVIES_PER_PAGE,
  MOVIES_PLAYLISTS_TAB_SEARCH,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import { scrollWindowToTop } from "@/lib/motion";
import { unwrapString } from "@/lib/nullable";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { isApiFailure } from "@/lib/is-api-failure";

export const Route = createFileRoute("/_auth/movies/playlist/$id")({
  loader: async ({ context, params }) => {
    const id = parseInt(params.id, 10);
    if (Number.isNaN(id)) return;
    await Promise.all([
      context.queryClient.ensureQueryData(moviePlaylistDetailsQueryOpts(id)),
      context.queryClient.ensureQueryData(
        moviePlaylistMoviesQueryOpts(id, 1, MOVIES_PER_PAGE, "asc"),
      ),
    ]);
  },
  component: MoviePlaylistPage,
});

function MoviePlaylistPage() {
  const { id } = Route.useParams();
  const playlistId = parseInt(id, 10);
  const [page, setPage] = useState(1);
  const [sort, setSort] = useState<"asc" | "desc">("asc");

  const {
    data,
    isLoading,
    error,
    refetch: refetchDetails,
  } = useQuery(moviePlaylistDetailsQueryOpts(playlistId));

  const {
    data: moviesRes,
    isLoading: moviesLoading,
    isError: moviesQueryError,
    refetch: refetchMovies,
  } = useQuery(
    moviePlaylistMoviesQueryOpts(playlistId, page, MOVIES_PER_PAGE, sort),
  );

  if (isLoading) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <Spinner className="size-10 text-primary" />
      </div>
    );
  }

  if (error || !data || data.error) {
    const msg = isApiFailure(data)
      ? data.message
      : "Failed to load playlist. Check your connection and try again.";
    return (
      <div className="py-12">
        <MoviesLoadError
          message={msg}
          onRetry={() => void refetchDetails()}
        />
        <Link
          to="/movies"
          search={MOVIES_PLAYLISTS_TAB_SEARCH}
          className="mt-4 inline-block text-primary hover:underline"
        >
          Back to movie playlists
        </Link>
      </div>
    );
  }

  const { playlist, movie_count } = data.data;
  const desc = unwrapString(playlist.description);

  const movies =
    moviesRes?.error === false ? moviesRes.data.movies : [];
  const totalPages =
    moviesRes?.error === false ? moviesRes.data.total_pages : 0;

  const announce =
    !moviesLoading && movies.length > 0
      ? `Page ${page} of ${Math.max(totalPages, 1)}, ${movies.length} movies on this page`
      : undefined;

  return (
    <div className="min-w-0">
      <title>{playlist.name} - Igloo</title>
      <meta
        name="description"
        content={`Movie playlist: ${playlist.name}`}
      />

      <Link
        to="/movies"
        search={MOVIES_PLAYLISTS_TAB_SEARCH}
        className={cn(
          MOTION_MICRO_COLORS_CLASS,
          "mb-6 inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-primary",
        )}
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        Movie playlists
      </Link>

      <header className="mb-8">
        <div className="flex items-start gap-3">
          <ListVideo className="mt-1 size-8 shrink-0 text-primary" aria-hidden="true" />
          <div className="min-w-0">
            <h1 className="text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
              {playlist.name}
            </h1>
            {desc ? (
              <p className="mt-2 text-muted-foreground">{desc}</p>
            ) : null}
            <p className="mt-2 text-sm text-muted-foreground">
              {movie_count} {movie_count === 1 ? "movie" : "movies"}
            </p>
          </div>
        </div>
      </header>

      <LiveAnnouncer message={announce} />

      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span className="text-sm text-muted-foreground">Playlist movies</span>
        <div className="flex flex-wrap items-center gap-2 sm:gap-3">
          <span className="text-sm text-muted-foreground">
            Page {page} of {Math.max(totalPages, 1)}
          </span>
          <button
            type="button"
            onClick={() => {
              setSort((s) => (s === "asc" ? "desc" : "asc"));
              setPage(1);
            }}
            className="rounded-full bg-muted px-3 py-1.5 text-sm text-muted-foreground hover:bg-accent"
          >
            Sort: {sort === "asc" ? "A–Z" : "Z–A"}
          </button>
        </div>
      </div>

      {moviesQueryError || isApiFailure(moviesRes) ? (
        <MoviesLoadError
          message={
            isApiFailure(moviesRes)
              ? moviesRes.message
              : "Couldn’t load movies in this playlist. Check your connection and try again."
          }
          onRetry={() => void refetchMovies()}
        />
      ) : moviesLoading ? (
        <div className="flex justify-center py-12">
          <Spinner className="size-8 text-primary" />
        </div>
      ) : movies.length === 0 ? (
        <p className="py-12 text-center text-muted-foreground">
          No movies in this playlist yet.
        </p>
      ) : (
        <>
          <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            {movies.map((m) => (
              <MovieCard key={m.id} movie={m} />
            ))}
          </div>
          {totalPages > 1 && (
            <LibraryPagination
              currentPage={page}
              totalPages={totalPages}
              onPageChange={(p) => {
                setPage(p);
                scrollWindowToTop();
              }}
            />
          )}
        </>
      )}
    </div>
  );
}
