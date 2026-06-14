import { useCallback, useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import {
  ArrowLeft,
  Disc3,
  Heart,
  List,
  ListMusic,
  MoreHorizontal,
  Music,
  Play,
  Plus,
  Shuffle,
  User,
  Users,
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import { useAppShellScrollContainer } from "@/hooks/useAppShellScrollContainer";
import { useElementVirtualizer } from "@/hooks/useElementVirtualizer";
import { showActionFailed } from "@/lib/toast-helpers";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import { unwrapString, unwrapInt, unwrapStringOrUndefined } from "@/lib/nullable";
import {
  albumsPaginatedQueryOpts,
  likedTrackIdsQueryOpts,
  likedTracksQueryOpts,
  musiciansPaginatedQueryOpts,
  musicStatsQueryOpts,
  playlistsQueryOpts,
  spotifyStatusQueryOpts,
  tracksInfiniteQueryOpts,
} from "@/lib/query-opts";
import { convertToAudioTrack } from "@/lib/audio-utils";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useAudioPlayerState } from "@/hooks/useAudioPlayerState";
import {
  ALBUMS_PER_PAGE,
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MUSICIANS_PER_PAGE,
  VIRTUAL_LIST_LETTER_HEIGHT,
  VIRTUAL_LIST_TRACK_HEIGHT,
} from "@/lib/constants";
import {
  getOffsetWithinScrollContainer,
  observeElementRectWithWindowFallback,
} from "@/lib/scroll-container";
import { cn } from "@/lib/utils";

import AlbumCard from "@/components/AlbumCard";
import MusicianCard from "@/components/MusicianCard";
import LibraryPagination from "@/components/LibraryPagination";
import TrackItem from "@/components/TrackItem";
import PlaylistCard from "@/components/PlaylistCard";
import CreatePlaylistDialog from "@/components/CreatePlaylistDialog";
import RequestAlbumDialog from "@/components/RequestAlbumDialog";
import type { TrackListItemType, VirtualItem } from "@/types";
import {
  musicSearchSchema,
  type MusicSearchParams,
} from "@/types/route-search";

export const Route = createFileRoute("/_auth/music/")({
  validateSearch: musicSearchSchema,
  loaderDeps: ({ search: { albumsPage, musiciansPage } }) => ({
    albumsPage,
    musiciansPage,
  }),
  loader: async ({ context, deps: { albumsPage, musiciansPage } }) => {
    const { queryClient } = context;

    await Promise.all([
      queryClient.ensureQueryData(musicStatsQueryOpts()),
      queryClient.ensureQueryData(
        albumsPaginatedQueryOpts(albumsPage, ALBUMS_PER_PAGE)
      ),
      queryClient.ensureQueryData(
        musiciansPaginatedQueryOpts(musiciansPage, MUSICIANS_PER_PAGE)
      ),
    ]);
  },
  component: MusicPage,
});

function MusicPage() {
  const navigate = Route.useNavigate();
  const { tab, albumsPage, musiciansPage, playlistsView, likedTracksPage } = Route.useSearch();
  const { isExiting, runTransition, usesContentAnimation } =
    useContentFadeTransition(CONTENT_FADE_TRANSITION_MS);

  // React 19 document metadata
  const pageTitle = "Music Library - Igloo";
  const pageDescription = "Browse your collection of musicians, albums, tracks, and playlists in your Igloo media library.";
  let topLevelTabContent = (
    <AlbumsTabContent currentPage={albumsPage} perPage={ALBUMS_PER_PAGE} />
  );

  if (tab === "musicians") {
    topLevelTabContent = <MusiciansTabContent currentPage={musiciansPage} />;
  }

  if (tab === "tracks") {
    topLevelTabContent = <TracksTabContent />;
  }

  if (tab === "playlists") {
    topLevelTabContent = (
      <PlaylistsTabContent
        playlistsView={playlistsView}
        likedTracksPage={likedTracksPage}
      />
    );
  }

  // Handle tab change - update URL while preserving other params
  const handleTabChange = (newTab: string) => {
    const nextTab = newTab as MusicSearchParams["tab"];

    runTransition({
      shouldAnimate: nextTab !== tab,
      onTransition: () =>
        navigate({
          to: "/music",
          search: (prev: MusicSearchParams) => ({
            ...prev,
            tab: nextTab,
          }),
          replace: true,
        }),
    });
  };

  const topLevelTabPanelClassName = cn(
    usesContentAnimation &&
      (isExiting ? CONTENT_FADE_EXIT_CLASS : CONTENT_FADE_ENTER_CLASS),
  );

  return (
    <div className="min-w-0">
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      {/* Page header */}
      <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS)}>
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-white md:text-4xl">
          <Music className="size-6 shrink-0 text-amber-400" aria-hidden="true" />
          <span>Music Library</span>
        </h1>
        <p className="mt-1.5 max-w-2xl text-sm text-slate-400 md:text-base">
          Browse your collection of musicians, albums, and tracks
        </p>
      </header>

      {/* Stats + More dropdown */}
      <div
        className={cn(
          "mb-5 flex flex-wrap items-center justify-between gap-3",
          MOTION_SECTION_ENTER_DELAYED_CLASS,
        )}
      >
        <LibraryStats />
        <MoreMenu />
      </div>

      {/* Tabs - controlled by URL search param */}
      <Tabs
        value={tab}
        onValueChange={handleTabChange}
        className={MOTION_SECTION_ENTER_DELAYED_CLASS}
      >
        <TabsList className="grid! h-auto w-full max-w-full grid-cols-2 gap-1 border border-slate-700/50 bg-slate-800/50 p-1 sm:w-fit sm:max-w-none sm:grid-cols-4">
          <TabsTrigger
            value="musicians"
            className="min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
          >
            <Users
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Musicians
          </TabsTrigger>
          <TabsTrigger
            value="albums"
            className="min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
          >
            <Disc3
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Albums
          </TabsTrigger>
          <TabsTrigger
            value="tracks"
            className="min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
          >
            <List
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Tracks
          </TabsTrigger>
          <TabsTrigger
            value="playlists"
            className="min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
          >
            <ListMusic
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

function LibraryStats() {
  const { data } = useQuery(musicStatsQueryOpts());
  const stats = data?.error === false ? data.data : null;

  const albumCount = stats?.total_albums ?? 0;
  const trackCount = stats?.total_tracks ?? 0;
  const musicianCount = stats?.total_musicians ?? 0;

  const statsLabel = `Library statistics: ${albumCount} albums, ${trackCount} tracks, ${musicianCount} musicians`;

  return (
    <section
      className={cn(
        "flex flex-wrap gap-x-6 gap-y-3",
        MOTION_SECTION_ENTER_DELAYED_CLASS,
      )}
      aria-label={statsLabel}
    >
      <div className="flex items-center gap-2" aria-hidden="true">
        <Disc3 className="size-4 text-amber-400" />
        <span className="font-medium text-white">{albumCount}</span>
        <span className="text-slate-400">Albums</span>
      </div>
      <div className="flex items-center gap-2" aria-hidden="true">
        <Music className="size-4 text-amber-400" />
        <span className="font-medium text-white">{trackCount}</span>
        <span className="text-slate-400">Tracks</span>
      </div>
      <div className="flex items-center gap-2" aria-hidden="true">
        <User className="size-4 text-amber-400" />
        <span className="font-medium text-white">{musicianCount}</span>
        <span className="text-slate-400">Musicians</span>
      </div>
    </section>
  );
}

function MoreMenu() {
  const moreOptionsButtonRef = useRef<HTMLButtonElement | null>(null);
  const [requestAlbumOpen, setRequestAlbumOpen] = useState(false);
  const { data: spotifyStatusData, isLoading: spotifyStatusLoading } = useQuery(
    spotifyStatusQueryOpts(),
  );
  const spotifyAvailable =
    spotifyStatusData?.error === false
      ? spotifyStatusData.data.available
      : false;
  const requestAlbumDisabled = spotifyStatusLoading || !spotifyAvailable;
  const requestAlbumDescription = spotifyStatusLoading
    ? "Spotify search status is still loading."
    : "Spotify search is unavailable on this server.";

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
            disabled={requestAlbumDisabled}
            aria-label={
              requestAlbumDisabled
                ? `Request Album unavailable. ${requestAlbumDescription}`
                : "Request Album"
            }
            title={requestAlbumDisabled ? requestAlbumDescription : undefined}
            onSelect={event => {
              if (requestAlbumDisabled) {
                event.preventDefault();
                return;
              }
              setRequestAlbumOpen(true);
            }}
          >
            <Plus className="mr-2 size-4" aria-hidden="true" />
            Request Album
            {requestAlbumDisabled && (
              <span className="sr-only"> {requestAlbumDescription}</span>
            )}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      {requestAlbumOpen && (
        <RequestAlbumDialog
          open={requestAlbumOpen}
          onOpenChange={setRequestAlbumOpen}
          restoreFocusRef={moreOptionsButtonRef}
        />
      )}
    </>
  );
}

