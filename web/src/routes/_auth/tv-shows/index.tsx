import { useRef, useState, type RefObject } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { Grid3X3, Tv } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import ShowCard from "@/components/shows/ShowCard";
import LibraryAllTab, { type LibraryNoun } from "@/components/shared/LibraryAllTab";
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
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  SHOWS_PER_PAGE,
} from "@/lib/constants";
import {
  showsByGenreQueryOpts,
  showsGenresQueryOpts,
  showsLibraryQueryOpts,
  showsStatsQueryOpts,
} from "@/lib/query-opts";
import { refreshShowLibraryCache } from "@/lib/show-library-cache";
import { cn } from "@/lib/utils";
import {
  showsSearchSchema,
  type ShowsSearchParams,
} from "@/lib/route-search";

export const Route = createFileRoute("/_auth/tv-shows/")({
  validateSearch: showsSearchSchema,
  loaderDeps: ({ search: { allPage, sort, tab, genreId, genresPage } }) => ({
    allPage,
    sort,
    tab,
    genreId,
    genresPage,
  }),
  loader: async ({
    context,
    deps: { allPage, sort, tab, genreId, genresPage },
  }) => {
    const { queryClient } = context;
    const promises: Promise<unknown>[] = [
      queryClient.ensureQueryData(showsStatsQueryOpts()),
      queryClient.ensureQueryData(
        showsLibraryQueryOpts(allPage, SHOWS_PER_PAGE, sort),
      ),
    ];
    if (tab === "genres") {
      promises.push(queryClient.ensureQueryData(showsGenresQueryOpts()));
      if (genreId != null && genreId > 0) {
        promises.push(
          queryClient.ensureQueryData(
            showsByGenreQueryOpts(genreId, genresPage, SHOWS_PER_PAGE, sort),
          ),
        );
      }
    }
    await Promise.all(promises);
  },
  component: TvShowsPage,
});

const SHOW_NOUN: LibraryNoun = { singular: "show", plural: "shows" };

// ---------------------------------------------------------------------------
// Page component
// ---------------------------------------------------------------------------

function TvShowsPage() {
  const navigate = Route.useNavigate();
  const { tab, allPage, sort, genreId, genresPage } = Route.useSearch();
  const genresTabTriggerRef = useRef<HTMLButtonElement | null>(null);
  const { isExiting, runTransition, usesContentAnimation } =
    useContentFadeTransition(CONTENT_FADE_TRANSITION_MS);

  const topLevelTabPanelClassName = cn(
    usesContentAnimation &&
      (isExiting ? CONTENT_FADE_EXIT_CLASS : CONTENT_FADE_ENTER_CLASS),
  );

  const topLevelTabContent =
    tab === "genres" ? (
      <GenresTabContent
        genreId={genreId}
        genresPage={genresPage}
        sort={sort}
        fallbackFocusRef={genresTabTriggerRef}
      />
    ) : (
      <AllShowsTabContent currentPage={allPage} sort={sort} />
    );

  const handleTabChange = (newTab: string) => {
    const nextTab = newTab as ShowsSearchParams["tab"];

    runTransition({
      shouldAnimate: nextTab !== tab,
      onTransition: () =>
        navigate({
          to: "/tv-shows",
          search: prev => ({ ...prev, tab: nextTab }),
          replace: true,
        }),
    });
  };

  return (
    <div className="min-w-0">
      <title>TV Shows - Igloo</title>
      <meta
        name="description"
        content="Browse and track your TV show library in your Igloo media center."
      />

      {/* Page header */}
      <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS)}>
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
          <Tv className="size-6 shrink-0 text-primary" aria-hidden="true" />
          <span>TV Show Library</span>
        </h1>
        <p className="mt-1.5 max-w-2xl text-sm text-muted-foreground md:text-base">
          Browse and follow your series collection
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
          queryOpts={showsStatsQueryOpts()}
          getTotal={data => data.total_shows}
          icon={Tv}
          label="Shows"
          noun={SHOW_NOUN}
        />
        <MoreMenu />
      </div>

      {/* Tabs — controlled by URL search param */}
      <Tabs
        value={tab}
        onValueChange={handleTabChange}
        className={MOTION_SECTION_ENTER_DELAYED_CLASS}
      >
        <TabsList className={cn(LIBRARY_TABS_LIST_CLASS, "grid-cols-2")}>
          <TabsTrigger value="all" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Grid3X3
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            All Shows
          </TabsTrigger>
          <TabsTrigger
            value="genres"
            ref={genresTabTriggerRef}
            className={LIBRARY_TAB_TRIGGER_CLASS}
          >
            <Tv
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Genres
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

function MoreMenu() {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <LibraryMoreMenu open={menuOpen} onOpenChange={setMenuOpen}>
      <RefreshLibraryMenuItem
        refresh={refreshShowLibraryCache}
        libraryNoun="Show"
        onSettled={() => setMenuOpen(false)}
      />
    </LibraryMoreMenu>
  );
}

// ---------------------------------------------------------------------------
// All Shows tab
// ---------------------------------------------------------------------------

type AllShowsTabContentProps = {
  currentPage: number;
  sort: "asc" | "desc";
};

function AllShowsTabContent({ currentPage, sort }: AllShowsTabContentProps) {
  const navigate = Route.useNavigate();

  return (
    <LibraryAllTab
      queryOpts={showsLibraryQueryOpts(currentPage, SHOWS_PER_PAGE, sort)}
      getItems={data => data.shows}
      renderCard={show => <ShowCard show={show} />}
      currentPage={currentPage}
      sort={sort}
      perPage={SHOWS_PER_PAGE}
      noun={SHOW_NOUN}
      emptyIcon={Tv}
      onPageChange={newPage =>
        navigate({
          to: "/tv-shows",
          search: (prev: ShowsSearchParams) => ({
            ...prev,
            allPage: newPage,
          }),
          replace: true,
        })
      }
      onSortToggle={() =>
        navigate({
          to: "/tv-shows",
          search: (prev: ShowsSearchParams) => ({
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
      genresQueryOpts={showsGenresQueryOpts()}
      getGenres={data =>
        data.genres.map(g => ({
          genre_id: g.genre_id,
          genre_tag: g.genre_tag,
          count: g.show_count,
        }))
      }
      genresListLabel="TV show genres"
      itemsQueryOpts={showsByGenreQueryOpts(
        genreId ?? 0,
        genresPage,
        SHOWS_PER_PAGE,
        sort,
      )}
      getItems={data => data.shows}
      renderCard={show => <ShowCard show={show} />}
      genreId={genreId}
      genresPage={genresPage}
      sort={sort}
      perPage={SHOWS_PER_PAGE}
      noun={SHOW_NOUN}
      emptyIcon={Tv}
      fallbackFocusRef={fallbackFocusRef}
      onSelectGenre={id =>
        navigate({
          to: "/tv-shows",
          search: (prev: ShowsSearchParams) => ({
            ...prev,
            genreId: id,
            genresPage: 1,
          }),
          replace: true,
        })
      }
      onClearGenre={() =>
        navigate({
          to: "/tv-shows",
          search: (prev: ShowsSearchParams) => ({
            ...prev,
            genreId: undefined,
            genresPage: 1,
          }),
          replace: true,
        })
      }
      onPageChange={newPage =>
        navigate({
          to: "/tv-shows",
          search: (prev: ShowsSearchParams) => ({
            ...prev,
            genresPage: newPage,
          }),
          replace: true,
        })
      }
      onSortToggle={() =>
        navigate({
          to: "/tv-shows",
          search: (prev: ShowsSearchParams) => ({
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
