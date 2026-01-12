import { useEffect, useRef, useState } from "react";
import { Link, createLazyFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { musicStatsQueryOpts, tracksInfiniteQueryOpts } from "@/lib/query-opts";
import { formatTrackDuration } from "@/lib/format";
import { useAudioPlayer } from "@/context/AudioPlayerContext";
import type { TrackListItemType } from "@/types";

// Virtual item types for the virtualized list
type VirtualItemLetter = { type: "letter"; letter: string };
type VirtualItemTrack = { type: "track"; track: TrackListItemType };
type VirtualItem = VirtualItemLetter | VirtualItemTrack;

// Height constants for virtual items
const LETTER_HEIGHT = 52;
const TRACK_HEIGHT = 60;

export const Route = createLazyFileRoute("/_auth/music/")({
  component: MusicPage,
});

function MusicPage() {
  return (
    <div>
      {/* Page header */}
      <header
        className='mb-8 rounded-lg p-3 -m-3 focus:outline-none focus:ring-2 focus:ring-amber-400 focus:bg-slate-800/30'
        tabIndex={0}
        aria-label='Music Library. Browse your collection of musicians, albums, and tracks.'
      >
        <h1
          className='text-3xl md:text-4xl font-semibold tracking-tight text-white flex items-center gap-3'
          aria-hidden='true'
        >
          <i className='fa-solid fa-music text-amber-400 text-2xl' />
          <span>Music Library</span>
        </h1>
        <p
          className='mt-2 text-slate-400 max-w-2xl text-sm md:text-base'
          aria-hidden='true'
        >
          Browse your collection of musicians, albums, and tracks
        </p>
      </header>

      {/* Library Stats */}
      <LibraryStats />

      {/* Tabs */}
      <Tabs defaultValue='albums'>
        <TabsList className='bg-slate-800/50 border border-slate-700/50 h-auto p-1'>
          <TabsTrigger
            value='musicians'
            className='data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 text-slate-400 hover:text-white px-4 py-2'
          >
            <i className='fa-solid fa-user-group mr-2' aria-hidden='true' />
            Musicians
          </TabsTrigger>
          <TabsTrigger
            value='albums'
            className='data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 text-slate-400 hover:text-white px-4 py-2'
          >
            <i className='fa-solid fa-compact-disc mr-2' aria-hidden='true' />
            Albums
          </TabsTrigger>
          <TabsTrigger
            value='tracks'
            className='data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 text-slate-400 hover:text-white px-4 py-2'
          >
            <i className='fa-solid fa-list mr-2' aria-hidden='true' />
            Tracks
          </TabsTrigger>
        </TabsList>

        <TabsContent value='musicians' className='mt-6'>
          <div className='text-slate-400'>Musicians content coming soon...</div>
        </TabsContent>

        <TabsContent value='albums' className='mt-6'>
          <div className='text-slate-400'>Albums content coming soon...</div>
        </TabsContent>

        <TabsContent value='tracks' className='mt-6'>
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
      className='flex flex-wrap gap-6 mb-6 rounded-lg p-3 -m-3 focus:outline-none focus:ring-2 focus:ring-amber-400 focus:bg-slate-800/30'
      aria-label={statsLabel}
      tabIndex={0}
    >
      <div className='flex items-center gap-2' aria-hidden='true'>
        <i className='fa-solid fa-compact-disc text-amber-400' />
        <span className='text-white font-medium'>{albumCount}</span>
        <span className='text-slate-400'>Albums</span>
      </div>
      <div className='flex items-center gap-2' aria-hidden='true'>
        <i className='fa-solid fa-music text-amber-400' />
        <span className='text-white font-medium'>{trackCount}</span>
        <span className='text-slate-400'>Tracks</span>
      </div>
      <div className='flex items-center gap-2' aria-hidden='true'>
        <i className='fa-solid fa-user text-amber-400' />
        <span className='text-white font-medium'>{musicianCount}</span>
        <span className='text-slate-400'>Musicians</span>
      </div>
    </section>
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
      page.error === false ? (page.data?.tracks ?? []) : []
    ) ?? [];

  // Convert to virtual items (tracks + letter headers)
  const virtualItems = flattenToVirtualItems(allTracks);

  // Window virtualizer for efficient rendering
  const virtualizer = useWindowVirtualizer({
    count: virtualItems.length,
    estimateSize: index => {
      const item = virtualItems[index];
      return item?.type === "letter" ? LETTER_HEIGHT : TRACK_HEIGHT;
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
      <div className='flex justify-center py-12'>
        <Spinner className='h-8 w-8 text-amber-400' />
      </div>
    );
  }

  if (allTracks.length === 0) {
    return (
      <div className='text-center py-12 text-slate-400'>
        <i className='fa-solid fa-music text-4xl mb-4 block opacity-50' />
        <p>No tracks found in your library.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Header with track count and play/shuffle buttons */}
      <div className='flex items-center justify-between mb-4'>
        <span className='text-slate-400 text-sm'>
          {totalTracks.toLocaleString()} tracks
        </span>
        <div className='flex items-center gap-2'>
          <PlayAllButton />
          <ShuffleButton />
        </div>
      </div>

      {/* Virtualized tracks list */}
      <div
        ref={listRef}
        className='rounded-lg border border-slate-800 bg-slate-900/50 overflow-hidden'
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
          <div className='flex justify-center py-4'>
            <Spinner className='h-6 w-6 text-amber-400' />
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
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handlePlayAll}
      disabled={isLoading}
      className='inline-flex items-center gap-2 px-4 py-2 rounded-full bg-slate-700 hover:bg-slate-600 text-white font-medium transition-colors disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
      aria-label='Play all tracks'
    >
      {isLoading ? (
        <Spinner className='h-4 w-4' />
      ) : (
        <i className='fa-solid fa-play' aria-hidden='true' />
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
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handleShuffle}
      disabled={isLoading}
      className='inline-flex items-center gap-2 px-4 py-2 rounded-full bg-amber-500 hover:bg-amber-400 text-slate-900 font-medium transition-colors disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
      aria-label='Shuffle all tracks'
    >
      {isLoading ? (
        <Spinner className='h-4 w-4' />
      ) : (
        <i className='fa-solid fa-shuffle' aria-hidden='true' />
      )}
      <span>Shuffle All</span>
    </button>
  );
}

function LetterHeader({ letter }: { letter: string }) {
  return (
    <div className='border-b border-amber-500/20 px-4 py-3 bg-slate-800/50'>
      <span className='text-2xl font-bold text-amber-400'>{letter}</span>
    </div>
  );
}

function TrackListItem({ track }: { track: TrackListItemType }) {
  const audioPlayer = useAudioPlayer();

  const handlePlay = () => {
    // Create a minimal track type for the audio player
    const audioTrack = {
      id: track.id,
      title: track.title,
      sort_title: track.title,
      file_path: track.file_path,
      file_name: "",
      container: "",
      mime_type: "",
      codec: track.codec,
      size: 0,
      track_index: 0,
      duration: track.duration,
      disc: 1,
      channels: "",
      channel_layout: "",
      bit_rate: track.bit_rate,
      profile: "",
      release_date: { String: "", Valid: false },
      year: { Int64: 0, Valid: false },
      composer: { String: "", Valid: false },
      copyright: { String: "", Valid: false },
      language: { String: "", Valid: false },
      album_id: track.album_id,
      musician_id: track.musician_id,
      created_at: "",
      updated_at: "",
    };

    audioPlayer.playTrack(audioTrack, [audioTrack], {
      cover: track.album_cover?.Valid ? track.album_cover.String : null,
      title: track.album_title.Valid
        ? track.album_title.String
        : "Unknown Album",
      musician: track.musician_name.Valid ? track.musician_name.String : null,
    });
  };

  return (
    <div className='flex items-center gap-4 px-4 py-3 hover:bg-slate-800/50 border-b border-slate-800 last:border-b-0 group'>
      {/* Play button - always visible */}
      <button
        onClick={handlePlay}
        className='w-9 h-9 flex items-center justify-center rounded-full bg-amber-500 hover:bg-amber-400 text-slate-900 transition-colors shrink-0'
        aria-label={`Play ${track.title}`}
      >
        <i className='fa-solid fa-play text-sm ml-0.5' aria-hidden='true' />
      </button>

      {/* Track info */}
      <div className='flex-1 min-w-0'>
        <p className='text-white font-medium truncate'>{track.title}</p>
        <p className='text-slate-400 text-sm truncate'>
          {track.musician_name.Valid
            ? track.musician_name.String
            : "Unknown Artist"}
        </p>
      </div>

      {/* Duration */}
      <span className='text-slate-400 text-sm tabular-nums hidden sm:block'>
        {formatTrackDuration(track.duration)}
      </span>

      {/* More menu - dropdown */}
      <TrackActionsMenu track={track} />
    </div>
  );
}

function TrackActionsMenu({ track }: { track: TrackListItemType }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          className='w-8 h-8 flex items-center justify-center rounded-full hover:bg-slate-700 text-slate-400 hover:text-white transition-colors'
          aria-label='More actions'
        >
          <i className='fa-solid fa-ellipsis-vertical' aria-hidden='true' />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align='end'
        className='bg-slate-800 border-slate-700 text-white'
      >
        {track.album_id.Valid && (
          <DropdownMenuItem
            asChild
            className='hover:bg-slate-700 cursor-pointer'
          >
            <Link
              to='/music/album/$id'
              params={{ id: track.album_id.Int64.toString() }}
            >
              <i
                className='fa-solid fa-compact-disc mr-2 text-amber-400'
                aria-hidden='true'
              />
              Go to Album
            </Link>
          </DropdownMenuItem>
        )}
        {/* TODO: Add to queue when audio player supports it */}
        {/* TODO: Go to Artist when artist pages are implemented */}
      </DropdownMenuContent>
    </DropdownMenu>
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