type MusiciansTabContentProps = {
  currentPage: number;
};

// Skeleton loader that matches grid layout to prevent CLS
function MusiciansTabSkeleton() {
  return (
    <div>
      {/* Skeleton header */}
      <div className="mb-6 flex items-center justify-between">
        <div className={cn("h-4 w-24 rounded-sm bg-slate-800", MOTION_LOADING_STATE_CLASS)} />
        <div className={cn("h-4 w-20 rounded-sm bg-slate-800", MOTION_LOADING_STATE_CLASS)} />
      </div>

      {/* Skeleton grid - matches actual grid dimensions */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: MUSICIANS_PER_PAGE }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "rounded-xl border border-slate-800 bg-slate-900 p-4",
              MOTION_LOADING_STATE_CLASS,
            )}
          >
            <div className="mx-auto mb-3 aspect-square w-full max-w-32 rounded-full bg-slate-800" />
            <div className="mx-auto h-4 w-3/4 rounded-sm bg-slate-800" />
            <div className="mx-auto mt-2 h-3 w-1/2 rounded-sm bg-slate-800" />
          </div>
        ))}
      </div>
    </div>
  );
}

function MusiciansTabContent({ currentPage }: MusiciansTabContentProps) {
  const navigate = Route.useNavigate();

  const { data, isLoading } = useQuery(
    musiciansPaginatedQueryOpts(currentPage, MUSICIANS_PER_PAGE),
  );

  const musicians = data?.error === false ? data.data.musicians : [];
  const totalPages = data?.error === false ? data.data.total_pages : 0;
  const hasMultiplePages = totalPages > 1;

  // Generate announcement for screen readers
  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (musicians.length === 0) return "No musicians found";
    return `Showing ${musicians.length} musicians, page ${currentPage} of ${totalPages}`;
  };

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        musiciansPage: newPage,
      }),
      replace: true,
    });

    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  if (isLoading) {
    return <MusiciansTabSkeleton />;
  }

  if (musicians.length === 0) {
    return (
      <div className="py-12 text-center text-slate-400">
        <Users className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
        <p>No musicians found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Announce content changes to screen readers */}
      <LiveAnnouncer message={getAnnouncement()} />
      {hasMultiplePages && (
        <div className="mb-5 flex justify-end">
          <span className="text-sm text-slate-400">
            Page {currentPage} of {totalPages}
          </span>
        </div>
      )}

      {/* Musicians grid - 5 columns on large screens for circular thumbnails */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {musicians.map(musician => (
          <MusicianCard key={musician.id} musician={musician} />
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

type AlbumsTabContentProps = {
  currentPage: number;
  perPage: number;
};

function AlbumsTabContent({ currentPage, perPage }: AlbumsTabContentProps) {
  const navigate = Route.useNavigate();

  const { data, isLoading } = useQuery(
    albumsPaginatedQueryOpts(currentPage, perPage),
  );

  const albums = data?.error === false ? data.data.albums : [];
  const totalPages = data?.error === false ? data.data.total_pages : 0;
  const hasMultiplePages = totalPages > 1;

  // Generate announcement for screen readers
  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (albums.length === 0) return "No albums found";
    return `Showing ${albums.length} albums, page ${currentPage} of ${totalPages}`;
  };

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        albumsPage: newPage,
      }),
      replace: true,
    });

    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12" role="status" aria-label="Loading albums">
        <Spinner className="size-8 text-amber-400" />
        <span className="sr-only">Loading albums...</span>
      </div>
    );
  }

  if (albums.length === 0) {
    return (
      <div className="py-12 text-center text-slate-400">
        <Disc3 className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
        <p>No albums found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Announce content changes to screen readers */}
      <LiveAnnouncer message={getAnnouncement()} />
      {hasMultiplePages && (
        <div className="mb-5 flex justify-end">
          <span className="text-sm text-slate-400">
            Page {currentPage} of {totalPages}
          </span>
        </div>
      )}

      {/* Albums grid */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
        {albums.map(album => (
          <AlbumCard key={album.id} album={album} />
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

function TracksTabContent() {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } =
    useInfiniteQuery(tracksInfiniteQueryOpts());

  const { data: likedIdsData } = useQuery(likedTrackIdsQueryOpts());
  const likedSet = new Set<number>(
    likedIdsData?.error === false ? (likedIdsData.data.liked_track_ids ?? []) : [],
  );

  const isFetchingNextRef = useRef(false);

  // Get total tracks count from first page
  const totalTracks =
    data?.pages[0]?.error === false ? (data.pages[0].data?.total ?? 0) : 0;

  // Flatten all pages into a single array
  const allTracks =
    data?.pages.flatMap(page =>
      page.error === false ? (page.data?.tracks ?? []) : [],
    ) ?? [];

  // Convert to virtual items (tracks + letter headers)
  const virtualItems = flattenToVirtualItems(allTracks);

  useEffect(() => {
    if (!isFetchingNextPage) {
      isFetchingNextRef.current = false;
    }
  }, [isFetchingNextPage]);

  const requestNextPage = useCallback(() => {
    if (isFetchingNextRef.current || isFetchingNextPage || !hasNextPage) {
      return;
    }

    isFetchingNextRef.current = true;
    void fetchNextPage().finally(() => {
      isFetchingNextRef.current = false;
    });
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

  // Generate announcement for screen readers
  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (allTracks.length === 0) return "No tracks found";
    if (isFetchingNextPage) return undefined;
    return `${allTracks.length} of ${totalTracks} tracks loaded`;
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12" role="status" aria-label="Loading tracks">
        <Spinner className="size-8 text-amber-400" />
        <span className="sr-only">Loading tracks...</span>
      </div>
    );
  }

  if (allTracks.length === 0) {
    return (
      <div className="py-12 text-center text-slate-400">
        <Music className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
        <p>No tracks found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Announce content changes to screen readers */}
      <LiveAnnouncer message={getAnnouncement()} />

      {/* Header with play/shuffle buttons */}
      <div className="mb-4 flex justify-end">
        <div className="flex flex-wrap justify-end gap-2">
          <PlayAllButton />
          <ShuffleButton />
        </div>
      </div>

      <VirtualizedTracksList
        virtualItems={virtualItems}
        likedSet={likedSet}
        totalTracks={totalTracks}
        hasNextPage={hasNextPage}
        isFetchingNextPage={isFetchingNextPage}
        requestNextPage={requestNextPage}
      />
    </div>
  );
}

type VirtualizedTracksListProps = {
  virtualItems: VirtualItem[];
  likedSet: Set<number>;
  totalTracks: number;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  requestNextPage: () => void;
};

function VirtualizedTracksList({
  virtualItems,
  likedSet,
  totalTracks,
  hasNextPage,
  isFetchingNextPage,
  requestNextPage,
}: VirtualizedTracksListProps) {
  "use no memo";

  const scrollContainer = useAppShellScrollContainer();
  const listRef = useRef<HTMLDivElement>(null);
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const [scrollMargin, setScrollMargin] = useState(0);

  useEffect(() => {
    const listElement = listRef.current;
    if (!listElement || !scrollContainer) {
      return;
    }

    const updateScrollMargin = () => {
      setScrollMargin(
        getOffsetWithinScrollContainer(listElement, scrollContainer),
      );
    };

    updateScrollMargin();

    const resizeObserver =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(updateScrollMargin);

    resizeObserver?.observe(listElement);
    resizeObserver?.observe(scrollContainer);
    window.addEventListener("resize", updateScrollMargin);

    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateScrollMargin);
    };
  }, [scrollContainer]);

  const virtualizer = useElementVirtualizer({
    count: virtualItems.length,
    getScrollElement: () => scrollContainer,
    initialRect: {
      width: scrollContainer?.clientWidth ?? window.innerWidth,
      height: scrollContainer?.clientHeight ?? window.innerHeight,
    },
    observeElementRect: observeElementRectWithWindowFallback,

    estimateSize: index => {
      const item = virtualItems[index];

      return item?.type === "letter"
        ? VIRTUAL_LIST_LETTER_HEIGHT
        : VIRTUAL_LIST_TRACK_HEIGHT;
    },

    overscan: 5,
    scrollMargin,
  });

  const renderedVirtualItems = virtualizer.getVirtualItems();

  useEffect(() => {
    virtualizer.measure();
  }, [scrollMargin, virtualizer, virtualItems.length]);

  useEffect(() => {
    const target = loadMoreRef.current;
    if (
      !target ||
      !scrollContainer ||
      !hasNextPage ||
      isFetchingNextPage ||
      typeof IntersectionObserver === "undefined"
    ) {
      return;
    }

    const observer = new IntersectionObserver(
      entries => {
        if (entries.some(entry => entry.isIntersecting)) {
          requestNextPage();
        }
      },
      {
        root: scrollContainer,
        rootMargin: "800px 0px",
      },
    );

    observer.observe(target);

    return () => observer.disconnect();
  }, [
    hasNextPage,
    isFetchingNextPage,
    requestNextPage,
    scrollContainer,
    virtualItems.length,
  ]);

  useEffect(() => {
    if (renderedVirtualItems.length === 0) return;

    const lastItem = renderedVirtualItems[renderedVirtualItems.length - 1];

    if (
      lastItem &&
      lastItem.index >= virtualItems.length - 10 &&
      hasNextPage &&
      !isFetchingNextPage
    ) {
      requestNextPage();
    }
  }, [
    renderedVirtualItems,
    virtualItems.length,
    hasNextPage,
    isFetchingNextPage,
    requestNextPage,
  ]);

  return (
    <div
      ref={listRef}
      className="overflow-hidden rounded-lg border border-slate-800 bg-slate-900/50"
      role="list"
      aria-label="Tracks"
    >
      <div
        style={{
          height: `${virtualizer.getTotalSize()}px`,
          width: "100%",
          position: "relative",
        }}
      >
        {renderedVirtualItems.map(virtualRow => {
          const item = virtualItems[virtualRow.index];

          if (!item) return null;

          return (
            <div
              key={virtualRow.key}
              role={item.type === "track" ? "listitem" : undefined}
              aria-posinset={item.type === "track" ? item.trackIndex : undefined}
              aria-setsize={item.type === "track" ? totalTracks : undefined}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                height: `${virtualRow.size}px`,
                transform: `translateY(${virtualRow.start - scrollMargin}px)`,
              }}
            >
              {item.type === "letter" ? (
                <LetterHeader letter={item.letter} />
              ) : (
                <TrackListItem track={item.track} isLiked={likedSet.has(item.track.id)} />
              )}
            </div>
          );
        })}

        <div
          ref={loadMoreRef}
          aria-hidden="true"
          style={{
            position: "absolute",
            left: 0,
            top: `${virtualizer.getTotalSize() - 1}px`,
            width: "100%",
            height: "1px",
          }}
        />
      </div>

      {isFetchingNextPage && (
        <div className="flex justify-center py-4">
          <Spinner className="size-6 text-amber-400" />
        </div>
      )}
    </div>
  );
}

