import { memo, useEffect, useRef, useState, type RefObject } from "react";
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
  Shuffle,
  User,
  Users,
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Spinner } from "@/components/ui/spinner";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { useVirtualizedInfiniteLoader } from "@/hooks/useVirtualizedInfiniteLoader";
import { useWindowScrollMargin } from "@/hooks/useWindowScrollMargin";
import { showActionFailed } from "@/lib/toast-helpers";
import { refreshMusicLibraryCache } from "@/lib/music-library-cache";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import LoadErrorAlert from "@/components/shared/LoadErrorAlert";
import { apiErrorMessage, isApiFailure } from "@/lib/is-api-failure";
import { pluralize } from "@/lib/format";
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
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import {
  ALBUMS_PER_PAGE,
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  LIKED_TRACKS_PER_PAGE,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MUSIC_CARD_GRID_CLASS,
  MUSICIANS_PER_PAGE,
  TRACK_LIST_CONTAINER_CLASS,
  VIRTUAL_LIST_LETTER_HEIGHT,
  VIRTUAL_LIST_TRACK_HEIGHT,
  LIBRARY_NOUNS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

import AlbumCard, { AlbumCardSkeleton } from "@/components/music/AlbumCard";
import MusicianCard, {
  MusicianCardSkeleton,
} from "@/components/music/MusicianCard";
import LibraryAllTab from "@/components/shared/LibraryAllTab";
import LibraryMoreMenu, {
  RequestMediaMenuItem,
  RefreshLibraryMenuItem,
} from "@/components/shared/LibraryMoreMenu";
import LibraryStats from "@/components/shared/LibraryStats";
import PlaylistCard, { PlaylistCardSkeleton } from "@/components/shared/PlaylistCard";
import PlaylistsTabToolbar from "@/components/shared/PlaylistsTabToolbar";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import TrackItem from "@/components/music/TrackItem";
import LibraryEmptyState from "@/components/shared/LibraryEmptyState";
import { Button } from "@/components/ui/button";
import PlaylistFormDialog from "@/components/music/PlaylistFormDialog";
import RequestAlbumDialog from "@/components/music/RequestAlbumDialog";
import RequestTrackDialog from "@/components/music/RequestTrackDialog";
import type { TrackListItemType, VirtualItem } from "@/types";
import {
  musicSearchSchema,
  type MusicSearchParams,
} from "@/lib/route-search";
import { routeHead } from "@/lib/route-head";

const MUSIC_HEAD = routeHead(
  "Music Library",
  "Browse your collection of musicians, albums, tracks, and playlists in your Igloo media library.",
);

export const Route = createFileRoute("/_auth/music/")({
  head: () => MUSIC_HEAD,
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
          "mb-5 flex items-start justify-between gap-3",
          MOTION_SECTION_ENTER_DELAYED_CLASS,
        )}
      >
        <LibraryStats
          className="min-w-0 flex-1"
          queryOpts={musicStatsQueryOpts()}
          figures={[
            {
              icon: Disc3,
              label: "Albums",
              noun: LIBRARY_NOUNS.album,
              getValue: data => data.total_albums,
            },
            {
              icon: Music,
              label: "Tracks",
              noun: LIBRARY_NOUNS.track,
              getValue: data => data.total_tracks,
            },
            {
              icon: User,
              label: "Musicians",
              noun: LIBRARY_NOUNS.musician,
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
        <RequestMediaMenuItem
          label="Request Album"
          provider="Spotify"
          available={spotifyAvailable}
          statusLoading={spotifyStatusLoading}
          onSelect={() => setRequestAlbumOpen(true)}
        />
        <RequestMediaMenuItem
          label="Request Track"
          provider="Spotify"
          available={spotifyAvailable}
          statusLoading={spotifyStatusLoading}
          onSelect={() => setRequestTrackOpen(true)}
        />
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
      noun={LIBRARY_NOUNS.musician}
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
      noun={LIBRARY_NOUNS.album}
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
// One placeholder row, the height of a real track row; the Tracks tab stacks
// eight, the liked view one per row on a full page.
function TrackRowSkeleton() {
  return (
    <div
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
  );
}

function TracksListSkeleton() {
  return (
    <div className={TRACK_LIST_CONTAINER_CLASS}>
      {Array.from({ length: 8 }).map((_, i) => (
        <TrackRowSkeleton key={i} />
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
      <LoadErrorAlert
        message={apiErrorMessage(
          firstPage,
          "Couldn’t load tracks. Check your connection and try again.",
        )}
        onRetry={() => void refetch()}
      />
    );
  }

  if (allTracks.length === 0) {
    return (
      <div>
        <LiveAnnouncer message={getAnnouncement()} />
        <LibraryEmptyState icon={Music} message="No tracks found in your library." />
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

// The library-wide pair stays focusable while its first batch loads: a
// disabled media control drops out of iOS VoiceOver's focus order (§1.7), so
// aria-disabled plus a guard stops a second start instead.
function PlayAllButton() {
  const [isLoading, setIsLoading] = useState(false);
  const audioPlayer = useAudioPlayerActions();

  const handlePlayAll = async () => {
    if (isLoading) return;
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
      aria-disabled={isLoading}
      className="min-h-10 rounded-full aria-disabled:opacity-50"
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
    if (isLoading) return;
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
      aria-disabled={isLoading}
      className="min-h-10 aria-disabled:opacity-50"
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

type PlaylistsFocusIntent = "enter-liked-from-toolbar" | "return-to-playlists";

// Playlists tab content
function PlaylistsTabContent({ playlistsView, likedTracksPage }: PlaylistsTabContentProps) {
  const navigate = Route.useNavigate();
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const createPlaylistRestoreRef = useRef<HTMLButtonElement | null>(null);
  const likedTracksButtonRef = useRef<HTMLButtonElement | null>(null);
  // Switching views swaps the whole tab body, so the button that was pressed
  // is gone; this says which control should take focus once the other view
  // has rendered.
  const focusIntentRef = useRef<PlaylistsFocusIntent | null>(null);
  const { data, isLoading, isError, refetch } = useQuery({
    ...playlistsQueryOpts(),
    enabled: playlistsView !== "liked",
  });

  const playlists = data?.error === false ? data.data.playlists : [];
  const loadFailed = isError || isApiFailure(data);

  useEffect(() => {
    if (playlistsView === "liked") return;
    if (focusIntentRef.current !== "return-to-playlists") return;

    focusIntentRef.current = null;
    focusDialogRestoreTarget(likedTracksButtonRef.current);
  }, [playlistsView]);

  const handleShowLiked = () => {
    focusIntentRef.current = "enter-liked-from-toolbar";
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        playlistsView: "liked",
        likedTracksPage: 1,
      }),
      replace: true,
    });
  };

  const handleExitLiked = () => {
    focusIntentRef.current = "return-to-playlists";
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        playlistsView: "playlists",
        likedTracksPage: 1,
      }),
      replace: true,
    });
  };

  const handleCreateOpen = () => {
    setShowCreateDialog(true);
  };

  if (playlistsView === "liked") {
    return (
      <LikedTracksInPlaylistsTab
        likedTracksPage={likedTracksPage}
        onExit={handleExitLiked}
        focusIntentRef={focusIntentRef}
      />
    );
  }

  // Generate announcement for screen readers.
  const getAnnouncement = () => {
    if (playlists.length === 0) return "No playlists yet";
    return `${pluralize(playlists.length, "playlist")} loaded`;
  };

  let body = (
    <div className={MUSIC_CARD_GRID_CLASS}>
      {playlists.map((playlist) => (
        <PlaylistCard key={playlist.id} playlist={playlist} />
      ))}
    </div>
  );

  if (isLoading) {
    body = <PlaylistsGridSkeleton />;
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
        icon={ListMusic}
        message="No playlists yet. Use New playlist to group tracks."
      />
    );
  }

  return (
    <div>
      {/* Announce content changes to screen readers, once there is a list to describe */}
      {!isLoading && !loadFailed && <LiveAnnouncer message={getAnnouncement()} />}
      <PlaylistsTabToolbar
        count={isLoading || loadFailed ? undefined : playlists.length}
        isLoading={isLoading}
        likedLabel="Liked tracks"
        likedAriaLabel="View liked tracks"
        createAriaLabel="Create new playlist"
        onShowLiked={handleShowLiked}
        onCreate={handleCreateOpen}
        likedButtonRef={likedTracksButtonRef}
        createButtonRef={createPlaylistRestoreRef}
      />

      {body}

      <PlaylistFormDialog
        mode="create"
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
  focusIntentRef: RefObject<PlaylistsFocusIntent | null>;
};

const LIKED_TRACK_NOUN = { singular: "liked track", plural: "liked tracks" };

function LikedTracksInPlaylistsTab({
  likedTracksPage,
  onExit,
  focusIntentRef,
}: LikedTracksInPlaylistsTabProps) {
  const navigate = Route.useNavigate();
  const backToPlaylistsButtonRef = useRef<HTMLButtonElement | null>(null);
  const audioPlayer = useAudioPlayerActions();
  const matchTrackPlayback = useTrackPlaybackMatcher();

  // The same key LibraryAllTab runs below, so TanStack serves both from one
  // request; the page reads it for the count and the play queue.
  const { data } = useQuery(likedTracksQueryOpts(likedTracksPage));

  const tracks = data?.error === false ? data.data.tracks : [];
  const total = data?.error === false ? data.data.total : 0;

  // The header renders while the list loads, so Back can take focus at once.
  useEffect(() => {
    if (focusIntentRef.current !== "enter-liked-from-toolbar") return;
    if (!backToPlaylistsButtonRef.current) return;

    focusIntentRef.current = null;
    focusDialogRestoreTarget(backToPlaylistsButtonRef.current);
  }, [focusIntentRef]);

  const handlePageChange = (newPage: number) => {
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        likedTracksPage: newPage,
      }),
      replace: true,
    });
  };

  // playTrackFromList, as the library tab does: a click on the current row
  // toggles play/pause, and the raw rows keep each track's own cover and
  // artist as the queue advances.
  const handlePlayTrack = (track: TrackListItemType) => {
    audioPlayer.playTrackFromList(tracks, track.id);
  };

  return (
    <LibraryAllTab
      queryOpts={likedTracksQueryOpts(likedTracksPage)}
      getItems={data => data.tracks}
      renderCard={track => (
        <TrackItem
          {...trackRowProps(track)}
          variant="library"
          {...matchTrackPlayback(track.id)}
          onPlay={() => handlePlayTrack(track)}
          showActionsMenu
        />
      )}
      currentPage={likedTracksPage}
      perPage={LIKED_TRACKS_PER_PAGE}
      noun={LIKED_TRACK_NOUN}
      emptyIcon={Heart}
      emptyMessage="No liked tracks yet. Tap the heart on any track to add it here."
      onPageChange={handlePageChange}
      gridClassName={TRACK_LIST_CONTAINER_CLASS}
      skeletonCard={<TrackRowSkeleton />}
      toolbarStartSlot={
        <>
          <button
            ref={backToPlaylistsButtonRef}
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
          {data?.error === false && (
            <span className="text-sm text-muted-foreground">
              {pluralize(total, "track")}
            </span>
          )}
        </>
      }
    />
  );
}

function PlaylistsGridSkeleton() {
  return (
    <div className={MUSIC_CARD_GRID_CLASS}>
      {Array.from({ length: 10 }).map((_, i) => (
        <PlaylistCardSkeleton key={i} />
      ))}
    </div>
  );
}
