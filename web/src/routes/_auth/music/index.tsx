import { memo, useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
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
  RefreshCw,
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
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { useVirtualizedInfiniteLoader } from "@/hooks/useVirtualizedInfiniteLoader";
import { useWindowScrollMargin } from "@/hooks/useWindowScrollMargin";
import { showActionFailed, showSuccess } from "@/lib/toast-helpers";
import { refreshMusicLibraryCache } from "@/lib/music-library-cache";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { isApiFailure } from "@/lib/is-api-failure";
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
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MUSICIANS_PER_PAGE,
  TRACK_LIST_CONTAINER_CLASS,
  VIRTUAL_LIST_LETTER_HEIGHT,
  VIRTUAL_LIST_TRACK_HEIGHT,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

import AlbumCard from "@/components/music/AlbumCard";
import MusicianCard from "@/components/music/MusicianCard";
import LibraryPagination from "@/components/shared/LibraryPagination";
import TrackItem from "@/components/music/TrackItem";
import PlaylistCard from "@/components/music/PlaylistCard";
import EmptyState from "@/components/shared/EmptyState";
import { Button } from "@/components/ui/button";
import CreatePlaylistDialog from "@/components/music/CreatePlaylistDialog";
import RequestAlbumDialog from "@/components/music/RequestAlbumDialog";
import RequestTrackDialog from "@/components/music/RequestTrackDialog";
import type { TrackListItemType, VirtualItem } from "@/types";
import {
  musicSearchSchema,
  type MusicSearchParams,
} from "@/lib/route-search";

const MUSIC_PAGE_TITLE = "Music Library - Igloo";
const MUSIC_PAGE_DESCRIPTION =
  "Browse your collection of musicians, albums, tracks, and playlists in your Igloo media library.";

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
      <title>{MUSIC_PAGE_TITLE}</title>
      <meta name="description" content={MUSIC_PAGE_DESCRIPTION} />

      {/* Page header */}
      <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS)}>
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
          <Music className="size-6 shrink-0 text-primary" aria-hidden="true" />
          <span>Music Library</span>
        </h1>
        <p className="mt-1.5 max-w-2xl text-sm text-muted-foreground md:text-base">
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
        <TabsList
          className={cn(LIBRARY_TABS_LIST_CLASS, "grid-cols-2 sm:grid-cols-4")}
        >
          <TabsTrigger value="musicians" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Users
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Musicians
          </TabsTrigger>
          <TabsTrigger value="albums" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <Disc3
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Albums
          </TabsTrigger>
          <TabsTrigger value="tracks" className={LIBRARY_TAB_TRIGGER_CLASS}>
            <List
              className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
              aria-hidden="true"
            />
            Tracks
          </TabsTrigger>
          <TabsTrigger value="playlists" className={LIBRARY_TAB_TRIGGER_CLASS}>
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
        <Disc3 className="size-4 text-primary" />
        <span className="font-medium text-foreground">{albumCount}</span>
        <span className="text-muted-foreground">Albums</span>
      </div>
      <div className="flex items-center gap-2" aria-hidden="true">
        <Music className="size-4 text-primary" />
        <span className="font-medium text-foreground">{trackCount}</span>
        <span className="text-muted-foreground">Tracks</span>
      </div>
      <div className="flex items-center gap-2" aria-hidden="true">
        <User className="size-4 text-primary" />
        <span className="font-medium text-foreground">{musicianCount}</span>
        <span className="text-muted-foreground">Musicians</span>
      </div>
    </section>
  );
}