function PlayAllButton() {
  const [isLoading, setIsLoading] = useState(false);
  const audioPlayer = useAudioPlayerActions();

  const handlePlayAll = async () => {
    setIsLoading(true);

    try {
      await audioPlayer.startPlayAllPlayback();
    } catch (error) {
      console.error("Failed to start playback:", error);
      showActionFailed("start playback", "Unable to start playing all tracks. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handlePlayAll}
      disabled={isLoading}
      className="inline-flex min-h-10 items-center gap-2 rounded-full bg-slate-700 px-3 py-2 font-medium text-white transition-colors hover:bg-slate-600 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:opacity-50 sm:px-4"
      aria-label="Play all tracks"
    >
      {isLoading ? (
        <Spinner className="size-4" />
      ) : (
        <Play className="size-4 fill-current" aria-hidden="true" />
      )}
      <span>Play all</span>
    </button>
  );
}

function ShuffleButton() {
  const [isLoading, setIsLoading] = useState(false);
  const audioPlayer = useAudioPlayerActions();

  const handleShuffle = async () => {
    setIsLoading(true);

    try {
      await audioPlayer.startShufflePlayback();
    } catch (error) {
      console.error("Failed to start shuffle playback:", error);
      showActionFailed("start shuffle", "Unable to start shuffle playback. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handleShuffle}
      disabled={isLoading}
      className="inline-flex min-h-10 items-center gap-2 rounded-full bg-amber-500 px-3 py-2 font-medium text-slate-900 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:opacity-50 sm:px-4"
      aria-label="Shuffle all tracks"
    >
      {isLoading ? (
        <Spinner className="size-4" />
      ) : (
        <Shuffle className="size-4" aria-hidden="true" />
      )}
      <span>Shuffle all</span>
    </button>
  );
}

function LetterHeader({ letter }: { letter: string }) {
  return (
    <div
      className="border-b border-amber-500/20 bg-slate-800/50 px-4 py-3"
      role="heading"
      aria-level={3}
      aria-label={`Tracks starting with ${letter}`}
    >
      <span className="text-2xl font-bold text-amber-400">{letter}</span>
    </div>
  );
}

function TrackListItem({ track, isLiked }: { track: TrackListItemType; isLiked: boolean }) {
  const audioPlayer = useAudioPlayerActions();
  const playerState = useAudioPlayerState();

  const handlePlay = () => {
    const audioTrack = convertToAudioTrack({
      id: track.id,
      title: track.title,
      file_path: track.file_path,
      duration: track.duration,
      codec: track.codec,
      bit_rate: track.bit_rate,
      album_id: track.album_id,
      musician_id: track.musician_id,
      album_cover: track.album_cover,
      musician_name: track.musician_name,
    });

    audioPlayer.playTrack(audioTrack, [audioTrack], {
      cover: unwrapString(track.album_cover),
      title: unwrapString(track.album_title) ?? "Unknown Album",
      musician: unwrapString(track.musician_name),
    });
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
      isPlaying={playerState.currentTrack?.id === track.id && playerState.isPlaying}
      isCurrentTrack={playerState.currentTrack?.id === track.id}
      isLiked={isLiked}
      onPlay={handlePlay}
      showActionsMenu
    />
  );
}

// Flatten tracks into virtual items with letter headers inserted
function flattenToVirtualItems(tracks: TrackListItemType[]): VirtualItem[] {
  const items: VirtualItem[] = [];
  let currentLetter: string | null = null;

  tracks.forEach((track, index) => {
    const firstChar = track.title.charAt(0).toUpperCase();
    const letter = /[A-Z]/.test(firstChar) ? firstChar : "#";

    // Insert letter header when we encounter a new letter
    if (letter !== currentLetter) {
      items.push({ type: "letter", letter });
      currentLetter = letter;
    }

    items.push({ type: "track", track, trackIndex: index + 1 });
  });

  return items;
}

type PlaylistsTabContentProps = {
  playlistsView: "playlists" | "liked";
  likedTracksPage: number;
};

// Playlists tab content
function PlaylistsTabContent({ playlistsView, likedTracksPage }: PlaylistsTabContentProps) {
  const navigate = Route.useNavigate();
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const createPlaylistRestoreRef = useRef<HTMLButtonElement | null>(null);
  const { data, isLoading } = useQuery({
    ...playlistsQueryOpts(),
    enabled: playlistsView !== "liked",
  });

  const playlists = data?.error === false ? data.data.playlists : [];

  const handleShowLiked = () =>
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        playlistsView: "liked",
        likedTracksPage: 1,
      }),
      replace: true,
    });

  const handleExitLiked = () =>
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        playlistsView: "playlists",
        likedTracksPage: 1,
      }),
      replace: true,
    });

  const handleCreateOpen = () => {
    setShowCreateDialog(true);
  };

  if (playlistsView === "liked") {
    return (
      <LikedTracksInPlaylistsTab
        likedTracksPage={likedTracksPage}
        onExit={handleExitLiked}
      />
    );
  }

  // Generate announcement for screen readers
  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (playlists.length === 0) return "No playlists yet";
    return `${playlists.length} playlist${playlists.length !== 1 ? "s" : ""} loaded`;
  };

  if (isLoading) {
    return <PlaylistsTabSkeleton />;
  }

  return (
    <div>
      {/* Announce content changes to screen readers */}
      <LiveAnnouncer message={getAnnouncement()} />
      {/* Header with count and create button */}
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span className="text-sm text-slate-400">
          {playlists.length} {playlists.length === 1 ? "playlist" : "playlists"}
        </span>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={handleShowLiked}
            className="inline-flex min-h-10 items-center gap-2 rounded-full border border-slate-600 px-3 py-2 text-sm font-medium text-slate-300 transition-colors hover:border-amber-500/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none sm:px-4"
            aria-label="View liked tracks"
          >
            <Heart className="size-4 shrink-0" aria-hidden="true" />
            Liked tracks
          </button>
          <button
            type="button"
            ref={createPlaylistRestoreRef}
            onClick={handleCreateOpen}
            className="inline-flex min-h-10 items-center gap-2 rounded-full bg-amber-500 px-3 py-2 text-sm font-medium text-slate-900 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none sm:px-4"
            aria-label="Create new playlist"
          >
            <Plus className="size-4 shrink-0" aria-hidden="true" />
            New playlist
          </button>
        </div>
      </div>

      {/* Playlists grid or empty state */}
      {playlists.length === 0 ? (
        <EmptyPlaylistsState onCreateClick={handleCreateOpen} />
      ) : (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
          {playlists.map((playlist) => (
            <PlaylistCard key={playlist.id} playlist={playlist} />
          ))}
        </div>
      )}

      <CreatePlaylistDialog
        open={showCreateDialog}
        onOpenChange={setShowCreateDialog}
        restoreFocusRef={createPlaylistRestoreRef}
      />
    </div>
  );
}

