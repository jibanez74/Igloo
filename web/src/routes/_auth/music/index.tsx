import { useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { fallback, zodSearchValidator } from "@tanstack/router-zod-adapter";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { z } from "zod";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "sonner";
import {
  albumsPaginatedQueryOpts,
  musiciansPaginatedQueryOpts,
  musicStatsQueryOpts,
  tracksInfiniteQueryOpts,
} from "@/lib/query-opts";
import { convertToAudioTrack } from "@/lib/audio-utils";
import { useAudioPlayer } from "@/hooks/useAudioPlayer";
import {
  ALBUMS_PER_PAGE,
  MUSICIANS_PER_PAGE,
  VIRTUAL_LIST_LETTER_HEIGHT,
  VIRTUAL_LIST_TRACK_HEIGHT,
} from "@/lib/constants";
import AlbumCard from "@/components/AlbumCard";
import MusicianCard from "@/components/MusicianCard";
import LibraryPagination from "@/components/LibraryPagination";
import TrackItem from "@/components/TrackItem";
import type { TrackListItemType, VirtualItem } from "@/types";

const musicSearchSchema = z.object({
  tab: fallback(z.enum(["musicians", "albums", "tracks"]), "albums").default(
    "albums"
  ),
  albumsPage: fallback(z.number().int().positive(), 1).default(1),
  musiciansPage: fallback(z.number().int().positive(), 1).default(1),
});

type MusicSearchParams = z.infer<typeof musicSearchSchema>;

export const Route = createFileRoute("/_auth/music/")({
  validateSearch: zodSearchValidator(musicSearchSchema),
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
  const { tab, albumsPage, musiciansPage } = Route.useSearch();

  // Handle tab change - update URL while preserving other params
  const handleTabChange = (newTab: string) => 
    navigate({
      to: "/music",
      search: (prev: MusicSearchParams) => ({
        ...prev,
        tab: newTab as MusicSearchParams["tab"],
      }),
      replace: true,
    });

  return (
    <div>
      {/* Page header */}
      <header
        className="-m-3 mb-8 rounded-lg p-3 focus:bg-slate-800/30 focus:ring-2 focus:ring-amber-400 focus:outline-none"
        tabIndex={0}
        aria-label="Music Library. Browse your collection of musicians, albums, and tracks."
      >
        <h1
          className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-white md:text-4xl"
          aria-hidden="true"
        >
          <i className="fa-solid fa-music text-2xl text-amber-400" />
          <span>Music Library</span>
        </h1>
        <p
          className="mt-2 max-w-2xl text-sm text-slate-400 md:text-base"
          aria-hidden="true"
        >
          Browse your collection of musicians, albums, and tracks
        </p>
      </header>

      {/* Library Stats */}
      <LibraryStats />

      {/* Tabs - controlled by URL search param */}
      <Tabs value={tab} onValueChange={handleTabChange}>
        <TabsList className="h-auto border border-slate-700/50 bg-slate-800/50 p-1">
          <TabsTrigger
            value="musicians"
            className="px-4 py-2 text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20"
          >
            <i className="fa-solid fa-user-group mr-2" aria-hidden="true" />
            Musicians
          </TabsTrigger>
          <TabsTrigger
            value="albums"
            className="px-4 py-2 text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20"
          >
            <i className="fa-solid fa-compact-disc mr-2" aria-hidden="true" />
            Albums
          </TabsTrigger>
          <TabsTrigger
            value="tracks"
            className="px-4 py-2 text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20"
          >
            <i className="fa-solid fa-list mr-2" aria-hidden="true" />
            Tracks
          </TabsTrigger>
        </TabsList>

        <TabsContent value="musicians" className="mt-6">
          <MusiciansTabContent currentPage={musiciansPage} />
        </TabsContent>

        <TabsContent value="albums" className="mt-6">
          <AlbumsTabContent
            currentPage={albumsPage}
            perPage={ALBUMS_PER_PAGE}
          />
        </TabsContent>

        <TabsContent value="tracks" className="mt-6">
          <TracksTabContent />
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
      className="-m-3 mb-6 flex flex-wrap gap-6 rounded-lg p-3 focus:bg-slate-800/30 focus:ring-2 focus:ring-amber-400 focus:outline-none"
      aria-label={statsLabel}
      tabIndex={0}
    >
      <div className="flex items-center gap-2" aria-hidden="true">
        <i className="fa-solid fa-compact-disc text-amber-400" />
        <span className="font-medium text-white">{albumCount}</span>
        <span className="text-slate-400">Albums</span>
      </div>
      <div className="flex items-center gap-2" aria-hidden="true">
        <i className="fa-solid fa-music text-amber-400" />
        <span className="font-medium text-white">{trackCount}</span>
        <span className="text-slate-400">Tracks</span>
      </div>
      <div className="flex items-center gap-2" aria-hidden="true">
        <i className="fa-solid fa-user text-amber-400" />
        <span className="font-medium text-white">{musicianCount}</span>
        <span className="text-slate-400">Musicians</span>
      </div>
    </section>
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
        <div className="h-4 w-24 animate-pulse rounded-sm bg-slate-800" />
        <div className="h-4 w-20 animate-pulse rounded-sm bg-slate-800" />
      </div>

      {/* Skeleton grid - matches actual grid dimensions */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: MUSICIANS_PER_PAGE }).map((_, i) => (
          <div
            key={i}
            className="animate-pulse rounded-xl border border-slate-800 bg-slate-900 p-4"
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
  const total = data?.error === false ? data.data.total : 0;

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
        <i
          className="fa-solid fa-user-group mb-4 block text-4xl opacity-50"
          aria-hidden="true"
        />
        <p>No musicians found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Header with count */}
      <div className="mb-6 flex items-center justify-between">
        <span className="text-sm text-slate-400">
          {total.toLocaleString()} musicians
        </span>
        <span className="text-sm text-slate-400">
          Page {currentPage} of {totalPages}
        </span>
      </div>

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
  const total = data?.error === false ? data.data.total : 0;

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
      <div className="flex justify-center py-12">
        <Spinner className="h-8 w-8 text-amber-400" />
      </div>
    );
  }

  if (albums.length === 0) {
    return (
      <div className="py-12 text-center text-slate-400">
        <i className="fa-solid fa-compact-disc mb-4 block text-4xl opacity-50" />
        <p>No albums found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Header with album count */}
      <div className="mb-6 flex items-center justify-between">
        <span className="text-sm text-slate-400">
          {total.toLocaleString()} albums
        </span>
        <span className="text-sm text-slate-400">
          Page {currentPage} of {totalPages}
        </span>
      </div>

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

  // Ref to measure offset from top of page for scrollMargin
  const listRef = useRef<HTMLDivElement>(null);
  const [scrollMargin, setScrollMargin] = useState(0);

  // Measure scroll margin after mount
  useEffect(() => {
    if (listRef.current) {
      setScrollMargin(listRef.current.offsetTop);
    }
  }, []);

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

  // Window virtualizer for efficient rendering
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
  });

  // Get virtual items for dependency tracking
  const renderedVirtualItems = virtualizer.getVirtualItems();

  // Trigger infinite scroll when near the end
  useEffect(() => {
    if (renderedVirtualItems.length === 0) return;

    const lastItem = renderedVirtualItems[renderedVirtualItems.length - 1];

    if (
      lastItem &&
      lastItem.index >= virtualItems.length - 10 &&
      hasNextPage &&
      !isFetchingNextPage
    ) {
      fetchNextPage();
    }
  }, [
    renderedVirtualItems,
    virtualItems.length,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
  ]);

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner className="h-8 w-8 text-amber-400" />
      </div>
    );
  }

  if (allTracks.length === 0) {
    return (
      <div className="py-12 text-center text-slate-400">
        <i className="fa-solid fa-music mb-4 block text-4xl opacity-50" />
        <p>No tracks found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Header with track count and play/shuffle buttons */}
      <div className="mb-4 flex items-center justify-between">
        <span className="text-sm text-slate-400">
          {totalTracks.toLocaleString()} tracks
        </span>
        <div className="flex items-center gap-2">
          <PlayAllButton />
          <ShuffleButton />
        </div>
      </div>

      {/* Virtualized tracks list */}
      <div
        ref={listRef}
        className="overflow-hidden rounded-lg border border-slate-800 bg-slate-900/50"
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
                  <TrackListItem track={item.track} />
                )}
              </div>
            );
          })}
        </div>

        {isFetchingNextPage && (
          <div className="flex justify-center py-4">
            <Spinner className="h-6 w-6 text-amber-400" />
          </div>
        )}
      </div>
    </div>
  );
}

