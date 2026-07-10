import { createLazyFileRoute, Link } from "@tanstack/react-router";
import { useQuery, type UseQueryOptions } from "@tanstack/react-query";
import { Search, Film, Disc3, User, Music } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import LibraryPagination from "@/components/LibraryPagination";
import MovieCard from "@/components/MovieCard";
import AlbumCard from "@/components/AlbumCard";
import MusicianCard from "@/components/MusicianCard";
import TrackItem from "@/components/TrackItem";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useAudioPlayerState } from "@/hooks/useAudioPlayerState";
import {
  unwrapInt,
  unwrapString,
  unwrapStringOrUndefined,
} from "@/lib/nullable";
import { isApiFailure } from "@/lib/is-api-failure";
import {
  searchAlbumsQueryOpts,
  searchAllQueryOpts,
  searchMoviesQueryOpts,
  searchMusiciansQueryOpts,
  searchTracksQueryOpts,
} from "@/lib/query-opts";
import {
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  SEARCH_PER_PAGE,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type {
  ApiResponseType,
  MoviesLibraryListItemType,
  PaginatedSearchResponse,
  SearchTab,
  SimpleAlbumType,
  SimpleMusicianType,
  TrackListItemType,
} from "@/types";
import type { SearchParams } from "@/types/route-search";

export const Route = createLazyFileRoute("/_auth/search/")({
  component: SearchPage,
});

const SEARCH_GRID_CLASS =
  "grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6";

function SearchPage() {
  const navigate = Route.useNavigate();
  const { q, tab, page } = Route.useSearch();
  const trimmed = q.trim();
  const { isExiting, runTransition, usesContentAnimation } =
    useContentFadeTransition(CONTENT_FADE_TRANSITION_MS);

  const handleTabChange = (newTab: string) => {
    const nextTab = newTab as SearchTab;

    runTransition({
      shouldAnimate: nextTab !== tab,
      onTransition: () =>
        navigate({
          to: "/search",
          search: (prev: SearchParams) => ({
            ...prev,
            tab: nextTab,
            page: 1,
          }),
          replace: true,
        }),
    });
  };

  if (!trimmed) {
    return (
      <div className="min-w-0">
        <title>Search - Igloo</title>
        <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS)}>
          <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
            <Search
              className="size-6 shrink-0 text-primary"
              aria-hidden="true"
            />
            <span>Search</span>
          </h1>
          <p className="mt-1.5 max-w-2xl text-sm text-muted-foreground md:text-base">
            Type a query in the search bar above to find movies, albums,
            musicians, and tracks in your library.
          </p>
        </header>
      </div>
    );
  }

  let topLevelTabContent = <AllResultsTab q={trimmed} />;

  if (tab === "movies") {
    topLevelTabContent = (
      <CategoryResultsTab
        label="movies"
        q={trimmed}
        page={page}
        queryOpts={searchMoviesQueryOpts(trimmed, page, SEARCH_PER_PAGE)}
        renderGrid={(items: MoviesLibraryListItemType[]) => (
          <div className={SEARCH_GRID_CLASS}>
            {items.map((movie) => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        )}
      />
    );
  }

  if (tab === "albums") {
    topLevelTabContent = (
      <CategoryResultsTab
        label="albums"
        q={trimmed}
        page={page}
        queryOpts={searchAlbumsQueryOpts(trimmed, page, SEARCH_PER_PAGE)}
        renderGrid={(items: SimpleAlbumType[]) => (
          <div className={SEARCH_GRID_CLASS}>
            {items.map((album) => (
              <AlbumCard key={album.id} album={album} />
            ))}
          </div>
        )}
      />
    );
  }

  if (tab === "musicians") {
    topLevelTabContent = (
      <CategoryResultsTab
        label="musicians"
        q={trimmed}
        page={page}
        queryOpts={searchMusiciansQueryOpts(trimmed, page, SEARCH_PER_PAGE)}
        renderGrid={(items: SimpleMusicianType[]) => (
          <div className={SEARCH_GRID_CLASS}>
            {items.map((musician) => (
              <MusicianCard key={musician.id} musician={musician} />
            ))}
          </div>
        )}
      />
    );
  }

  if (tab === "tracks") {
    topLevelTabContent = (
      <CategoryResultsTab
        label="tracks"
        q={trimmed}
        page={page}
        queryOpts={searchTracksQueryOpts(trimmed, page, SEARCH_PER_PAGE)}
        renderGrid={(items: TrackListItemType[]) => (
          <TracksResultsList tracks={items} />
        )}
      />
    );
  }

  const topLevelTabPanelClassName = cn(
    usesContentAnimation &&
      (isExiting ? CONTENT_FADE_EXIT_CLASS : CONTENT_FADE_ENTER_CLASS),
  );

  return (
    <div className="min-w-0">
      <title>{`Search: ${trimmed} - Igloo`}</title>
      <meta
        name="description"
        content={`Search results in your Igloo library for "${trimmed}".`}
      />

      <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS)}>
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
          <Search
            className="size-6 shrink-0 text-primary"
            aria-hidden="true"
          />
          <span>
            Search results for{" "}
            <span className="text-primary">&lsquo;{trimmed}&rsquo;</span>
          </span>
        </h1>
      </header>

      <Tabs
        value={tab}
        onValueChange={handleTabChange}
        className={MOTION_SECTION_ENTER_DELAYED_CLASS}
      >
        <TabsList
          className={cn(LIBRARY_TABS_LIST_CLASS, "grid-cols-2 sm:grid-cols-5")}
        >
          <TabsTrigger value="all" className={LIBRARY_TAB_TRIGGER_CLASS}>
            All
          </TabsTrigger>
          <TabsTrigger value="movies" className={LIBRARY_TAB_TRIGGER_CLASS}>
            Movies
          </TabsTrigger>
          <TabsTrigger value="albums" className={LIBRARY_TAB_TRIGGER_CLASS}>
            Albums
          </TabsTrigger>
          <TabsTrigger value="musicians" className={LIBRARY_TAB_TRIGGER_CLASS}>
            Musicians
          </TabsTrigger>
          <TabsTrigger
            value="tracks"
            className={LIBRARY_TAB_TRIGGER_CLASS}
          >
            Tracks
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
// All tab — top N of each entity with "See all" links
// ---------------------------------------------------------------------------

function AllResultsTab({ q }: { q: string }) {
  const { data, isLoading, isError, refetch } = useQuery(
    searchAllQueryOpts(q),
  );

  if (isLoading) {
    return <AllResultsSkeleton />;
  }

  if (isError || isApiFailure(data)) {
    return (
      <SearchLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t run that search. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  if (data?.error !== false) {
    return null;
  }

  const { movies, albums, musicians, tracks } = data.data;
  const totalAll =
    movies.total + albums.total + musicians.total + tracks.total;

  const announcement =
    totalAll === 0
      ? `No results for ${q}`
      : `${totalAll.toLocaleString()} results for ${q}: ${movies.total} movies, ${albums.total} albums, ${musicians.total} musicians, ${tracks.total} tracks`;

  if (totalAll === 0) {
    return (
      <>
        <LiveAnnouncer message={announcement} announcementKey={q} />
        <EmptyResults q={q} />
      </>
    );
  }

  return (
    <div className="space-y-10">
      <LiveAnnouncer message={announcement} announcementKey={q} />

      {movies.total > 0 && (
        <AllSection
          icon={<Film className="size-5 text-primary" aria-hidden="true" />}
          title="Movies"
          total={movies.total}
          resultCount={movies.results.length}
          tab="movies"
          q={q}
        >
          <div className={SEARCH_GRID_CLASS}>
            {movies.results.map((movie) => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        </AllSection>
      )}

      {albums.total > 0 && (
        <AllSection
          icon={<Disc3 className="size-5 text-primary" aria-hidden="true" />}
          title="Albums"
          total={albums.total}
          resultCount={albums.results.length}
          tab="albums"
          q={q}
        >
          <div className={SEARCH_GRID_CLASS}>
            {albums.results.map((album) => (
              <AlbumCard key={album.id} album={album} />
            ))}
          </div>
        </AllSection>
      )}

      {musicians.total > 0 && (
        <AllSection
          icon={<User className="size-5 text-primary" aria-hidden="true" />}
          title="Musicians"
          total={musicians.total}
          resultCount={musicians.results.length}
          tab="musicians"
          q={q}
        >
          <div className={SEARCH_GRID_CLASS}>
            {musicians.results.map((musician) => (
              <MusicianCard key={musician.id} musician={musician} />
            ))}
          </div>
        </AllSection>
      )}

      {tracks.total > 0 && (
        <AllSection
          icon={<Music className="size-5 text-primary" aria-hidden="true" />}
          title="Tracks"
          total={tracks.total}
          resultCount={tracks.results.length}
          tab="tracks"
          q={q}
        >
          <TracksResultsList tracks={tracks.results} />
        </AllSection>
      )}
    </div>
  );
}

type AllSectionProps = {
  icon: React.ReactNode;
  title: string;
  total: number;
  resultCount: number;
  tab: SearchTab;
  q: string;
  children: React.ReactNode;
};

function AllSection({
  icon,
  title,
  total,
  resultCount,
  tab,
  q,
  children,
}: AllSectionProps) {
  const showSeeAll = total > resultCount;

  return (
    <section aria-label={`${title} results`}>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-2 text-xl font-semibold text-foreground">
          {icon}
          <span>{title}</span>
          <span className="text-sm font-normal text-muted-foreground">
            ({total.toLocaleString()})
          </span>
        </h2>
        {showSeeAll && (
          <Link
            to="/search"
            search={{ q, tab, page: 1 }}
            className={cn(
              "rounded-sm text-sm font-medium text-primary hover:underline",
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            See all {total.toLocaleString()} {title.toLowerCase()} →
          </Link>
        )}
      </div>
      {children}
    </section>
  );
}

// ---------------------------------------------------------------------------
// Category tabs — one generic component; only the query options and the grid
// renderer differ per category
// ---------------------------------------------------------------------------

type CategoryResultsTabProps<T> = {
  label: Exclude<SearchTab, "all">;
  q: string;
  page: number;
  queryOpts: UseQueryOptions<ApiResponseType<PaginatedSearchResponse<T>>>;
  renderGrid: (items: T[]) => React.ReactNode;
};

function CategoryResultsTab<T>({
  label,
  q,
  page,
  queryOpts,
  renderGrid,
}: CategoryResultsTabProps<T>) {
  const navigate = Route.useNavigate();
  const { data, isLoading, isError, refetch } = useQuery(queryOpts);

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/search",
      search: (prev: SearchParams) => ({ ...prev, page: newPage }),
      replace: true,
    });
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  return (
    <CategoryTabFrame
      label={label}
      q={q}
      isLoading={isLoading}
      isError={isError}
      isApiFailure={isApiFailure(data)}
      message={isApiFailure(data) ? data.message : undefined}
      onRetry={() => void refetch()}
      results={data?.error === false ? data.data.results : []}
      total={data?.error === false ? data.data.total : 0}
      page={data?.error === false ? data.data.page : page}
      totalPages={data?.error === false ? data.data.total_pages : 0}
      onPageChange={handlePageChange}
      renderGrid={renderGrid}
    />
  );
}

// ---------------------------------------------------------------------------
// Shared category-tab frame: handles loading / error / empty / pagination
// ---------------------------------------------------------------------------

type CategoryTabFrameProps<T> = {
  label: "movies" | "albums" | "musicians" | "tracks";
  q: string;
  page: number;
  isLoading: boolean;
  isError: boolean;
  isApiFailure: boolean;
  message: string | undefined;
  onRetry: () => void;
  results: T[];
  total: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  renderGrid: (items: T[]) => React.ReactNode;
};

function CategoryTabFrame<T>({
  label,
  q,
  page,
  isLoading,
  isError,
  isApiFailure: isFailure,
  message,
  onRetry,
  results,
  total,
  totalPages,
  onPageChange,
  renderGrid,
}: CategoryTabFrameProps<T>) {
  if (isLoading) {
    return <CategorySkeleton />;
  }

  if (isError || isFailure) {
    return (
      <SearchLoadError
        message={
          message ??
          `Couldn’t load ${label}. Check your connection and try again.`
        }
        onRetry={onRetry}
      />
    );
  }

  if (results.length === 0) {
    return (
      <>
        <LiveAnnouncer
          message={`No ${label} match ${q}`}
          announcementKey={`${q}-${label}-empty`}
        />
        <p className="py-12 text-center text-muted-foreground">
          No {label} match &lsquo;{q}&rsquo;.
        </p>
      </>
    );
  }

  const announcement = `Showing ${results.length} ${label}, page ${page} of ${totalPages}, ${total.toLocaleString()} total`;

  return (
    <div>
      <LiveAnnouncer
        message={announcement}
        announcementKey={`${q}-${label}-${page}`}
      />

      <div className="mb-5 flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">
          {total.toLocaleString()} {label}
        </span>
        {totalPages > 1 && (
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
        )}
      </div>

      <div className="mb-8">{renderGrid(results)}</div>

      {totalPages > 1 && (
        <LibraryPagination
          currentPage={page}
          totalPages={totalPages}
          onPageChange={onPageChange}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Track list and row that can play through the audio player
// ---------------------------------------------------------------------------

function TracksResultsList({ tracks }: { tracks: TrackListItemType[] }) {
  return (
    <ul
      className="overflow-hidden rounded-xl border border-border bg-card/50"
      aria-label="Track results"
    >
      {tracks.map((track) => (
        <li key={track.id} className="border-b border-border last:border-b-0">
          <SearchTrackItem track={track} queue={tracks} />
        </li>
      ))}
    </ul>
  );
}

function SearchTrackItem({
  track,
  queue,
}: {
  track: TrackListItemType;
  queue: TrackListItemType[];
}) {
  const audioPlayer = useAudioPlayerActions();
  const playerState = useAudioPlayerState();

  const handlePlay = () => {
    audioPlayer.playTrackFromList(queue, track.id);
  };

  return (
    <TrackItem
      id={track.id}
      title={track.title}
      duration={track.duration}
      subtitle={unwrapString(track.musician_name) ?? "Unknown Artist"}
      albumId={unwrapInt(track.album_id)}
      albumTitle={unwrapStringOrUndefined(track.album_title)}
      musicianId={unwrapInt(track.musician_id)}
      musicianName={unwrapStringOrUndefined(track.musician_name)}
      variant="library"
      isPlaying={
        playerState.currentTrack?.id === track.id && playerState.isPlaying
      }
      isCurrentTrack={playerState.currentTrack?.id === track.id}
      onPlay={handlePlay}
      showActionsMenu
    />
  );
}

// ---------------------------------------------------------------------------
// Empty / error / skeleton helpers
// ---------------------------------------------------------------------------

function EmptyResults({ q }: { q: string }) {
  return (
    <div className="py-12 text-center text-muted-foreground">
      <Search className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
      <p>
        No results found for &lsquo;{q}&rsquo;. Try a different search term.
      </p>
    </div>
  );
}

type SearchLoadErrorProps = {
  message: string;
  onRetry: () => void;
};

function SearchLoadError({ message, onRetry }: SearchLoadErrorProps) {
  return (
    <div
      role="alert"
      className="rounded-xl border border-destructive/30 bg-destructive/10 p-6 text-center"
    >
      <p className="mb-4 text-sm text-destructive">{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className={cn(
          "inline-flex min-h-10 items-center gap-2 rounded-full bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90",
          MOTION_MICRO_CONTROL_CLASS,
          FOCUS_VISIBLE_RING_CLASS,
        )}
      >
        Try again
      </button>
    </div>
  );
}

function CategorySkeleton() {
  return (
    <div>
      <div className={cn("mb-5 h-4 w-32 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
      <div className={SEARCH_GRID_CLASS}>
        {Array.from({ length: SEARCH_PER_PAGE }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "overflow-hidden rounded-xl border border-border bg-card",
              MOTION_LOADING_STATE_CLASS,
            )}
          >
            <div className="aspect-2/3 bg-muted" />
            <div className="p-3">
              <div className="h-4 w-3/4 rounded-sm bg-muted" />
              <div className="mt-2 h-3 w-1/2 rounded-sm bg-muted" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function AllResultsSkeleton() {
  return (
    <div className="space-y-10">
      {Array.from({ length: 3 }).map((_, s) => (
        <div key={s}>
          <div className={cn("mb-4 h-6 w-40 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
          <div className={SEARCH_GRID_CLASS}>
            {Array.from({ length: 6 }).map((_, i) => (
              <div
                key={i}
                className={cn(
                  "overflow-hidden rounded-xl border border-border bg-card",
                  MOTION_LOADING_STATE_CLASS,
                )}
              >
                <div className="aspect-2/3 bg-muted" />
                <div className="p-3">
                  <div className="h-4 w-3/4 rounded-sm bg-muted" />
                  <div className="mt-2 h-3 w-1/2 rounded-sm bg-muted" />
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