type LikedTracksInPlaylistsTabProps = {
  likedTracksPage: number;
  onExit: () => void;
};

function LikedTracksInPlaylistsTab({ likedTracksPage, onExit }: LikedTracksInPlaylistsTabProps) {
  const navigate = Route.useNavigate();
  const audioPlayer = useAudioPlayerActions();
  const audioPlayerState = useAudioPlayerState();

  const { data, isLoading } = useQuery(likedTracksQueryOpts(likedTracksPage));

  const tracks = data?.error === false ? data.data.tracks : [];
  const total = data?.error === false ? data.data.total : 0;
  const totalPages = data?.error === false ? data.data.total_pages : 0;

  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (tracks.length === 0) return "No liked tracks";
    return `${total} liked track${total !== 1 ? "s" : ""}, page ${likedTracksPage} of ${totalPages}`;
  };

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        likedTracksPage: newPage,
      }),
      replace: true,
    });
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const handlePlayTrack = (track: TrackListItemType) => {
    const audioTrack = convertToAudioTrack({
      id: track.id,
      title: track.title,
      file_path: track.file_path,
      duration: track.duration,
      codec: track.codec,
      bit_rate: track.bit_rate,
      album_id: track.album_id,
      musician_id: track.musician_id,
      album_cover: track.album_cover,
      musician_name: track.musician_name,
    });
    const allAudioTracks = tracks.map((t) =>
      convertToAudioTrack({
        id: t.id,
        title: t.title,
        file_path: t.file_path,
        duration: t.duration,
        codec: t.codec,
        bit_rate: t.bit_rate,
        album_id: t.album_id,
        musician_id: t.musician_id,
        album_cover: t.album_cover,
        musician_name: t.musician_name,
      })
    );
    audioPlayer.playTrack(audioTrack, allAudioTracks, {
      cover: null,
      title: "Liked Tracks",
      musician: null,
    });
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12" role="status" aria-label="Loading liked tracks">
        <Spinner className="size-8 text-amber-400" />
        <span className="sr-only">Loading liked tracks...</span>
      </div>
    );
  }

  return (
    <div>
      <LiveAnnouncer message={getAnnouncement()} />

      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={onExit}
            className="flex items-center gap-2 text-sm text-slate-400 transition-colors hover:text-white focus:text-amber-400 focus:ring-2 focus:ring-amber-400 focus:outline-none"
            aria-label="Back to playlists"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            Playlists
          </button>
          <span className="text-slate-600" aria-hidden="true">/</span>
          <h2 className="flex items-center gap-2 font-semibold text-white">
            <Heart className="size-4 fill-current text-red-400" aria-hidden="true" />
            Liked Tracks
          </h2>
        </div>
        <span className="text-sm text-slate-400">
          {total} {total === 1 ? "track" : "tracks"}
        </span>
      </div>

      {/* Track list or empty state */}
      {tracks.length === 0 ? (
        <div className="rounded-xl border border-amber-500/10 bg-slate-800/30 py-12 text-center">
          <div className="mx-auto mb-4 flex size-16 items-center justify-center rounded-full bg-linear-to-br from-slate-700 via-slate-800 to-cyan-900/40">
            <Heart className="size-6 text-slate-500" aria-hidden="true" />
          </div>
          <p className="text-slate-300">No liked tracks yet.</p>
          <p className="mt-2 text-sm text-slate-400">
            Tap the heart icon on any track to add it here.
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-xl border border-amber-500/10 bg-slate-800/30">
          {tracks.map((track) => (
            <TrackItem
              key={track.id}
              id={track.id}
              title={track.title}
              duration={track.duration}
              subtitle={unwrapString(track.musician_name) ?? "Unknown Artist"}
              albumId={unwrapInt(track.album_id)}
              albumTitle={unwrapStringOrUndefined(track.album_title)}
              musicianId={unwrapInt(track.musician_id)}
              musicianName={unwrapStringOrUndefined(track.musician_name)}
              variant="library"
              isLiked
              isPlaying={
                audioPlayerState.currentTrack?.id === track.id &&
                audioPlayerState.isPlaying
              }
              isCurrentTrack={audioPlayerState.currentTrack?.id === track.id}
              onPlay={() => handlePlayTrack(track)}
              showActionsMenu
            />
          ))}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="mt-6">
          <LibraryPagination
            currentPage={likedTracksPage}
            totalPages={totalPages}
            onPageChange={handlePageChange}
          />
        </div>
      )}
    </div>
  );
}