function PlayAllButton() {
  const [isLoading, setIsLoading] = useState(false);
  const audioPlayer = useAudioPlayer();

  const handlePlayAll = async () => {
    setIsLoading(true);

    try {
      await audioPlayer.startPlayAllPlayback();
    } catch (error) {
      console.error("Failed to start playback:", error);
      toast.error("Playback failed", {
        description: "Unable to start playing all tracks. Please try again.",
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handlePlayAll}
      disabled={isLoading}
      className="inline-flex items-center gap-2 rounded-full bg-slate-700 px-4 py-2 font-medium text-white transition-colors hover:bg-slate-600 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:opacity-50"
      aria-label="Play all tracks"
    >
      {isLoading ? (
        <Spinner className="size-4" />
      ) : (
        <i className="fa-solid fa-play" aria-hidden="true" />
      )}
      <span>Play All</span>
    </button>
  );
}

function ShuffleButton() {
  const [isLoading, setIsLoading] = useState(false);
  const audioPlayer = useAudioPlayer();

  const handleShuffle = async () => {
    setIsLoading(true);

    try {
      await audioPlayer.startShufflePlayback();
    } catch (error) {
      console.error("Failed to start shuffle playback:", error);
      toast.error("Shuffle failed", {
        description: "Unable to start shuffle playback. Please try again.",
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handleShuffle}
      disabled={isLoading}
      className="inline-flex items-center gap-2 rounded-full bg-amber-500 px-4 py-2 font-medium text-slate-900 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:opacity-50"
      aria-label="Shuffle all tracks"
    >
      {isLoading ? (
        <Spinner className="size-4" />
      ) : (
        <i className="fa-solid fa-shuffle" aria-hidden="true" />
      )}
      <span>Shuffle All</span>
    </button>
  );
}

function LetterHeader({ letter }: { letter: string }) {
  return (
    <div className="animate-in border-b border-amber-500/20 bg-slate-800/50 px-4 py-3 duration-300 fade-in">
      <span className="text-2xl font-bold text-amber-400">{letter}</span>
    </div>
  );
}

function TrackListItem({ track }: { track: TrackListItemType }) {
  const audioPlayer = useAudioPlayer();

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
      cover: track.album_cover?.Valid ? track.album_cover.String : null,
      title: track.album_title.Valid
        ? track.album_title.String
        : "Unknown Album",
      musician: track.musician_name.Valid ? track.musician_name.String : null,
    });
  };

  return (
    <TrackItem
      id={track.id}
      title={track.title}
      duration={track.duration}
      subtitle={
        track.musician_name.Valid
          ? track.musician_name.String
          : "Unknown Artist"
      }
      albumId={track.album_id.Valid ? Number(track.album_id.Int64) : null}
      albumTitle={track.album_title.Valid ? track.album_title.String : undefined}
      musicianId={track.musician_id?.Valid ? Number(track.musician_id.Int64) : null}
      musicianName={track.musician_name.Valid ? track.musician_name.String : undefined}
      variant="library"
      isPlaying={audioPlayer.currentTrack?.id === track.id && audioPlayer.isPlaying}
      isCurrentTrack={audioPlayer.currentTrack?.id === track.id}
      onPlay={handlePlay}
      showActionsMenu
    />
  );
}

// Flatten tracks into virtual items with letter headers inserted
function flattenToVirtualItems(tracks: TrackListItemType[]): VirtualItem[] {
  const items: VirtualItem[] = [];
  let currentLetter: string | null = null;

  for (const track of tracks) {
    const firstChar = track.title.charAt(0).toUpperCase();
    const letter = /[A-Z]/.test(firstChar) ? firstChar : "#";

    // Insert letter header when we encounter a new letter
    if (letter !== currentLetter) {
      items.push({ type: "letter", letter });
      currentLetter = letter;
    }

    items.push({ type: "track", track });
  }

  return items;
}
