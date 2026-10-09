import {
  useEffect,
  useRef,
  useState,
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
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import PlaylistFormDialog from "@/components/shared/PlaylistFormDialog";
import MovieCard from "@/components/movies/MovieCard";
import LibraryAllTab from "@/components/shared/LibraryAllTab";
import LibraryEmptyState from "@/components/shared/LibraryEmptyState";
import LibraryGenresTab from "@/components/shared/LibraryGenresTab";
import LibraryMoreMenu, {
  RequestMediaMenuItem,
  RefreshLibraryMenuItem,
} from "@/components/shared/LibraryMoreMenu";
import LibraryStats from "@/components/shared/LibraryStats";
import PlaylistCard, { PlaylistCardSkeleton } from "@/components/shared/PlaylistCard";
import PlaylistsTabToolbar from "@/components/shared/PlaylistsTabToolbar";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import {
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_MENU_ITEM_CLASS,
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MOVIES_PER_PAGE,
  MUSIC_CARD_GRID_CLASS,
  LIBRARY_NOUNS,
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
import LoadErrorAlert from "@/components/shared/LoadErrorAlert";
import { nounForCount } from "@/lib/format";
import { apiErrorMessage, isApiFailure } from "@/lib/is-api-failure";
import { refreshMovieLibraryCache } from "@/lib/movie-library-cache";
import { cn } from "@/lib/utils";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import RequestMovieDialog from "@/components/movies/RequestMovieDialog";
import {
  moviesSearchSchema,
  type MoviesSearchParams,
} from "@/lib/route-search";
import { routeHead } from "@/lib/route-head";
import { loadOnEntry } from "@/lib/route-loads";
import SkeletonStatus from "@/components/shared/SkeletonStatus";

const MOVIES_HEAD = routeHead(
  "Movies",
  "Browse and organize your personal movie collection in your Igloo media library.",
);

export const Route = createFileRoute("/_auth/movies/")({
  head: () => MOVIES_HEAD,
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
    cause,
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
    await loadOnEntry(cause, promises);
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
          "mb-5 flex items-start justify-between gap-3",
          MOTION_SECTION_ENTER_DELAYED_CLASS,
        )}
      >
        <LibraryStats
          className="min-w-0 flex-1"
          queryOpts={moviesStatsQueryOpts()}
          figures={[
            {
              icon: Film,
              label: "Movies",
              noun: LIBRARY_NOUNS.movie,
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
        <RequestMediaMenuItem
          label="Request Movie"
          provider="TMDB"
          available={tmdbAvailable}
          statusLoading={tmdbStatusLoading}
          onSelect={() => setRequestMovieOpen(true)}
        />
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
      noun={LIBRARY_NOUNS.movie}
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
      noun={LIBRARY_NOUNS.movie}
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

  // The toolbar renders while the list loads, so the Liked button can take
  // focus back as soon as this view mounts.
  useEffect(() => {
    if (view === "liked") return;
    if (focusIntentRef.current !== "return-to-playlists") return;

    focusIntentRef.current = null;
    focusDialogRestoreTarget(
      likedMoviesButtonRef.current,
      playlistsTabTriggerRef.current,
    );
  }, [focusIntentRef, playlistsTabTriggerRef, view]);

  const loadFailed = isError || isApiFailure(data);

  const handleShowLiked = () => {
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
  };

  const handleCreateOpen = () => {
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

  let body = (
    <div className={MUSIC_CARD_GRID_CLASS}>
      {playlists.map(p => (
        <PlaylistCard key={p.id} playlist={p} />
      ))}
    </div>
  );

  if (isLoading) {
    body = (
      <SkeletonStatus label="Loading playlists">
        <div className={MUSIC_CARD_GRID_CLASS}>
          {Array.from({ length: 10 }).map((_, i) => (
            <PlaylistCardSkeleton key={i} />
          ))}
        </div>
      </SkeletonStatus>
    );
  } else if (loadFailed) {
    body = (
      <LoadErrorAlert
        message={apiErrorMessage(data, "Couldn’t load playlists. Check your connection and try again.")}
        onRetry={() => void refetch()}
      />
    );
  } else if (playlists.length === 0) {
    body = (
      <LibraryEmptyState
        icon={ListVideo}
        message="No movie playlists yet. Use New playlist to group films."
      />
    );
  }

  return (
    <div>
      <PlaylistsTabToolbar
        count={isLoading || loadFailed ? undefined : playlists.length}
        isLoading={isLoading}
        likedLabel="Liked movies"
        onShowLiked={handleShowLiked}
        onCreate={handleCreateOpen}
        likedButtonRef={likedMoviesButtonRef}
        createButtonRef={createPlaylistRestoreRef}
      />

      {body}

      <PlaylistFormDialog
        kind="movie"
        mode="create"
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
              {total.toLocaleString()} {nounForCount(total, LIKED_MOVIE_NOUN)}
            </span>
          )}
        </>
      }
    />
  );
}

