import { memo, useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import {
  ArrowLeft,
  Disc3,
  Heart,
  List,
  ListMusic,
  Music,
  Play,
  Plus,
  Shuffle,
  User,
  Users,
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { useVirtualizedInfiniteLoader } from "@/hooks/useVirtualizedInfiniteLoader";
import { useWindowScrollMargin } from "@/hooks/useWindowScrollMargin";
import { showActionFailed } from "@/lib/toast-helpers";
import { refreshMusicLibraryCache } from "@/lib/music-library-cache";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { isApiFailure } from "@/lib/is-api-failure";
import { trackRowProps } from "@/lib/track-row-props";
import {
  albumsPaginatedQueryOpts,
  likedTracksQueryOpts,
  musiciansPaginatedQueryOpts,
  musicStatsQueryOpts,
  playlistsQueryOpts,
  spotifyStatusQueryOpts,
  tracksInfiniteQueryOpts,
} from "@/lib/query-opts";
import { convertToAudioTrack } from "@/lib/audio-utils";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import {
  ALBUMS_PER_PAGE,
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
  MUSICIANS_PER_PAGE,
  TRACK_LIST_CONTAINER_CLASS,
  VIRTUAL_LIST_LETTER_HEIGHT,
  VIRTUAL_LIST_TRACK_HEIGHT,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import { scrollWindowToTop } from "@/lib/motion";

import AlbumCard, { AlbumCardSkeleton } from "@/components/music/AlbumCard";
import MusicianCard, {
  MusicianCardSkeleton,
} from "@/components/music/MusicianCard";
import LibraryAllTab, {
  type LibraryNoun,
} from "@/components/shared/LibraryAllTab";
import LibraryMoreMenu, {
  RefreshLibraryMenuItem,
} from "@/components/shared/LibraryMoreMenu";
import LibraryPagination from "@/components/shared/LibraryPagination";
import LibraryStats from "@/components/shared/LibraryStats";
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

const ALBUM_NOUN: LibraryNoun = { singular: "album", plural: "albums" };
const MUSICIAN_NOUN: LibraryNoun = { singular: "musician", plural: "musicians" };
const TRACK_NOUN: LibraryNoun = { singular: "track", plural: "tracks" };

// Five columns on large screens: circular thumbs and playlist covers read
// better with a little more room than the six-column poster grid gives.
const MUSIC_CARD_GRID_CLASS =
  "grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5";

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

  let topLevelTabContent = <AlbumsTabContent currentPage={albumsPage} />;

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
        <LibraryStats
          queryOpts={musicStatsQueryOpts()}
          figures={[
            {
              icon: Disc3,
              label: "Albums",
              noun: ALBUM_NOUN,
              getValue: data => data.total_albums,
            },
            {
              icon: Music,
              label: "Tracks",
              noun: TRACK_NOUN,
              getValue: data => data.total_tracks,
            },
            {
              icon: User,
              label: "Musicians",
              noun: MUSICIAN_NOUN,
              getValue: data => data.total_musicians,
            },
          ]}
        />
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

function MoreMenu() {
  const moreOptionsButtonRef = useRef<HTMLButtonElement | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [requestAlbumOpen, setRequestAlbumOpen] = useState(false);
  const [requestTrackOpen, setRequestTrackOpen] = useState(false);
  const { data: spotifyStatusData, isLoading: spotifyStatusLoading } = useQuery(
    spotifyStatusQueryOpts(),
  );

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
      <LibraryMoreMenu
        open={menuOpen}
        onOpenChange={setMenuOpen}
        triggerRef={moreOptionsButtonRef}
      >
        <RefreshLibraryMenuItem
          refresh={refreshMusicLibraryCache}
          libraryNoun="Music"
          onSettled={() => setMenuOpen(false)}
        />
        <DropdownMenuItem
          className={LIBRARY_MENU_ITEM_CLASS}
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
          className={LIBRARY_MENU_ITEM_CLASS}
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
      </LibraryMoreMenu>

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

function MusiciansTabContent({ currentPage }: { currentPage: number }) {
  const navigate = Route.useNavigate();

  return (
    <LibraryAllTab
      queryOpts={musiciansPaginatedQueryOpts(currentPage, MUSICIANS_PER_PAGE)}
      getItems={data => data.musicians}
      renderCard={musician => <MusicianCard musician={musician} />}
      currentPage={currentPage}
      perPage={MUSICIANS_PER_PAGE}
      noun={MUSICIAN_NOUN}
      emptyIcon={Users}
      gridClassName={MUSIC_CARD_GRID_CLASS}
      skeletonCard={<MusicianCardSkeleton />}
      onPageChange={newPage =>
        navigate({
          to: "/music",
          search: (prev: MusicSearchParams) => ({
            ...prev,
            musiciansPage: newPage,
          }),
          replace: true,
        })
      }
    />
  );
}

function AlbumsTabContent({ currentPage }: { currentPage: number }) {
  const navigate = Route.useNavigate();

  return (
    <LibraryAllTab
      queryOpts={albumsPaginatedQueryOpts(currentPage, ALBUMS_PER_PAGE)}
      getItems={data => data.albums}
      renderCard={album => <AlbumCard album={album} />}
      currentPage={currentPage}
      perPage={ALBUMS_PER_PAGE}
      noun={ALBUM_NOUN}
      emptyIcon={Disc3}
      skeletonCard={<AlbumCardSkeleton />}
      onPageChange={newPage =>
        navigate({
          to: "/music",
          search: (prev: MusicSearchParams) => ({
            ...prev,
            albumsPage: newPage,
          }),
          replace: true,
        })
      }
    />
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
  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    isError,
    refetch,
  } = useInfiniteQuery(tracksInfiniteQueryOpts());

  const firstPage = data?.pages[0];
  const firstPageFailed = isApiFailure(firstPage);

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

  if (isError || firstPageFailed) {
    return (
      <MoviesLoadError
        message={
          firstPageFailed
            ? firstPage.message
            : "Couldn’t load tracks. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  if (allTracks.length === 0) {
    return (
      <div className="py-12 text-center text-muted-foreground">
        <LiveAnnouncer message={getAnnouncement()} />
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
  totalTracks: number;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => Promise<unknown>;
};

function VirtualizedTracksList({
  virtualItems,
  allTracks,
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
      // Tailwind's preflight sets list-style:none on every <ul>, which drops list
      // semantics in Safari/VoiceOver; the role restores them.
      // react-doctor-disable-next-line react-doctor/prefer-tag-over-role
      role="list"
      aria-label="Tracks"
    >
      {/* Presentational spacer: keeps the list -> listitem ownership intact so
          the intervening positioning wrapper doesn't strip the list semantics. */}
      <div
        role="presentation"
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
              role={item.type === "track" ? "listitem" : "presentation"}
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
    <h3
      className="border-b border-primary/20 bg-muted/50 px-4 py-3 text-2xl font-bold text-primary"
      aria-label={`Tracks starting with ${letter}`}
    >
      {letter}
    </h3>
  );
}

// Memoized because the parent VirtualizedTracksList opts out of the React
// Compiler ("use no memo") and re-renders on every scroll tick; without memo,
// each windowed row re-renders each frame even though `track` has stable
// identity from the cached query pages. The compiler can't cover this: it only
// memoizes within a component, and the opted-out parent hands fresh row JSX
// each render, so this memo is required.
// react-doctor-disable-next-line react-doctor/react-compiler-no-manual-memoization
const TrackListItem = memo(function TrackListItem({
  track,
  queueRef,
}: {
  track: TrackListItemType;
  queueRef: React.RefObject<TrackListItemType[]>;
}) {
  const audioPlayer = useAudioPlayerActions();
  const matchTrackPlayback = useTrackPlaybackMatcher();

  const handlePlay = () => {
    audioPlayer.playTrackFromList(queueRef.current, track.id);
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
        <div className={MUSIC_CARD_GRID_CLASS}>
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
  const matchTrackPlayback = useTrackPlaybackMatcher();

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
    scrollWindowToTop();
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
              {...trackRowProps(track)}
              variant="library"
              {...matchTrackPlayback(track.id)}
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
      <div className={MUSIC_CARD_GRID_CLASS}>
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
