import {
  useEffect,
  useRef,
  useState,
  type MouseEvent,
  type MutableRefObject,
  type RefObject,
} from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDownAZ,
  ArrowUpAZ,
  Film,
  Grid3X3,
  Heart,
  ListVideo,
  MoreHorizontal,
  Plus,
  X,
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import CreateMoviePlaylistDialog from "@/components/CreateMoviePlaylistDialog";
import MovieCard from "@/components/MovieCard";
import MoviePlaylistCard from "@/components/MoviePlaylistCard";
import LibraryPagination from "@/components/LibraryPagination";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import {
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  MOVIES_PER_PAGE,
} from "@/lib/constants";
import {
  likedMoviesQueryOpts,
  moviePlaylistsQueryOpts,
  moviesByGenreQueryOpts,
  moviesGenresQueryOpts,
  moviesLibraryQueryOpts,
  moviesStatsQueryOpts,
  tmdbStatusQueryOpts,
} from "@/lib/query-opts";
import { MoviesLoadError } from "@/components/MoviesLoadError";
import { isApiFailure } from "@/lib/is-api-failure";
import { cn } from "@/lib/utils";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import RequestMovieDialog from "@/components/RequestMovieDialog";
import {
  moviesSearchSchema,
  type MoviesSearchParams,
} from "@/types/route-search";

export const Route = createFileRoute("/_auth/movies/")({
  validateSearch: moviesSearchSchema,
  loaderDeps: ({
    search: { allPage, sort, tab, genreId, genresPage, view, playlistsPage },
  }) => ({
    allPage,
    sort,
    tab,
    genreId,
    genresPage,
    view,
    playlistsPage,
  }),
  loader: async ({
    context,
    deps: { allPage, sort, tab, genreId, genresPage, view, playlistsPage },
  }) => {
    const { queryClient } = context;
    const promises: Promise<unknown>[] = [
      queryClient.ensureQueryData(moviesStatsQueryOpts()),
      queryClient.ensureQueryData(
        moviesLibraryQueryOpts(allPage, MOVIES_PER_PAGE, sort),
      ),
    ];
    if (tab === "genres") {
      promises.push(queryClient.ensureQueryData(moviesGenresQueryOpts()));
      if (genreId != null && genreId > 0) {
        promises.push(
          queryClient.ensureQueryData(
            moviesByGenreQueryOpts(genreId, genresPage, MOVIES_PER_PAGE, sort),
          ),
        );
      }
    }
    if (tab === "playlists") {
      promises.push(queryClient.ensureQueryData(moviePlaylistsQueryOpts()));
      if (view === "liked") {
        promises.push(
          queryClient.ensureQueryData(
            likedMoviesQueryOpts(playlistsPage, MOVIES_PER_PAGE, sort),
          ),
        );
      }
    }
    await Promise.all(promises);
  },
  component: MoviesPage,
});

type PlaylistsFocusIntent =
  | "enter-liked-from-toolbar"
  | "return-to-playlists";

// ---------------------------------------------------------------------------
// Page component
// ---------------------------------------------------------------------------

