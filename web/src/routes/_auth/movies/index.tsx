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
  Film,
  Grid3X3,
  Heart,
  ListVideo,
  Plus,
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import CreateMoviePlaylistDialog from "@/components/movies/CreateMoviePlaylistDialog";
import MovieCard from "@/components/movies/MovieCard";
import MoviePlaylistCard from "@/components/movies/MoviePlaylistCard";
import LibraryAllTab, {
  type LibraryNoun,
} from "@/components/shared/LibraryAllTab";
import LibraryGenresTab from "@/components/shared/LibraryGenresTab";
import LibraryMoreMenu, {
  RefreshLibraryMenuItem,
} from "@/components/shared/LibraryMoreMenu";
import LibraryStats from "@/components/shared/LibraryStats";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import {
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_MENU_ITEM_CLASS,
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
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
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { isApiFailure } from "@/lib/is-api-failure";
import { refreshMovieLibraryCache } from "@/lib/movie-library-cache";
import { cn } from "@/lib/utils";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import RequestMovieDialog from "@/components/movies/RequestMovieDialog";
import {
  moviesSearchSchema,
  type MoviesSearchParams,
} from "@/lib/route-search";

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

const MOVIE_NOUN: LibraryNoun = { singular: "movie", plural: "movies" };

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
      <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS)}>
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
          <Film className="size-6 shrink-0 text-primary" aria-hidden="true" />
          <span>Movie Library</span>
        </h1>
        <p className="mt-1.5 max-w-2xl text-sm text-muted-foreground md:text-base">
          Browse, organize, and enjoy your film collection
        </p>
      </header>

      {/* Stats + More dropdown */}
      <div
        className={cn(
          "mb-5 flex flex-wrap items-center justify-between gap-3",
          MOTION_SECTION_ENTER_DELAYED_CLASS,
        )}
      >
        <LibraryStats
          queryOpts={moviesStatsQueryOpts()}
          figures={[
            {
              icon: Film,
              label: "Movies",
              noun: MOVIE_NOUN,
              getValue: data => data.total_movies,
            },
          ]}
        />
        <MoreMenu
          onOpenLikedMovies={handleOpenLikedMovies}
          onOpenMoviePlaylists={handleOpenMoviePlaylists}
        />
      </div>

      {/* Tabs — controlled by URL search param */}
      <Tabs
        value={tab}
        onValueChange={handleTabChange}
        className={MOTION_SECTION_ENTER_DELAYED_CLASS}
      >
        <TabsList
          className={cn(LIBRARY_TABS_LIST_CLASS, "grid-cols-3 sm:grid-cols-3")}
        >
          <TabsTrigger value="all" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Grid3X3
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            All Movies
          </TabsTrigger>
          <TabsTrigger
            value="genres"
            ref={genresTabTriggerRef}
            className={LIBRARY_TAB_TRIGGER_CLASS}
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
            className={LIBRARY_TAB_TRIGGER_CLASS}
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
// More dropdown
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
  const [menuOpen, setMenuOpen] = useState(false);
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
      <LibraryMoreMenu
        open={menuOpen}
        onOpenChange={setMenuOpen}
        triggerRef={moreOptionsButtonRef}
      >
        <DropdownMenuItem
          className={LIBRARY_MENU_ITEM_CLASS}
          onClick={onOpenLikedMovies}
        >
          <Heart className="mr-2 size-4" aria-hidden="true" />
          Liked movies
        </DropdownMenuItem>
        <DropdownMenuItem
          className={LIBRARY_MENU_ITEM_CLASS}
          onClick={onOpenMoviePlaylists}
        >
          <ListVideo className="mr-2 size-4" aria-hidden="true" />
          Movie playlists
        </DropdownMenuItem>
        <RefreshLibraryMenuItem
          refresh={refreshMovieLibraryCache}
          libraryNoun="Movie"
          onSettled={() => setMenuOpen(false)}
        />
        <DropdownMenuItem
          className={LIBRARY_MENU_ITEM_CLASS}
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
      </LibraryMoreMenu>

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

  return (
    <LibraryAllTab
      queryOpts={moviesLibraryQueryOpts(currentPage, MOVIES_PER_PAGE, sort)}
      getItems={data => data.movies}
      renderCard={movie => <MovieCard movie={movie} />}
      currentPage={currentPage}
      sort={sort}
      perPage={MOVIES_PER_PAGE}
      noun={MOVIE_NOUN}
      emptyIcon={Film}
      onPageChange={newPage =>
        navigate({
          to: "/movies",
          search: (prev: MoviesSearchParams) => ({
            ...prev,
            allPage: newPage,
          }),
          replace: true,
        })
      }
      onSortToggle={() =>
        navigate({
          to: "/movies",
          search: (prev: MoviesSearchParams) => ({
            ...prev,
            sort: prev.sort === "asc" ? "desc" : "asc",
            allPage: 1,
          }),
          replace: true,
        })
      }
    />
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

  return (
    <LibraryGenresTab
      genresQueryOpts={moviesGenresQueryOpts()}
      getGenres={data =>
        data.genres.map(g => ({
          genre_id: g.genre_id,
          genre_tag: g.genre_tag,
          count: g.movie_count,
        }))
      }
      genresListLabel="Movie genres"
      itemsQueryOpts={moviesByGenreQueryOpts(
        genreId ?? 0,
        genresPage,
        MOVIES_PER_PAGE,
        sort,
      )}
      getItems={data => data.movies}
      renderCard={movie => <MovieCard movie={movie} />}
      genreId={genreId}
      genresPage={genresPage}
      sort={sort}
      perPage={MOVIES_PER_PAGE}
      noun={MOVIE_NOUN}
      emptyIcon={Film}
      fallbackFocusRef={fallbackFocusRef}
      onSelectGenre={id =>
        navigate({
          to: "/movies",
          search: (prev: MoviesSearchParams) => ({
            ...prev,
            genreId: id,
            genresPage: 1,
          }),
          replace: true,
        })
      }
      onClearGenre={() =>
        navigate({
          to: "/movies",
          search: (prev: MoviesSearchParams) => ({
            ...prev,
            genreId: undefined,
            genresPage: 1,
          }),
          replace: true,
        })
      }
      onPageChange={newPage =>
        navigate({
          to: "/movies",
          search: (prev: MoviesSearchParams) => ({
            ...prev,
            genresPage: newPage,
          }),
          replace: true,
        })
      }
      onSortToggle={() =>
        navigate({
          to: "/movies",
          search: (prev: MoviesSearchParams) => ({
            ...prev,
            sort: prev.sort === "asc" ? "desc" : "asc",
            genresPage: 1,
          }),
          replace: true,
        })
      }
    />
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
        <span className="text-sm text-muted-foreground">
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
            className={cn(
              "inline-flex min-h-10 items-center gap-2 rounded-full border border-border px-3 py-2 text-sm font-medium text-muted-foreground hover:border-primary/50 hover:text-foreground sm:px-4",
              MOTION_MICRO_CONTROL_CLASS,
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            <Heart className="size-4 shrink-0" aria-hidden="true" />
            Liked movies
          </button>
          <button
            type="button"
            onClick={handleCreateOpen}
            className={cn(
              "inline-flex min-h-10 items-center gap-2 rounded-full bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 sm:px-4",
              MOTION_MICRO_CONTROL_CLASS,
              FOCUS_VISIBLE_RING_CLASS,
            )}
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

const LIKED_MOVIE_NOUN = { singular: "liked movie", plural: "liked movies" };

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

  // The same key LibraryAllTab runs below, so TanStack serves both from one
  // request; the page reads it only for the count beside the back link.
  const { data, isLoading } = useQuery(
    likedMoviesQueryOpts(playlistsPage, MOVIES_PER_PAGE, sort),
  );

  const total = data?.error === false ? data.data.total : 0;

  useEffect(() => {
    if (isLoading || focusIntentRef.current !== "enter-liked-from-toolbar") {
      return;
    }
    if (!backToPlaylistsButtonRef.current) return;

    focusIntentRef.current = null;
    focusDialogRestoreTarget(backToPlaylistsButtonRef.current);
  }, [focusIntentRef, isLoading]);

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/movies",
      search: (prev: MoviesSearchParams) => ({
        ...prev,
        playlistsPage: newPage,
      }),
      replace: true,
    });
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

  return (
    <LibraryAllTab
      queryOpts={likedMoviesQueryOpts(playlistsPage, MOVIES_PER_PAGE, sort)}
      getItems={data => data.movies}
      renderCard={movie => <MovieCard movie={movie} />}
      currentPage={playlistsPage}
      sort={sort}
      perPage={MOVIES_PER_PAGE}
      noun={LIKED_MOVIE_NOUN}
      emptyIcon={Heart}
      onPageChange={handlePageChange}
      onSortToggle={handleSortToggle}
      toolbarStartSlot={
        <>
          <button
            ref={backToPlaylistsButtonRef}
            type="button"
            onClick={onExitLiked}
            className={cn(
              "rounded-sm text-sm font-medium text-primary hover:underline",
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            Back to playlists
          </button>
          {data?.error === false && (
            <span className="text-sm text-muted-foreground">
              {total.toLocaleString()} liked
            </span>
          )}
        </>
      }
    />
  );
}

function PlaylistsTabSkeleton() {
  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div className={cn("h-4 w-24 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
        <div className={cn("h-10 w-40 rounded-full bg-muted", MOTION_LOADING_STATE_CLASS)} />
      </div>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: 10 }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "rounded-xl border border-border bg-card p-4",
              MOTION_LOADING_STATE_CLASS,
            )}
          >
            <div className="mx-auto mb-3 aspect-square w-full rounded-lg bg-muted" />
            <div className="mx-auto h-4 w-3/4 rounded-sm bg-muted" />
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
      <div className="mb-5 flex size-20 items-center justify-center rounded-full bg-linear-to-br from-muted via-muted to-primary/30 shadow-lg shadow-primary/5 sm:size-24">
        <ListVideo
          className="size-8 text-primary/40 sm:size-10"
          aria-hidden="true"
        />
      </div>
      <h3 className="mb-2 text-xl font-semibold text-foreground">
        No movie playlists yet
      </h3>
      <p className="mb-5 max-w-sm text-muted-foreground sm:mb-6">
        Create a playlist to group films. Music playlists stay on the Music
        page.
      </p>
      <button
        type="button"
        onClick={onCreate}
        className={cn(
          "inline-flex min-h-11 items-center gap-2 rounded-full bg-primary px-5 py-2.5 font-semibold text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 sm:px-6 sm:py-3",
          MOTION_MICRO_CONTROL_CLASS,
          FOCUS_VISIBLE_RING_CLASS,
        )}
      >
        <Plus className="size-4" aria-hidden="true" />
        Create your first playlist
      </button>
    </div>
  );
}