function MoreMenu() {
  const queryClient = useQueryClient();
  const moreOptionsButtonRef = useRef<HTMLButtonElement | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [refreshingLibrary, setRefreshingLibrary] = useState(false);
  const [requestAlbumOpen, setRequestAlbumOpen] = useState(false);
  const [requestTrackOpen, setRequestTrackOpen] = useState(false);
  const { data: spotifyStatusData, isLoading: spotifyStatusLoading } = useQuery(
    spotifyStatusQueryOpts(),
  );

  const handleRefreshLibrary = async () => {
    if (refreshingLibrary) return;

    setRefreshingLibrary(true);
    try {
      await refreshMusicLibraryCache(queryClient);
      showSuccess("Library refreshed", "Music library data is up to date.");
    } catch (error) {
      console.error("Failed to refresh music library:", error);
      showActionFailed(
        "refresh library",
        "Unable to refresh the music library. Please try again.",
      );
    } finally {
      setRefreshingLibrary(false);
      setMenuOpen(false);
    }
  };
  const spotifyAvailable =
    spotifyStatusData?.error === false
      ? spotifyStatusData.data.available
      : false;
  const spotifyRequestDisabled = spotifyStatusLoading || !spotifyAvailable;
  const spotifyRequestDescription = spotifyStatusLoading
    ? "Spotify search status is still loading."
    : "Spotify search is unavailable on this server.";

  return (
    <>
      <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
        <DropdownMenuTrigger
          ref={moreOptionsButtonRef}
          className={cn(
            "inline-flex items-center justify-center rounded-full p-2 text-muted-foreground hover:bg-accent hover:text-foreground",
            MOTION_MICRO_CONTROL_CLASS,
            FOCUS_VISIBLE_RING_CLASS,
          )}
          aria-label="More options"
        >
          <MoreHorizontal className="size-5" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          className="border-border bg-muted"
        >
          <DropdownMenuItem
            className="cursor-pointer text-foreground focus:bg-accent focus:text-foreground"
            disabled={refreshingLibrary}
            onSelect={event => {
              // Keep the menu open while the async refresh runs so the
              // spinner/disabled state stays perceivable; it closes when the
              // refresh settles (see handleRefreshLibrary).
              event.preventDefault();
              if (refreshingLibrary) return;
              void handleRefreshLibrary();
            }}
          >
            {refreshingLibrary ? (
              <Spinner className="mr-2 size-4 text-primary" />
            ) : (
              <RefreshCw className="mr-2 size-4" aria-hidden="true" />
            )}
            Refresh Library
          </DropdownMenuItem>
          <DropdownMenuItem
            className="cursor-pointer text-foreground focus:bg-accent focus:text-foreground"
            disabled={spotifyRequestDisabled}
            aria-label={
              spotifyRequestDisabled
                ? `Request Album unavailable. ${spotifyRequestDescription}`
                : "Request Album"
            }
            title={spotifyRequestDisabled ? spotifyRequestDescription : undefined}
            onSelect={event => {
              if (spotifyRequestDisabled) {
                event.preventDefault();
                return;
              }
              setRequestAlbumOpen(true);
            }}
          >
            <Plus className="mr-2 size-4" aria-hidden="true" />
            Request Album
            {spotifyRequestDisabled && (
              <span className="sr-only"> {spotifyRequestDescription}</span>
            )}
          </DropdownMenuItem>
          <DropdownMenuItem
            className="cursor-pointer text-foreground focus:bg-accent focus:text-foreground"
            disabled={spotifyRequestDisabled}
            aria-label={
              spotifyRequestDisabled
                ? `Request Track unavailable. ${spotifyRequestDescription}`
                : "Request Track"
            }
            title={spotifyRequestDisabled ? spotifyRequestDescription : undefined}
            onSelect={event => {
              if (spotifyRequestDisabled) {
                event.preventDefault();
                return;
              }
              setRequestTrackOpen(true);
            }}
          >
            <Plus className="mr-2 size-4" aria-hidden="true" />
            Request Track
            {spotifyRequestDisabled && (
              <span className="sr-only"> {spotifyRequestDescription}</span>
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

      {requestTrackOpen && (
        <RequestTrackDialog
          open={requestTrackOpen}
          onOpenChange={setRequestTrackOpen}
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
      {/* Skeleton grid - matches actual grid dimensions */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: MUSICIANS_PER_PAGE }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "rounded-xl border border-border bg-card p-4",
              MOTION_LOADING_STATE_CLASS,
            )}
          >
            <div className="mx-auto mb-3 aspect-square w-full max-w-32 rounded-full bg-muted" />
            <div className="mx-auto h-4 w-3/4 rounded-sm bg-muted" />
            <div className="mx-auto mt-2 h-3 w-1/2 rounded-sm bg-muted" />
          </div>
        ))}
      </div>
    </div>
  );
}