function PlaylistsTabSkeleton() {
  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div className={cn("h-4 w-24 rounded-sm bg-slate-800", MOTION_LOADING_STATE_CLASS)} />
        <div className={cn("h-10 w-32 rounded-full bg-slate-800", MOTION_LOADING_STATE_CLASS)} />
      </div>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: 10 }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "rounded-xl border border-slate-800 bg-slate-900 p-4",
              MOTION_LOADING_STATE_CLASS,
            )}
          >
            <div className="mx-auto mb-3 aspect-square w-full rounded-lg bg-slate-800" />
            <div className="mx-auto h-4 w-3/4 rounded-sm bg-slate-800" />
            <div className="mx-auto mt-2 h-3 w-1/2 rounded-sm bg-slate-800" />
          </div>
        ))}
      </div>
    </div>
  );
}

type EmptyPlaylistsStateProps = {
  onCreateClick: () => void;
};

function EmptyPlaylistsState({ onCreateClick }: EmptyPlaylistsStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center sm:py-16">
      <div className="mb-5 flex size-20 items-center justify-center rounded-full bg-linear-to-br from-slate-700 via-slate-800 to-amber-900/30 shadow-lg shadow-amber-500/5 sm:size-24">
        <ListMusic
          className="size-8 text-amber-200/40 sm:size-10"
          aria-hidden="true"
        />
      </div>
      <h3 className="mb-2 text-xl font-semibold text-white">
        No playlists yet
      </h3>
      <p className="mb-5 max-w-sm text-slate-400 sm:mb-6">
        Create your first playlist to start organizing your favorite tracks.
      </p>
      <button
        onClick={onCreateClick}
        className="inline-flex min-h-11 items-center gap-2 rounded-full bg-amber-500 px-5 py-2.5 font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none sm:px-6 sm:py-3"
      >
        <Plus className="size-4" aria-hidden="true" />
        Create your first playlist
      </button>
    </div>
  );
}