function MoviesPage() {
  const navigate = Route.useNavigate();
  const { tab, allPage, sort, genreId, genresPage, view, playlistsPage } =
    Route.useSearch();
  const genresTabTriggerRef = useRef<HTMLButtonElement | null>(null);
  const playlistsTabTriggerRef = useRef<HTMLButtonElement | null>(null);
  const playlistsFocusIntentRef = useRef<PlaylistsFocusIntent | null>(null);
  const { isExiting, runTransition, usesContentAnimation } =
    useContentFadeTransition(CONTENT_FADE_TRANSITION_MS);

  const topLevelTabPanelClassName = cn(
    usesContentAnimation &&
      (isExiting ? CONTENT_FADE_EXIT_CLASS : CONTENT_FADE_ENTER_CLASS),
  );
  const primeEnterLikedFocus = () => {
    playlistsFocusIntentRef.current = "enter-liked-from-toolbar";
  };
  let topLevelTabContent = (
    <AllMoviesTabContent currentPage={allPage} sort={sort} />
  );

  if (tab === "genres") {
    topLevelTabContent = (
      <GenresTabContent
        genreId={genreId}
        genresPage={genresPage}
        sort={sort}
        fallbackFocusRef={genresTabTriggerRef}
      />
    );
  }

  if (tab === "playlists") {
    topLevelTabContent = (
      <PlaylistsTabContent
        view={view}
        playlistsPage={playlistsPage}
        sort={sort}
        focusIntentRef={playlistsFocusIntentRef}
        primeEnterLikedFocus={primeEnterLikedFocus}
        playlistsTabTriggerRef={playlistsTabTriggerRef}
      />
    );
  }

  const navigateWithTabTransition = (
    nextTab: MoviesSearchParams["tab"],
    getNextSearch: (prev: MoviesSearchParams) => MoviesSearchParams,
  ) => {
    const shouldAnimate = nextTab !== tab;
    const navigateToNextTab = () =>
      navigate({
        to: "/movies",
        search: prev => getNextSearch(prev),
        replace: true,
      });

    runTransition({
      shouldAnimate,
      onTransition: navigateToNextTab,
    });
  };

  const handleTabChange = (newTab: string) => {
    const nextTab = newTab as MoviesSearchParams["tab"];

    navigateWithTabTransition(nextTab, prev => ({
      ...prev,
      tab: nextTab,
      ...(nextTab !== "playlists" ? { view: undefined } : {}),
    }));
  };

  const handleOpenLikedMovies = () => {
    primeEnterLikedFocus();
    navigateWithTabTransition("playlists", prev => ({
      ...prev,
      tab: "playlists",
      view: "liked",
      playlistsPage: 1,
    }));
  };

  const handleOpenMoviePlaylists = () => {
    navigateWithTabTransition("playlists", prev => ({
      ...prev,
      tab: "playlists",
      view: undefined,
      playlistsPage: 1,
    }));
  };

  return (
    <div className="min-w-0">
      <title>Movies - Igloo</title>
      <meta
        name="description"
        content="Browse and organize your personal movie collection in your Igloo media library."
      />

      {/* Page header */}
      <header className="mb-6 sm:mb-7">
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-white md:text-4xl">
          <Film className="size-6 shrink-0 text-amber-400" aria-hidden="true" />
          <span>Movie Library</span>
        </h1>
        <p className="mt-1.5 max-w-2xl text-sm text-slate-400 md:text-base">
          Browse, organize, and enjoy your film collection
        </p>
      </header>

      {/* Stats + More dropdown */}
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <MoviesStats />
        <MoreMenu
          onOpenLikedMovies={handleOpenLikedMovies}
          onOpenMoviePlaylists={handleOpenMoviePlaylists}
        />
      </div>

      {/* Tabs — controlled by URL search param */}
      <Tabs value={tab} onValueChange={handleTabChange}>
        <TabsList className="grid! h-auto w-full max-w-full grid-cols-3 gap-1 border border-slate-700/50 bg-slate-800/50 p-1 sm:w-fit sm:max-w-none sm:grid-cols-3">
          <TabsTrigger
            value="all"
            className="min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
          >
            <Grid3X3
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            All Movies
          </TabsTrigger>
          <TabsTrigger
            value="genres"
            ref={genresTabTriggerRef}
            className="min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
          >
            <Film
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Genres
          </TabsTrigger>
          <TabsTrigger
            value="playlists"
            ref={playlistsTabTriggerRef}
            className="min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
          >
            <ListVideo
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Playlists
          </TabsTrigger>
        </TabsList>

        <TabsContent value={tab} className="mt-5 sm:mt-6">
          <div key={tab} className={topLevelTabPanelClassName}>
            {topLevelTabContent}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Stats row
// ---------------------------------------------------------------------------

function MoviesStats() {
  const { data, isError, isLoading, refetch } = useQuery(
    moviesStatsQueryOpts(),
  );

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t load library statistics. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  const total = data?.error === false ? data.data.total_movies : 0;
  const label = isLoading
    ? "Library statistics: loading"
    : `Library statistics: ${total} movies`;

  return (
    <section
      className="flex flex-wrap gap-6"
      aria-label={label}
    >
      <div className="flex items-center gap-2" aria-hidden="true">
        <Film className="size-4 text-amber-400" />
        <span className="font-medium text-white">
          {isLoading ? "—" : total}
        </span>
        <span className="text-slate-400">Movies</span>
      </div>
    </section>
  );
}

// ---------------------------------------------------------------------------
// More dropdown (placeholders only)
// ---------------------------------------------------------------------------

type MoreMenuProps = {
  onOpenLikedMovies: () => void;
  onOpenMoviePlaylists: () => void;
};

function MoreMenu({
  onOpenLikedMovies,
  onOpenMoviePlaylists,
}: MoreMenuProps) {
  const moreOptionsButtonRef = useRef<HTMLButtonElement | null>(null);
  const [requestMovieOpen, setRequestMovieOpen] = useState(false);
  const { data: tmdbStatusData, isLoading: tmdbStatusLoading } = useQuery(
    tmdbStatusQueryOpts(),
  );
  const tmdbAvailable =
    tmdbStatusData?.error === false ? tmdbStatusData.data.available : false;
  const requestMovieDisabled = tmdbStatusLoading || !tmdbAvailable;
  const requestMovieDescription = tmdbStatusLoading
    ? "TMDB search status is still loading."
    : "TMDB search is unavailable on this server.";

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          ref={moreOptionsButtonRef}
          className="inline-flex items-center justify-center rounded-full p-2 text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
          aria-label="More options"
        >
          <MoreHorizontal className="size-5" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          className="border-slate-700 bg-slate-800"
        >
          <DropdownMenuItem
            className="cursor-pointer text-slate-200 focus:bg-slate-700 focus:text-white"
            onClick={onOpenLikedMovies}
          >
            <Heart className="mr-2 size-4" aria-hidden="true" />
            Liked movies
          </DropdownMenuItem>
          <DropdownMenuItem
            className="cursor-pointer text-slate-200 focus:bg-slate-700 focus:text-white"
            onClick={onOpenMoviePlaylists}
          >
            <ListVideo className="mr-2 size-4" aria-hidden="true" />
            Movie playlists
          </DropdownMenuItem>
          <DropdownMenuItem
            className="cursor-pointer text-slate-200 focus:bg-slate-700 focus:text-white"
            disabled={requestMovieDisabled}
            aria-label={
              requestMovieDisabled
                ? `Request Movie unavailable. ${requestMovieDescription}`
                : "Request Movie"
            }
            title={requestMovieDisabled ? requestMovieDescription : undefined}
            onSelect={(event) => {
              if (requestMovieDisabled) {
                event.preventDefault();
                return;
              }
              setRequestMovieOpen(true);
            }}
          >
            <Plus className="mr-2 size-4" aria-hidden="true" />
            Request Movie
            {requestMovieDisabled && (
              <span className="sr-only"> {requestMovieDescription}</span>
            )}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      {requestMovieOpen && (
        <RequestMovieDialog
          open={requestMovieOpen}
          onOpenChange={setRequestMovieOpen}
          restoreFocusRef={moreOptionsButtonRef}
        />
      )}
    </>
  );
}

// ---------------------------------------------------------------------------
// All Movies tab
// ---------------------------------------------------------------------------

type AllMoviesTabContentProps = {
  currentPage: number;
  sort: "asc" | "desc";
};

function AllMoviesTabContent({ currentPage, sort }: AllMoviesTabContentProps) {
  const navigate = Route.useNavigate();

  const { data, isLoading, isError, refetch } = useQuery(
    moviesLibraryQueryOpts(currentPage, MOVIES_PER_PAGE, sort),
  );

  const movies = data?.error === false ? data.data.movies : [];
  const totalPages = data?.error === false ? data.data.total_pages : 0;
  const hasMultiplePages = totalPages > 1;

  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (movies.length === 0) return "No movies found";
    return `Showing ${movies.length} movies, page ${currentPage} of ${totalPages}`;
  };

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        allPage: newPage,
      }),
      replace: true,
    });
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const handleSortToggle = () =>
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        sort: prev.sort === "asc" ? "desc" : "asc",
        allPage: 1,
      }),
      replace: true,
    });

  if (isLoading) {
    return <AllMoviesTabSkeleton />;
  }

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t load movies. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  if (movies.length === 0) {
    return (
      <div className="py-12 text-center text-slate-400">
        <Film className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
        <p>No movies found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      <LiveAnnouncer message={getAnnouncement()} />

      {/* Header with count, page info, and sort toggle */}
      <div
        className={
          hasMultiplePages
            ? "mb-5 flex items-center justify-between gap-2"
            : "mb-5 flex justify-end"
        }
      >
        {hasMultiplePages && (
          <span className="text-sm text-slate-400">
            Page {currentPage} of {totalPages}
          </span>
        )}
        <div className="flex items-center gap-2 sm:gap-3">
          <button
            onClick={handleSortToggle}
            className="inline-flex items-center gap-1.5 rounded-full bg-slate-800 px-3 py-1.5 text-sm font-medium text-slate-300 transition-colors hover:bg-slate-700 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
            aria-label={
              sort === "asc"
                ? "Sorted A to Z, click to sort Z to A"
                : "Sorted Z to A, click to sort A to Z"
            }
          >
            {sort === "asc" ? (
              <>
                <ArrowDownAZ className="size-4" aria-hidden="true" />
                A–Z
              </>
            ) : (
              <>
                <ArrowUpAZ className="size-4" aria-hidden="true" />
                Z–A
              </>
            )}
          </button>
        </div>
      </div>

      {/* Movie grid */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
        {movies.map(movie => (
          <MovieCard key={movie.id} movie={movie} />
        ))}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <LibraryPagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={handlePageChange}
        />
      )}
    </div>
  );
}

