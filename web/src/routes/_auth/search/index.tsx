import { createFileRoute, Link, redirect, stripSearchParams } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Search, Film, Tv, Disc3, User, Music } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { type LibraryNoun } from "@/components/shared/LibraryAllTab";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import LibraryPagination from "@/components/shared/LibraryPagination";
import LoadErrorAlert from "@/components/shared/LoadErrorAlert";
import { PosterCardSkeleton } from "@/components/shared/PosterCard";
import MovieCard from "@/components/movies/MovieCard";
import ShowCard from "@/components/shows/ShowCard";
import AlbumCard from "@/components/music/AlbumCard";
import MusicianCard from "@/components/music/MusicianCard";
import TrackItem from "@/components/music/TrackItem";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import { trackRowProps } from "@/lib/track-row-props";
import { apiErrorMessage, isApiFailure } from "@/lib/is-api-failure";
import {
  searchAllQueryOpts,
  searchCategoryQueryOpts,
} from "@/lib/query-opts";
import {
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_POSTER_GRID_CLASS,
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  SEARCH_PER_PAGE,
  TRACK_LIST_CONTAINER_CLASS,
  LIBRARY_NOUNS,
  SEARCH_INDEX_DEFAULT_SEARCH,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import { scrollWindowToTop } from "@/lib/motion";
import type {
  PagedSearchTab,
  SearchCategoryData,
  SearchTab,
  TrackListItemType,
} from "@/types";
import { searchSearchSchema, type SearchParams } from "@/lib/route-search";
import { nounForCount, pluralize } from "@/lib/format";
import { routeHead } from "@/lib/route-head";
import { loadOnEntry } from "@/lib/route-loads";
import SkeletonStatus from "@/components/shared/SkeletonStatus";

// The tab value doubles as the visible category word, so each one carries both
// forms - a single result reads "1 show", not "1 shows".
const SEARCH_TAB_NOUNS: Record<PagedSearchTab, LibraryNoun> = {
  movies: LIBRARY_NOUNS.movie,
  shows: LIBRARY_NOUNS.show,
  albums: LIBRARY_NOUNS.album,
  musicians: LIBRARY_NOUNS.musician,
  tracks: LIBRARY_NOUNS.track,
};

function redirectToLastSearchPage({
  q,
  tab,
  requestedPage,
  totalPages,
}: {
  q: string;
  tab: PagedSearchTab;
  requestedPage: number;
  totalPages: number;
}) {
  if (totalPages === 0 || requestedPage <= totalPages) {
    return;
  }

  throw redirect({
    to: "/search",
    search: {
      q,
      tab,
      page: totalPages,
    },
    replace: true,
  });
}

const SEARCH_HEAD = routeHead("Search");

export const Route = createFileRoute("/_auth/search/")({
  validateSearch: searchSearchSchema,
  search: { middlewares: [stripSearchParams(SEARCH_INDEX_DEFAULT_SEARCH)] },
  loaderDeps: ({ search: { q, tab, page } }) => ({ q, tab, page }),
  loader: async ({ context, cause, deps: { q, tab, page } }) => {
    const trimmed = q.trim();
    if (!trimmed) return;

    const { queryClient } = context;
    if (tab === "all") {
      await loadOnEntry(cause, [
        queryClient.ensureQueryData(searchAllQueryOpts(trimmed)),
      ]);
      return;
    }

    const results = queryClient.ensureQueryData(
      searchCategoryQueryOpts(tab, trimmed, page, SEARCH_PER_PAGE),
    );
    // A new query, tab or page from the header and the pager is always in
    // range, so only an entered URL needs the last-page clamp below.
    if (cause === "stay") {
      await loadOnEntry(cause, [results]);
      return;
    }

    const result = await results;

    if (result.error === false) {
      redirectToLastSearchPage({
        q: trimmed,
        tab,
        requestedPage: page,
        totalPages: result.data.total_pages,
      });
    }
  },
  head: ({ match }) => {
    const trimmed = match.search.q.trim();

    return trimmed
      ? routeHead(
          `Search: ${trimmed}`,
          `Search results in your Igloo library for "${trimmed}".`,
        )
      : SEARCH_HEAD;
  },
  component: SearchPage,
});

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
        <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS)}>
          <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
            <Search
              className="size-6 shrink-0 text-primary"
              aria-hidden="true"
            />
            <span>Search</span>
          </h1>
          <p className="mt-1.5 max-w-2xl text-sm text-muted-foreground md:text-base">
            Type a query in the search bar above to find movies, shows,
            albums, musicians, and tracks in your library.
          </p>
        </header>
      </div>
    );
  }

  let topLevelTabContent = <AllResultsTab q={trimmed} />;

  if (tab === "movies") {
    topLevelTabContent = (
      <CategoryResultsTab
        kind="movies"
        q={trimmed}
        page={page}
        renderGrid={(items) => (
          <div className={LIBRARY_POSTER_GRID_CLASS}>
            {items.map((movie) => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        )}
      />
    );
  }

  if (tab === "shows") {
    topLevelTabContent = (
      <CategoryResultsTab
        kind="shows"
        q={trimmed}
        page={page}
        renderGrid={(items) => (
          <div className={LIBRARY_POSTER_GRID_CLASS}>
            {items.map((show) => (
              <ShowCard key={show.id} show={show} />
            ))}
          </div>
        )}
      />
    );
  }

  if (tab === "albums") {
    topLevelTabContent = (
      <CategoryResultsTab
        kind="albums"
        q={trimmed}
        page={page}
        renderGrid={(items) => (
          <div className={LIBRARY_POSTER_GRID_CLASS}>
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
        kind="musicians"
        q={trimmed}
        page={page}
        renderGrid={(items) => (
          <div className={LIBRARY_POSTER_GRID_CLASS}>
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
        kind="tracks"
        q={trimmed}
        page={page}
        renderGrid={(items) => (
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
          className={cn(
            LIBRARY_TABS_LIST_CLASS,
            "grid-cols-2 sm:grid-cols-3 lg:grid-cols-6",
          )}
        >
          <TabsTrigger value="all" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Search
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            All
          </TabsTrigger>
          <TabsTrigger value="movies" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Film
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Movies
          </TabsTrigger>
          <TabsTrigger value="shows" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Tv
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Shows
          </TabsTrigger>
          <TabsTrigger value="albums" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Disc3
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Albums
          </TabsTrigger>
          <TabsTrigger value="musicians" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <User
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Musicians
          </TabsTrigger>
          <TabsTrigger
            value="tracks"
            className={LIBRARY_TAB_TRIGGER_CLASS}
          >
            <Music
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
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
      <LoadErrorAlert
        message={apiErrorMessage(data, "Couldn’t run that search.")}
        onRetry={() => void refetch()}
      />
    );
  }

  if (data?.error !== false) {
    return null;
  }

  const { movies, shows, albums, musicians, tracks } = data.data;
  const totalAll =
    movies.total + shows.total + albums.total + musicians.total + tracks.total;

  const announcement =
    totalAll === 0
      ? `No results for ${q}`
      : `${totalAll.toLocaleString()} results for ${q}: ${pluralize(movies.total, "movie")}, ${pluralize(shows.total, "show")}, ${pluralize(albums.total, "album")}, ${pluralize(musicians.total, "musician")}, ${pluralize(tracks.total, "track")}`;

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
          <div className={LIBRARY_POSTER_GRID_CLASS}>
            {movies.results.map((movie) => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        </AllSection>
      )}

      {shows.total > 0 && (
        <AllSection
          icon={<Tv className="size-5 text-primary" aria-hidden="true" />}
          title="Shows"
          total={shows.total}
          resultCount={shows.results.length}
          tab="shows"
          q={q}
        >
          <div className={LIBRARY_POSTER_GRID_CLASS}>
            {shows.results.map((show) => (
              <ShowCard key={show.id} show={show} />
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
          <div className={LIBRARY_POSTER_GRID_CLASS}>
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
          <div className={LIBRARY_POSTER_GRID_CLASS}>
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
// Category tabs — one generic component handling loading / error / empty /
// pagination; only the grid renderer differs per category
// ---------------------------------------------------------------------------

type CategoryResultsTabProps<K extends PagedSearchTab> = {
  kind: K;
  q: string;
  page: number;
  renderGrid: (items: SearchCategoryData[K]["results"]) => React.ReactNode;
};

function CategoryResultsTab<K extends PagedSearchTab>({
  kind,
  q,
  page,
  renderGrid,
}: CategoryResultsTabProps<K>) {
  const navigate = Route.useNavigate();
  const { data, isLoading, isError, refetch } = useQuery(
    searchCategoryQueryOpts(kind, q, page, SEARCH_PER_PAGE),
  );

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/search",
      search: (prev: SearchParams) => ({ ...prev, page: newPage }),
      replace: true,
    });
    scrollWindowToTop();
  };

  if (isLoading) {
    return <CategorySkeleton />;
  }

  if (isError || isApiFailure(data)) {
    return (
      <LoadErrorAlert
        message={apiErrorMessage(data, `Couldn’t load ${kind}.`)}
        onRetry={() => void refetch()}
      />
    );
  }

  const results = data?.error === false ? data.data.results : [];
  const total = data?.error === false ? data.data.total : 0;
  const currentPage = data?.error === false ? data.data.page : page;
  const totalPages = data?.error === false ? data.data.total_pages : 0;

  if (results.length === 0) {
    return (
      <>
        <LiveAnnouncer
          message={`No ${kind} match ${q}`}
          announcementKey={`${q}-${kind}-empty`}
        />
        <p className="py-12 text-center text-muted-foreground">
          No {kind} match &lsquo;{q}&rsquo;.
        </p>
      </>
    );
  }

  const noun = SEARCH_TAB_NOUNS[kind];
  const announcement = `Showing ${results.length} ${nounForCount(results.length, noun)}, page ${currentPage} of ${totalPages}, ${total.toLocaleString()} total`;

  return (
    <div>
      <LiveAnnouncer
        message={announcement}
        announcementKey={`${q}-${kind}-${currentPage}`}
      />

      <div className="mb-5 flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">
          {total.toLocaleString()} {nounForCount(total, noun)}
        </span>
        {totalPages > 1 && (
          <span className="text-sm text-muted-foreground">
            Page {currentPage} of {totalPages}
          </span>
        )}
      </div>

      <div className="mb-8">{renderGrid(results)}</div>

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

// ---------------------------------------------------------------------------
// Track list and row that can play through the audio player
// ---------------------------------------------------------------------------

function TracksResultsList({ tracks }: { tracks: TrackListItemType[] }) {
  return (
    <ul
      className={TRACK_LIST_CONTAINER_CLASS}
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
  const matchTrackPlayback = useTrackPlaybackMatcher();

  const handlePlay = () => {
    audioPlayer.playTrackFromList(queue, track.id);
  };

  return (
    <TrackItem
      {...trackRowProps(track)}
      variant="library"
      {...matchTrackPlayback(track.id)}
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

function CategorySkeleton() {
  return (
    <SkeletonStatus label="Loading search results">
      <div className={cn("mb-5 h-4 w-32 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
      <div className={LIBRARY_POSTER_GRID_CLASS}>
        {Array.from({ length: SEARCH_PER_PAGE }).map((_, i) => (
          <PosterCardSkeleton key={i} />
        ))}
      </div>
    </SkeletonStatus>
  );
}

function AllResultsSkeleton() {
  return (
    <SkeletonStatus label="Loading search results">
      <div className="space-y-10">
        {Array.from({ length: 3 }).map((_, s) => (
          <div key={s}>
            <div className={cn("mb-4 h-6 w-40 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
            <div className={LIBRARY_POSTER_GRID_CLASS}>
              {Array.from({ length: 6 }).map((_, i) => (
                <PosterCardSkeleton key={i} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </SkeletonStatus>
  );
}