function MusiciansTabContent({ currentPage }: MusiciansTabContentProps) {
  const navigate = Route.useNavigate();

  const { data, isLoading, isError, refetch } = useQuery(
    musiciansPaginatedQueryOpts(currentPage, MUSICIANS_PER_PAGE),
  );

  const musicians = data?.error === false ? data.data.musicians : [];
  const totalPages = data?.error === false ? data.data.total_pages : 0;
  const hasMultiplePages = totalPages > 1;

  // Generate announcement for screen readers
  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (musicians.length === 0) return "No musicians found";
    return `Showing ${musicians.length} musician${musicians.length === 1 ? "" : "s"}, page ${currentPage} of ${totalPages}`;
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

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t load musicians. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  if (musicians.length === 0) {
    return (
      <div className="py-12 text-center text-muted-foreground">
        <LiveAnnouncer message={getAnnouncement()} />
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
          <span className="text-sm text-muted-foreground">
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

// Skeleton loader that matches the albums grid layout to prevent CLS
function AlbumsTabSkeleton() {
  return (
    <div>
      {/* Skeleton grid - matches the real albums grid dimensions */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
        {Array.from({ length: ALBUMS_PER_PAGE }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "overflow-hidden rounded-xl border border-border bg-card",
              MOTION_LOADING_STATE_CLASS,
            )}
          >
            <div className="aspect-square bg-muted" />
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

function AlbumsTabContent({ currentPage, perPage }: AlbumsTabContentProps) {
  const navigate = Route.useNavigate();

  const { data, isLoading, isError, refetch } = useQuery(
    albumsPaginatedQueryOpts(currentPage, perPage),
  );

  const albums = data?.error === false ? data.data.albums : [];
  const totalPages = data?.error === false ? data.data.total_pages : 0;
  const hasMultiplePages = totalPages > 1;

  // Generate announcement for screen readers
  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (albums.length === 0) return "No albums found";
    return `Showing ${albums.length} album${albums.length === 1 ? "" : "s"}, page ${currentPage} of ${totalPages}`;
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
    return <AlbumsTabSkeleton />;
  }

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t load albums. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  if (albums.length === 0) {
    return (
      <div className="py-12 text-center text-muted-foreground">
        <LiveAnnouncer message={getAnnouncement()} />
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
          <span className="text-sm text-muted-foreground">
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

// Skeleton loader that matches the library track-list layout to prevent CLS.
// Shared by the Tracks tab and the Liked Tracks view.
function TracksListSkeleton() {
  return (
    <div className={TRACK_LIST_CONTAINER_CLASS}>
      {Array.from({ length: 8 }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-3 p-3 sm:gap-4 sm:px-4"
          style={{ height: `${VIRTUAL_LIST_TRACK_HEIGHT}px` }}
        >
          <div
            className={cn(
              "size-9 shrink-0 rounded-full bg-muted",
              MOTION_LOADING_STATE_CLASS,
            )}
          />
          <div className="min-w-0 flex-1">
            <div className={cn("h-4 w-1/2 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
            <div className={cn("mt-2 h-3 w-1/3 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
          </div>
          <div className={cn("h-3 w-10 shrink-0 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
        </div>
      ))}
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

  // Generate announcement for screen readers
  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (allTracks.length === 0) return "No tracks found";
    if (isFetchingNextPage) return undefined;
    return `${allTracks.length} of ${totalTracks} tracks loaded`;
  };

  if (isLoading) {
    return <TracksListSkeleton />;
  }

  if (allTracks.length === 0) {
    return (
      <div className="py-12 text-center text-muted-foreground">
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
        allTracks={allTracks}
        likedSet={likedSet}
        totalTracks={totalTracks}
        hasNextPage={hasNextPage}
        isFetchingNextPage={isFetchingNextPage}
        fetchNextPage={fetchNextPage}
      />
    </div>
  );
}

type VirtualizedTracksListProps = {
  virtualItems: VirtualItem[];
  allTracks: TrackListItemType[];
  likedSet: Set<number>;
  totalTracks: number;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => Promise<unknown>;
};

function VirtualizedTracksList({
  virtualItems,
  allTracks,
  likedSet,
  totalTracks,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
}: VirtualizedTracksListProps) {
  "use no memo";

  const { listRef, scrollMargin } = useWindowScrollMargin<HTMLDivElement>();

  // Rows are memoized (see TrackListItem), so they receive the loaded track
  // list through a ref whose identity never changes; clicking play reads the
  // freshest list from it to queue the whole tab.
  const allTracksRef = useRef<TrackListItemType[]>(allTracks);
  useEffect(() => {
    allTracksRef.current = allTracks;
  });

  const onChange = useVirtualizedInfiniteLoader({
    itemCount: virtualItems.length,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
    scopeKey: "music-tracks",
  });

  const virtualizer = useWindowVirtualizer({
    count: virtualItems.length,

    estimateSize: index => {
      const item = virtualItems[index];

      return item?.type === "letter"
        ? VIRTUAL_LIST_LETTER_HEIGHT
        : VIRTUAL_LIST_TRACK_HEIGHT;
    },

    overscan: 5,
    scrollMargin,
    onChange,
  });

  const renderedVirtualItems = virtualizer.getVirtualItems();

  useEffect(() => {
    virtualizer.measure();
  }, [scrollMargin, virtualizer, virtualItems.length]);

  return (
    <div
      ref={listRef}
      className={TRACK_LIST_CONTAINER_CLASS}
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
                <TrackListItem
                  track={item.track}
                  isLiked={likedSet.has(item.track.id)}
                  queueRef={allTracksRef}
                />
              )}
            </div>
          );
        })}

      </div>

      {isFetchingNextPage && (
        <div className="flex justify-center py-4">
          <Spinner className="size-6 text-primary" />
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
    }

    setIsLoading(false);
  };

  return (
    <Button
      variant="outline"
      onClick={handlePlayAll}
      disabled={isLoading}
      className="min-h-10 rounded-full"
      aria-label="Play all tracks"
    >
      {isLoading ? (
        <Spinner className="size-4" />
      ) : (
        <Play className="size-4 fill-current" aria-hidden="true" />
      )}
      <span>Play all</span>
    </Button>
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
    }

    setIsLoading(false);
  };

  return (
    <Button
      variant="accent-pill"
      onClick={handleShuffle}
      disabled={isLoading}
      className="min-h-10"
      aria-label="Shuffle all tracks"
    >
      {isLoading ? (
        <Spinner className="size-4" />
      ) : (
        <Shuffle className="size-4" aria-hidden="true" />
      )}
      <span>Shuffle all</span>
    </Button>
  );
}

function LetterHeader({ letter }: { letter: string }) {
  return (
    <div
      className="border-b border-primary/20 bg-muted/50 px-4 py-3"
      role="heading"
      aria-level={3}
      aria-label={`Tracks starting with ${letter}`}
    >
      <span className="text-2xl font-bold text-primary">{letter}</span>
    </div>
  );
}

// Memoized because the parent VirtualizedTracksList opts out of the React
// Compiler ("use no memo") and re-renders on every scroll tick; without memo,
// each windowed row re-renders each frame even though `track` (stable identity
// from the cached query pages) and `isLiked` (boolean) are unchanged. The
// compiler can't cover this: it only memoizes within a component, and the
// opted-out parent hands fresh row JSX each render, so this memo is required.
// react-doctor-disable-next-line react-doctor/react-compiler-no-manual-memoization
const TrackListItem = memo(function TrackListItem({
  track,
  isLiked,
  queueRef,
}: {
  track: TrackListItemType;
  isLiked: boolean;
  queueRef: React.RefObject<TrackListItemType[]>;
}) {
  const audioPlayer = useAudioPlayerActions();
  const playerState = useAudioPlayerState();

  const handlePlay = () => {
    audioPlayer.playTrackFromList(queueRef.current, track.id);
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
});

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

  // Generate announcement for screen readers.
  // Reached only after the isLoading early-return above, so no loading case here.
  const getAnnouncement = () => {
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
        <span className="text-sm text-muted-foreground">
          {playlists.length} {playlists.length === 1 ? "playlist" : "playlists"}
        </span>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={handleShowLiked}
            className="min-h-10 rounded-full"
            aria-label="View liked tracks"
          >
            <Heart className="size-4 shrink-0" aria-hidden="true" />
            Liked tracks
          </Button>
          <Button
            ref={createPlaylistRestoreRef}
            variant="accent-pill"
            onClick={handleCreateOpen}
            className="min-h-10"
            aria-label="Create new playlist"
          >
            <Plus className="size-4 shrink-0" aria-hidden="true" />
            New playlist
          </Button>
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

  // Reached only after the isLoading early-return below, so no loading case here.
  const getAnnouncement = () => {
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
    const audioTrack = convertToAudioTrack(track);
    const allAudioTracks = tracks.map((t) => convertToAudioTrack(t));
    audioPlayer.playTrack(audioTrack, allAudioTracks, {
      cover: null,
      title: "Liked Tracks",
      musician: null,
    });
  };

  if (isLoading) {
    return <TracksListSkeleton />;
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
            className={cn(
              "flex items-center gap-2 rounded-sm text-sm text-muted-foreground hover:text-foreground focus-visible:text-primary",
              MOTION_MICRO_CONTROL_CLASS,
              FOCUS_VISIBLE_RING_CLASS,
            )}
            aria-label="Back to playlists"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            Playlists
          </button>
          <span className="text-muted-foreground" aria-hidden="true">/</span>
          <h2 className="flex items-center gap-2 font-semibold text-foreground">
            <Heart className="size-4 fill-current text-destructive" aria-hidden="true" />
            Liked Tracks
          </h2>
        </div>
        <span className="text-sm text-muted-foreground">
          {total} {total === 1 ? "track" : "tracks"}
        </span>
      </div>

      {/* Track list or empty state */}
      {tracks.length === 0 ? (
        <EmptyState
          bordered
          icon={Heart}
          title="No liked tracks yet"
          description="Tap the heart icon on any track to add it here."
        />
      ) : (
        <div className={TRACK_LIST_CONTAINER_CLASS}>
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
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className={cn("h-4 w-24 rounded-sm bg-muted", MOTION_LOADING_STATE_CLASS)} />
        <div className="flex flex-wrap gap-2">
          <div className={cn("h-10 w-32 rounded-full bg-muted", MOTION_LOADING_STATE_CLASS)} />
          <div className={cn("h-10 w-32 rounded-full bg-muted", MOTION_LOADING_STATE_CLASS)} />
        </div>
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
            <div className="mx-auto mt-2 h-3 w-1/2 rounded-sm bg-muted" />
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
    <EmptyState
      icon={ListMusic}
      title="No playlists yet"
      description="Create your first playlist to start organizing your favorite tracks."
      action={
        <Button
          variant="accent-pill"
          size="lg"
          onClick={onCreateClick}
          className="font-semibold shadow-lg shadow-primary/20"
        >
          <Plus className="size-4" aria-hidden="true" />
          Create your first playlist
        </Button>
      }
    />
  );
}