function AllMoviesTabSkeleton() {
  return (
    <div>
      <div className="mb-5 flex justify-end">
        <div className="h-8 w-16 animate-pulse rounded-full bg-slate-800" />
      </div>
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
        {Array.from({ length: MOVIES_PER_PAGE }).map((_, i) => (
          <div
            key={i}
            className="animate-pulse overflow-hidden rounded-xl border border-slate-800 bg-slate-900"
          >
            <div className="aspect-2/3 bg-slate-800" />
            <div className="p-3">
              <div className="h-4 w-3/4 rounded-sm bg-slate-800" />
              <div className="mt-2 h-3 w-1/2 rounded-sm bg-slate-800" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Genres tab
// ---------------------------------------------------------------------------

type GenresTabContentProps = {
  genreId: number | undefined;
  genresPage: number;
  sort: "asc" | "desc";
  fallbackFocusRef: RefObject<HTMLButtonElement | null>;
};

function GenresTabContent({
  genreId,
  genresPage,
  sort,
  fallbackFocusRef,
}: GenresTabContentProps) {
  const navigate = Route.useNavigate();
  const genreButtonRefs = useRef(new Map<number, HTMLButtonElement>());
  const pendingRestoreGenreIdRef = useRef<number | null>(null);

  const genresQuery = useQuery(moviesGenresQueryOpts());
  const genresRes = genresQuery.data;
  const genresLoading = genresQuery.isLoading;

  const genres = genresRes?.error === false ? genresRes.data.genres : [];

  const moviesQuery = useQuery({
    ...moviesByGenreQueryOpts(genreId ?? 0, genresPage, MOVIES_PER_PAGE, sort),
  });
  const moviesRes = moviesQuery.data;
  const moviesLoading = moviesQuery.isLoading;

  const movies = moviesRes?.error === false ? moviesRes.data.movies : [];
  const totalPages =
    moviesRes?.error === false ? moviesRes.data.total_pages : 0;
  const total = moviesRes?.error === false ? moviesRes.data.total : 0;
  const hasMultiplePages = totalPages > 1;
  const hasSelectedGenre = genreId != null;

  const selectedGenreTag =
    genreId != null
      ? genres.find(g => g.genre_id === genreId)?.genre_tag
      : undefined;

  useEffect(() => {
    const restoreGenreId = pendingRestoreGenreIdRef.current;
    if (genreId != null || restoreGenreId == null) return;

    pendingRestoreGenreIdRef.current = null;
    focusDialogRestoreTarget(
      genreButtonRefs.current.get(restoreGenreId),
      fallbackFocusRef.current,
    );
  }, [fallbackFocusRef, genreId]);

  const getAnnouncement = () => {
    if (!hasSelectedGenre) return undefined;
    if (moviesLoading) return undefined;
    if (movies.length === 0) return "No movies in this genre";
    return `Showing ${movies.length} movies, page ${genresPage} of ${totalPages}`;
  };

  const handleSelectGenre = (id: number) => {
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        genreId: id,
        genresPage: 1,
      }),
      replace: true,
    });
  };

  const handleClearGenre = () => {
    pendingRestoreGenreIdRef.current = genreId ?? null;
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        genreId: undefined,
        genresPage: 1,
      }),
      replace: true,
    });
  };

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        genresPage: newPage,
      }),
      replace: true,
    });
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const handleSortToggle = () =>
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        sort: prev.sort === "asc" ? "desc" : "asc",
        genresPage: 1,
      }),
      replace: true,
    });

  if (genresLoading) {
    return <GenresTabSkeleton />;
  }

  if (genresQuery.isError || isApiFailure(genresRes)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(genresRes)
            ? genresRes.message
            : "Couldn’t load genres. Check your connection and try again."
        }
        onRetry={() => void genresQuery.refetch()}
      />
    );
  }

  if (genres.length === 0) {
    return (
      <div className="py-12 text-center text-slate-400">
        <Film className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
        <p>No genres with movies in your library yet.</p>
      </div>
    );
  }

  return (
    <div>
      <ul
        className={
          hasSelectedGenre
            ? "mb-5 grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7"
            : "mb-5 grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
        }
        aria-label="Movie genres"
      >
        {genres.map(g => {
          const selected = genreId === g.genre_id;
          return (
            <li key={g.genre_id} className="min-w-0">
              <button
                type="button"
                ref={node => {
                  if (node) {
                    genreButtonRefs.current.set(g.genre_id, node);
                    return;
                  }
                  genreButtonRefs.current.delete(g.genre_id);
                }}
                onClick={() => handleSelectGenre(g.genre_id)}
                className={`flex w-full min-w-0 flex-col justify-between rounded-lg border text-left transition-colors focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none ${
                  hasSelectedGenre ? "min-h-14 p-2" : "min-h-20 p-3"
                } ${
                  selected
                    ? "border-amber-500 bg-amber-500 text-slate-900 shadow-lg shadow-amber-500/15"
                    : "border-slate-700 bg-slate-800/70 text-slate-200 hover:border-amber-500/40 hover:bg-slate-800"
                }`}
                aria-pressed={selected}
              >
                <span className="line-clamp-2 text-sm font-semibold">
                  {g.genre_tag}
                </span>
                <span
                  className={`${hasSelectedGenre ? "mt-1" : "mt-3"} text-xs ${
                    selected ? "text-slate-900/70" : "text-slate-400"
                  }`}
                >
                  {g.movie_count} {g.movie_count === 1 ? "movie" : "movies"}
                </span>
              </button>
            </li>
          );
        })}
      </ul>

      {hasSelectedGenre && (
        <>
          <LiveAnnouncer message={getAnnouncement()} />

          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="text-sm font-medium text-white">
                {selectedGenreTag ?? "Genre"}
              </span>
              <span className="text-sm text-slate-400">
                {total.toLocaleString()} movies
              </span>
              <button
                type="button"
                onClick={handleClearGenre}
                className="inline-flex shrink-0 items-center gap-1 rounded-full border border-slate-600 px-3 py-1 text-xs font-medium text-slate-300 transition-colors hover:border-slate-500 hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none"
                aria-label="Clear genre filter"
              >
                <X className="size-3.5" aria-hidden="true" />
                Clear
              </button>
            </div>
            <div className="flex flex-wrap items-center gap-2 sm:gap-3">
              {hasMultiplePages && (
                <span className="text-sm text-slate-400">
                  Page {genresPage} of {totalPages}
                </span>
              )}
              <button
                type="button"
                onClick={handleSortToggle}
                className="inline-flex items-center gap-1.5 rounded-full bg-slate-800 px-3 py-1.5 text-sm font-medium text-slate-300 transition-colors hover:bg-slate-700 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                aria-label={
                  sort === "asc"
                    ? "Sorted A to Z, click to sort Z to A"
                    : "Sorted Z to A, click to sort A to Z"
                }
              >
                {sort === "asc" ? (
                  <>
                    <ArrowDownAZ className="size-4" aria-hidden="true" />
                    A–Z
                  </>
                ) : (
                  <>
                    <ArrowUpAZ className="size-4" aria-hidden="true" />
                    Z–A
                  </>
                )}
              </button>
            </div>
          </div>

          {moviesQuery.isError || isApiFailure(moviesRes) ? (
            <MoviesLoadError
              message={
                isApiFailure(moviesRes)
                  ? moviesRes.message
                  : "Couldn’t load movies for this genre. Check your connection and try again."
              }
              onRetry={() => void moviesQuery.refetch()}
            />
          ) : moviesLoading ? (
            <AllMoviesTabSkeleton />
          ) : movies.length === 0 ? (
            <div className="py-12 text-center text-slate-400">
              <Film
                className="mx-auto mb-4 size-10 opacity-50"
                aria-hidden="true"
              />
              <p>No movies found for this genre.</p>
            </div>
          ) : (
            <>
              <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
                {movies.map(movie => (
                  <MovieCard key={movie.id} movie={movie} />
                ))}
              </div>
              {totalPages > 1 && (
                <LibraryPagination
                  currentPage={genresPage}
                  totalPages={totalPages}
                  onPageChange={handlePageChange}
                />
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}

function GenresTabSkeleton() {
  return (
    <div>
      <div className="mb-5 grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {Array.from({ length: 10 }).map((_, i) => (
          <div
            key={i}
            className="min-h-20 animate-pulse rounded-lg border border-slate-800 bg-slate-900"
          />
        ))}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Playlists tab + Liked (view=liked)
// ---------------------------------------------------------------------------

type PlaylistsTabContentProps = {
  view: "liked" | undefined;
  playlistsPage: number;
  sort: "asc" | "desc";
  focusIntentRef: MutableRefObject<PlaylistsFocusIntent | null>;
  primeEnterLikedFocus: () => void;
  playlistsTabTriggerRef: RefObject<HTMLButtonElement | null>;
};

function PlaylistsTabContent({
  view,
  playlistsPage,
  sort,
  focusIntentRef,
  primeEnterLikedFocus,
  playlistsTabTriggerRef,
}: PlaylistsTabContentProps) {
  const navigate = Route.useNavigate();
  const [showCreate, setShowCreate] = useState(false);
  const createPlaylistRestoreRef = useRef<HTMLButtonElement | null>(null);
  const likedMoviesButtonRef = useRef<HTMLButtonElement | null>(null);

  const { data, isLoading, isError, refetch } = useQuery({
    ...moviePlaylistsQueryOpts(),
    enabled: view !== "liked",
  });
  const playlists = data?.error === false ? data.data.playlists : [];

  useEffect(() => {
    if (view === "liked" || isLoading) return;
    if (focusIntentRef.current !== "return-to-playlists") return;

    focusIntentRef.current = null;
    focusDialogRestoreTarget(
      likedMoviesButtonRef.current,
      playlistsTabTriggerRef.current,
    );
  }, [focusIntentRef, isLoading, playlistsTabTriggerRef, view]);

  const handleCreateOpen = (event: MouseEvent<HTMLButtonElement>) => {
    createPlaylistRestoreRef.current = event.currentTarget;
    setShowCreate(true);
  };

  if (view === "liked") {
    return (
      <LikedMoviesInPlaylistsTab
        playlistsPage={playlistsPage}
        sort={sort}
        focusIntentRef={focusIntentRef}
        onExitLiked={() => {
          focusIntentRef.current = "return-to-playlists";
          navigate({
            to: "/movies",
            search: (prev: MoviesSearchParams) => ({
              ...prev,
              view: undefined,
              playlistsPage: 1,
            }),
            replace: true,
          });
        }}
      />
    );
  }

  if (isLoading) {
    return <PlaylistsTabSkeleton />;
  }

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t load playlists. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <div>
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span className="text-sm text-slate-400">
          {playlists.length} {playlists.length === 1 ? "playlist" : "playlists"}
        </span>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            ref={likedMoviesButtonRef}
            onClick={() => {
              primeEnterLikedFocus();
              navigate({
                to: "/movies",
                search: (prev: MoviesSearchParams) => ({
                  ...prev,
                  view: "liked",
                  playlistsPage: 1,
                }),
                replace: true,
              });
            }}
            className="inline-flex min-h-10 items-center gap-2 rounded-full border border-slate-600 px-3 py-2 text-sm font-medium text-slate-300 transition-colors hover:border-amber-500/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none sm:px-4"
          >
            <Heart className="size-4 shrink-0" aria-hidden="true" />
            Liked movies
          </button>
          <button
            type="button"
            onClick={handleCreateOpen}
            className="inline-flex min-h-10 items-center gap-2 rounded-full bg-amber-500 px-3 py-2 text-sm font-medium text-slate-900 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:outline-none sm:px-4"
          >
            <Plus className="size-4 shrink-0" aria-hidden="true" />
            New playlist
          </button>
        </div>
      </div>

      {playlists.length === 0 ? (
        <EmptyMoviePlaylistsState onCreate={handleCreateOpen} />
      ) : (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
          {playlists.map(p => (
            <MoviePlaylistCard key={p.id} playlist={p} />
          ))}
        </div>
      )}

      <CreateMoviePlaylistDialog
        open={showCreate}
        onOpenChange={setShowCreate}
        restoreFocusRef={createPlaylistRestoreRef}
      />
    </div>
  );
}

type LikedMoviesInPlaylistsTabProps = {
  playlistsPage: number;
  sort: "asc" | "desc";
  onExitLiked: () => void;
  focusIntentRef: MutableRefObject<PlaylistsFocusIntent | null>;
};

function LikedMoviesInPlaylistsTab({
  playlistsPage,
  sort,
  onExitLiked,
  focusIntentRef,
}: LikedMoviesInPlaylistsTabProps) {
  const navigate = Route.useNavigate();
  const backToPlaylistsButtonRef = useRef<HTMLButtonElement | null>(null);

  const { data, isLoading, isError, refetch } = useQuery(
    likedMoviesQueryOpts(playlistsPage, MOVIES_PER_PAGE, sort),
  );

  const movies = data?.error === false ? data.data.movies : [];
  const totalPages = data?.error === false ? data.data.total_pages : 0;
  const total = data?.error === false ? data.data.total : 0;

  useEffect(() => {
    if (isLoading || focusIntentRef.current !== "enter-liked-from-toolbar") {
      return;
    }
    if (!backToPlaylistsButtonRef.current) return;

    focusIntentRef.current = null;
    focusDialogRestoreTarget(backToPlaylistsButtonRef.current);
  }, [focusIntentRef, isLoading]);

  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (movies.length === 0) return "No liked movies";
    return `Showing ${movies.length} liked movies, page ${playlistsPage} of ${totalPages}`;
  };

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        playlistsPage: newPage,
      }),
      replace: true,
    });
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const handleSortToggle = () =>
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        sort: prev.sort === "asc" ? "desc" : "asc",
        playlistsPage: 1,
      }),
      replace: true,
    });

  if (isLoading) {
    return <AllMoviesTabSkeleton />;
  }

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t load liked movies. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  if (movies.length === 0) {
    return (
      <div>
        <div className="mb-6 flex flex-wrap items-center gap-3">
          <button
            ref={backToPlaylistsButtonRef}
            type="button"
            onClick={onExitLiked}
            className="text-sm font-medium text-amber-400 hover:underline"
          >
            Back to playlists
          </button>
        </div>
        <div className="py-12 text-center text-slate-400">
          <Heart
            className="mx-auto mb-4 size-10 opacity-50"
            aria-hidden="true"
          />
          <p>You have not liked any movies yet.</p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <LiveAnnouncer message={getAnnouncement()} />

      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-wrap items-center gap-3">
          <button
            ref={backToPlaylistsButtonRef}
            type="button"
            onClick={onExitLiked}
            className="text-sm font-medium text-amber-400 hover:underline"
          >
            Back to playlists
          </button>
          <span className="text-sm text-slate-400">
            {total.toLocaleString()} liked
          </span>
        </div>
        <div className="flex flex-wrap items-center gap-2 sm:gap-3">
          <span className="text-sm text-slate-400">
            Page {playlistsPage} of {totalPages}
          </span>
          <button
            type="button"
            onClick={handleSortToggle}
            className="inline-flex items-center gap-1.5 rounded-full bg-slate-800 px-3 py-1.5 text-sm font-medium text-slate-300 transition-colors hover:bg-slate-700 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
            aria-label={
              sort === "asc"
                ? "Sorted A to Z, click to sort Z to A"
                : "Sorted Z to A, click to sort A to Z"
            }
          >
            {sort === "asc" ? (
              <>
                <ArrowDownAZ className="size-4" aria-hidden="true" />
                A–Z
              </>
            ) : (
              <>
                <ArrowUpAZ className="size-4" aria-hidden="true" />
                Z–A
              </>
            )}
          </button>
        </div>
      </div>

      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
        {movies.map(movie => (
          <MovieCard key={movie.id} movie={movie} />
        ))}
      </div>

      {totalPages > 1 && (
        <LibraryPagination
          currentPage={playlistsPage}
          totalPages={totalPages}
          onPageChange={handlePageChange}
        />
      )}
    </div>
  );
}

function PlaylistsTabSkeleton() {
  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div className="h-4 w-24 animate-pulse rounded-sm bg-slate-800" />
        <div className="h-10 w-40 animate-pulse rounded-full bg-slate-800" />
      </div>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: 10 }).map((_, i) => (
          <div
            key={i}
            className="animate-pulse rounded-xl border border-slate-800 bg-slate-900 p-4"
          >
            <div className="mx-auto mb-3 aspect-square w-full rounded-lg bg-slate-800" />
            <div className="mx-auto h-4 w-3/4 rounded-sm bg-slate-800" />
          </div>
        ))}
      </div>
    </div>
  );
}

type EmptyMoviePlaylistsStateProps = {
  onCreate: (event: MouseEvent<HTMLButtonElement>) => void;
};

function EmptyMoviePlaylistsState({ onCreate }: EmptyMoviePlaylistsStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center sm:py-16">
      <div className="mb-5 flex size-20 items-center justify-center rounded-full bg-linear-to-br from-slate-700 via-slate-800 to-amber-900/30 shadow-lg shadow-amber-500/5 sm:size-24">
        <ListVideo
          className="size-8 text-amber-200/40 sm:size-10"
          aria-hidden="true"
        />
      </div>
      <h3 className="mb-2 text-xl font-semibold text-white">
        No movie playlists yet
      </h3>
      <p className="mb-5 max-w-sm text-slate-400 sm:mb-6">
        Create a playlist to group films. Music playlists stay on the Music
        page.
      </p>
      <button
        type="button"
        onClick={onCreate}
        className="inline-flex min-h-11 items-center gap-2 rounded-full bg-amber-500 px-5 py-2.5 font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none sm:px-6 sm:py-3"
      >
        <Plus className="size-4" aria-hidden="true" />
        Create your first playlist
      </button>
    </div>
  );
}
