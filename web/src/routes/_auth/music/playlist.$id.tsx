import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMutation } from "@tanstack/react-query";
import {
  showDeleted,
  showRemoved,
  showActionFailed,
} from "@/lib/toast-helpers";
import {
  ListMusic,
  Music,
  Clock,
  User,
  Play,
  Shuffle,
  Pencil,
  Trash2,
  List,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import TrackItem from "@/components/music/TrackItem";
import PlaylistFormDialog from "@/components/music/PlaylistFormDialog";
import ConfirmDialog from "@/components/shared/ConfirmDialog";
import MusicDetailBackNav from "@/components/music/MusicDetailBackNav";
import MusicDetailSkeleton from "@/components/music/MusicDetailSkeleton";
import MediaDetailGuard from "@/components/shared/MediaDetailGuard";

// Lazy load DraggableTrackList to reduce initial bundle size
// This component includes the heavy @dnd-kit packages
const DraggableTrackList = lazy(() => import("@/components/music/DraggableTrackList"));
import {
  playlistDetailsQueryOpts,
  playlistTracksInfiniteQueryOpts,
} from "@/lib/query-opts";
import { trackRowProps } from "@/lib/track-row-props";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { unwrapString } from "@/lib/nullable";
import { deletePlaylist, removeTrackFromPlaylist, reorderPlaylistTracks } from "@/lib/api";
import { convertToAudioTrack, dedupeById } from "@/lib/audio-utils";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { useVirtualizedInfiniteLoader } from "@/hooks/useVirtualizedInfiniteLoader";
import { useWindowScrollMargin } from "@/hooks/useWindowScrollMargin";
import { formatDuration, pluralize } from "@/lib/format";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MUSIC_PLAYLISTS_TAB_SEARCH,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
  VIRTUAL_LIST_TRACK_HEIGHT,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import { parseRouteId } from "@/lib/route-id";
import type {
  PlayableTrackData,
  PlaylistDetailResponseType,
  PlaylistTrackType,
} from "@/types";

// A playlist row already carries every field the player needs, including the
// per-track album/artist. Keep it as PlayableTrackData and hand that to the
// player alongside the queue, or a mixed playlist shows the playlist's own name
// and cover for every track.
function playlistTrackToPlayableData(
  track: PlaylistTrackType,
): PlayableTrackData {
  return {
    id: track.id,
    title: track.title,
    duration: track.duration,
    codec: track.codec,
    bit_rate: track.bit_rate,
    album_id: track.album_id,
    musician_id: track.musician_id,
    album_cover: track.album_cover,
    musician_name: track.musician_name,
    album_title: track.album_title,
  };
}

export const Route = createFileRoute("/_auth/music/playlist/$id")({
  loader: async ({ context, params }) => {
    const id = parseRouteId(params.id);
    if (id == null) return;
    await context.queryClient.ensureQueryData(playlistDetailsQueryOpts(id));
  },
  component: PlaylistPage,
});

function PlaylistPage() {
  const { id } = Route.useParams();
  // 0 for a malformed id: the request then fails into the error branch
  // below, exactly as an unknown playlist does.
  const playlistId = parseRouteId(id) ?? 0;

  const { data, isLoading, error } = useQuery(
    playlistDetailsQueryOpts(playlistId)
  );

  return (
    <MediaDetailGuard
      id={playlistId}
      noun="playlist"
      back="musicPlaylists"
      isPending={isLoading}
      isError={Boolean(error)}
      data={data}
      payload={data?.error === false ? data.data : null}
      skeleton={<MusicDetailSkeleton variant="playlist" />}
    >
      {loaded => <PlaylistContent playlistId={playlistId} data={loaded} />}
    </MediaDetailGuard>
  );
}

type PlaylistContentProps = {
  playlistId: number;
  data: PlaylistDetailResponseType;
};

function PlaylistContent({ playlistId, data }: PlaylistContentProps) {
  const navigate = Route.useNavigate();
  const queryClient = useQueryClient();
  const audioPlayer = useAudioPlayerActions();
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  // The rest of the playlist downloading behind playback that has already
  // started — a progress hint on the buttons, not a block on using them.
  const [isLoadingRest, setIsLoadingRest] = useState(false);
  const editButtonRef = useRef<HTMLButtonElement | null>(null);
  const deleteButtonRef = useRef<HTMLButtonElement | null>(null);

  const { playlist, track_count, duration, is_owner, can_edit } = data;
  const coverUrl = getMediaImageUrl(unwrapString(playlist.cover_image));
  const { showPoster: showCover, onError: onCoverError } = usePosterFallback(
    coverUrl ?? "",
  );
  const description = unwrapString(playlist.description);

  // React 19 document metadata - dynamic based on playlist
  const pageTitle = `${playlist.name} - Igloo`;
  const pageDescription = `Listen to ${playlist.name} - ${pluralize(track_count, "track")}, ${formatDuration(duration)} in your Igloo playlist.`;

  // Infinite query for tracks
  const {
    data: tracksData,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading: isLoadingTracks,
  } = useInfiniteQuery(playlistTracksInfiniteQueryOpts(playlistId));

  // Flatten all pages into a single array
  const allTracks =
    tracksData?.pages.flatMap((page) =>
      page.error === false ? (page.data?.tracks ?? []) : []
    ) ?? [];

  // Delete playlist mutation
  const deleteMutation = useMutation({
    mutationFn: () => deletePlaylist(playlistId),
    onSuccess: (result) => {
      if (result.error) {
        showActionFailed("delete playlist", result.message);
        return;
      }
      queryClient.invalidateQueries({ queryKey: [PLAYLISTS_KEY] });
      showDeleted("Playlist");
      navigate({ to: "/music", search: MUSIC_PLAYLISTS_TAB_SEARCH });
    },
    onError: () => {
      showActionFailed("delete playlist");
    },
  });

  // Remove track mutation
  const removeTrackMutation = useMutation({
    mutationFn: (trackId: number) =>
      removeTrackFromPlaylist(playlistId, trackId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [PLAYLIST_TRACKS_KEY, playlistId] });
      queryClient.invalidateQueries({ queryKey: [PLAYLISTS_KEY] });
      showRemoved("Track", "from playlist");
    },
    onError: () => {
      showActionFailed("remove track");
    },
  });

  // Reorder tracks mutation
  const reorderMutation = useMutation({
    mutationFn: (trackIds: number[]) =>
      reorderPlaylistTracks(playlistId, trackIds),
    onSuccess: (result) => {
      if (result.error) {
        showActionFailed("reorder tracks", result.message);
        // Refetch to restore original order
        queryClient.invalidateQueries({ queryKey: [PLAYLIST_TRACKS_KEY, playlistId] });
        return;
      }
      // Invalidate to sync with server
      queryClient.invalidateQueries({ queryKey: [PLAYLIST_TRACKS_KEY, playlistId] });
    },
    onError: () => {
      showActionFailed("reorder tracks");
      // Refetch to restore original order
      queryClient.invalidateQueries({ queryKey: [PLAYLIST_TRACKS_KEY, playlistId] });
    },
  });

  // The tracks list is an infinite query, so allTracks only holds the pages the
  // user has scrolled into, while the header buttons promise the playlist's
  // full track_count. Drain the rest in the background: on a long playlist this
  // is dozens of sequential round trips, and making the first note wait on them
  // is worse than starting with what is already here.
  //
  // fetchNextPage resolves with the updated observer result, so the loop reads
  // hasNextPage off that instead of the stale value captured in this closure.
  // It never rejects (TanStack swallows the error, and apiRequest returns an
  // error envelope rather than throwing), so a failed page shows up only as a
  // short result — hence the page-count guard, which stops the loop if a round
  // ever fails to add a page.
  const loadRemainingTracks = async () => {
    let result = await fetchNextPage();
    let pageCount = result.data?.pages.length ?? 0;

    while (result.hasNextPage) {
      result = await fetchNextPage();

      const nextCount = result.data?.pages.length ?? 0;
      if (nextCount <= pageCount) break;
      pageCount = nextCount;
    }

    return (
      result.data?.pages.flatMap((page) =>
        page.error === false ? (page.data?.tracks ?? []) : [],
      ) ?? []
    );
  };

  // playQueue/shuffleQueue instead of playTrack: these header buttons are
  // explicit "start over" entry points and must restart even when the first
  // track is already the current one (playTrack toggles in that case).
  const startPlaylistQueue = async (shuffle: boolean) => {
    const action = shuffle ? "shuffle playlist" : "play playlist";

    if (allTracks.length === 0) {
      showActionFailed(action, "This playlist has no tracks to play yet.");
      return;
    }

    // A playlist may hold the same track at two positions. The player finds the
    // current track with findIndex, so a repeated id would make next/prev jump
    // back to the first copy — dedupe as playTrackFromList does.
    const loaded = dedupeById(allTracks.map(playlistTrackToPlayableData));
    const albumInfo = { cover: coverUrl, title: playlist.name, musician: null };
    const audioTracks = loaded.map(convertToAudioTrack);

    const queueId = shuffle
      ? audioPlayer.shuffleQueue(audioTracks, albumInfo, loaded)
      : audioPlayer.playQueue(audioTracks, albumInfo, loaded);

    if (queueId === null || !hasNextPage) return;

    setIsLoadingRest(true);
    const tracks = await loadRemainingTracks();
    setIsLoadingRest(false);

    // reshuffleTail: the button said "Shuffle all N", so the tracks that only
    // arrived now have to be mixed through the part the user has not reached
    // yet, not tacked onto the end in a block.
    audioPlayer.extendQueue(
      dedupeById(tracks.map(playlistTrackToPlayableData)),
      queueId,
      { reshuffleTail: shuffle },
    );

    if (tracks.length < track_count) {
      showActionFailed(
        action,
        `Only ${tracks.length} of ${track_count} tracks could be loaded.`,
      );
    }
  };

  const handlePlayAll = () => {
    void startPlaylistQueue(false);
  };

  const handleShuffle = () => {
    void startPlaylistQueue(true);
  };

  const handleDeletePlaylist = () => {
    setShowDeleteDialog(true);
  };

  return (
    <article
      className={cn(
        DETAIL_PAGE_CONTENT_ENTER_CLASS,
        "w-full min-w-0 overflow-x-hidden pb-6 sm:pb-10",
      )}
      aria-labelledby="playlist-name"
    >
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      {/* Header section */}
      <header className="mb-8 flex flex-col gap-6 sm:mb-10 sm:gap-8 lg:flex-row">
        {/* Playlist cover */}
        <figure className="mx-auto shrink-0 lg:mx-0">
          <div className="aspect-square w-40 overflow-hidden rounded-xl border border-primary/20 bg-muted shadow-2xl shadow-primary/10 sm:w-48 lg:w-56 xl:w-64">
            {showCover ? (
              <img
                src={coverUrl ?? ""}
                alt={playlist.name}
                className="size-full object-cover"
                onError={onCoverError}
              />
            ) : (
              <div className="flex size-full items-center justify-center bg-linear-to-br from-muted via-muted to-primary/30">
                <ListMusic className="size-16 text-primary/20" aria-hidden="true" />
              </div>
            )}
          </div>
        </figure>

        {/* Playlist info */}
        <div className="flex max-w-full min-w-0 flex-1 flex-col overflow-hidden text-center lg:text-left">
          {/* Name */}
          <h1
            id="playlist-name"
            className="text-2xl font-bold text-foreground sm:truncate sm:text-3xl md:text-4xl lg:text-5xl"
            title={playlist.name}
          >
            {playlist.name}
          </h1>

          {/* Description */}
          {description && (
            <p className="mt-2 line-clamp-2 text-sm text-muted-foreground sm:mt-3 sm:line-clamp-none sm:text-base md:max-w-2xl">
              {description}
            </p>
          )}

          {/* Stats row */}
          <ul
            className="mt-4 flex flex-wrap items-center justify-center gap-x-3 gap-y-2 text-xs text-muted-foreground sm:gap-x-4 sm:text-sm lg:justify-start lg:text-base"
            aria-label="Playlist statistics"
          >
            <li className="flex items-center gap-1.5">
              <Music className="size-4 text-muted-foreground" aria-hidden="true" />
              <span>{pluralize(track_count, "track")}</span>
            </li>
            <li className="flex items-center gap-1.5">
              <Clock className="size-4 text-muted-foreground" aria-hidden="true" />
              <span>{formatDuration(duration)}</span>
            </li>
            {is_owner && (
              <li className="flex items-center gap-1.5">
                <User className="size-4 text-primary" aria-hidden="true" />
                <span className="text-primary">Owner</span>
              </li>
            )}
          </ul>

          {/* Play buttons. Deliberately never disabled: playback starts from the
              pages already loaded, and the spinner only reports the rest
              arriving behind it. A disabled media control is also unreachable
              under iOS VoiceOver. `size` re-declares rounded-md, so both
              buttons re-assert rounded-full to stay a matching pair. */}
          {track_count > 0 && (
            <div className="mt-5 flex flex-col justify-center gap-2 sm:mt-6 sm:flex-row sm:gap-3 lg:justify-start">
              <Button
                type="button"
                variant="accent-pill"
                size="lg"
                onClick={handlePlayAll}
                className="w-full rounded-full font-semibold shadow-lg shadow-primary/20 sm:w-auto"
                aria-label={`Play all ${pluralize(track_count, "track")}`}
              >
                {isLoadingRest ? (
                  <Spinner className="size-4" />
                ) : (
                  <Play className="size-4 fill-current" aria-hidden="true" />
                )}
                Play All
              </Button>
              <Button
                type="button"
                variant="outline"
                size="lg"
                onClick={handleShuffle}
                className="w-full rounded-full font-semibold sm:w-auto"
                aria-label={`Shuffle all ${pluralize(track_count, "track")}`}
              >
                {isLoadingRest ? (
                  <Spinner className="size-4" />
                ) : (
                  <Shuffle className="size-4" aria-hidden="true" />
                )}
                Shuffle
              </Button>
            </div>
          )}

          {/* Edit and Delete buttons for owner */}
          {is_owner && (
            <div className="mt-4 flex flex-wrap justify-center gap-3 sm:gap-4 lg:justify-start">
              <button
                type="button"
                ref={editButtonRef}
                onClick={() => setShowEditDialog(true)}
                className={cn(
                  MOTION_MICRO_COLORS_CLASS,
                  FOCUS_VISIBLE_RING_CLASS,
                  "inline-flex items-center gap-1.5 rounded-sm text-xs text-muted-foreground hover:text-primary focus-visible:text-primary sm:gap-2 sm:text-sm",
                )}
                aria-label="Edit playlist"
              >
                <Pencil className="size-4" aria-hidden="true" />
                <span>Edit</span>
                <span className="hidden sm:inline">Details</span>
              </button>
              <button
                type="button"
                ref={deleteButtonRef}
                onClick={handleDeletePlaylist}
                disabled={deleteMutation.isPending}
                className={cn(
                  MOTION_MICRO_COLORS_CLASS,
                  FOCUS_VISIBLE_RING_CLASS,
                  "inline-flex items-center gap-1.5 rounded-sm text-xs text-muted-foreground hover:text-destructive focus-visible:text-destructive disabled:opacity-50 sm:gap-2 sm:text-sm",
                )}
                aria-label="Delete playlist"
              >
                {deleteMutation.isPending ? (
                  <Spinner className="size-4" />
                ) : (
                  <Trash2 className="size-4" aria-hidden="true" />
                )}
                <span>Delete</span>
                <span className="hidden sm:inline">Playlist</span>
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Tracks section */}
      <section aria-labelledby="tracks-heading">
        <h2
          id="tracks-heading"
          className="mb-4 flex items-center gap-2 text-xl font-semibold text-foreground"
        >
          <List className="size-5 text-primary" aria-hidden="true" />
          Tracks
        </h2>

        {isLoadingTracks ? (
          <div className="flex justify-center py-12">
            <Spinner className="size-8 text-primary" />
          </div>
        ) : allTracks.length === 0 ? (
          <div className="rounded-xl border border-primary/10 bg-muted/30 py-12 text-center">
            <div className="mx-auto mb-4 flex size-16 items-center justify-center rounded-full bg-linear-to-br from-muted via-muted to-primary/40">
              <Music className="size-6 text-primary/40" aria-hidden="true" />
            </div>
            <p className="text-muted-foreground">No tracks in this playlist yet.</p>
            <p className="mt-2 text-sm text-muted-foreground">
              Add tracks from your library to get started.
            </p>
          </div>
        ) : (
          <PlaylistTracksList
            playlistId={playlistId}
            tracks={allTracks}
            canEdit={can_edit}
            hasNextPage={hasNextPage}
            isFetchingNextPage={isFetchingNextPage}
            fetchNextPage={fetchNextPage}
            onRemoveTrack={(trackId) => removeTrackMutation.mutate(trackId)}
            onReorderTracks={(trackIds) => reorderMutation.mutate(trackIds)}
          />
        )}
      </section>

      <MusicDetailBackNav
        tab="playlists"
        label="Back to Playlists"
        className="mt-8"
      />

      {/* Edit Playlist Dialog */}
      {is_owner && (
        <PlaylistFormDialog
          mode="edit"
          open={showEditDialog}
          onOpenChange={setShowEditDialog}
          playlist={playlist}
          restoreFocusRef={editButtonRef}
        />
      )}

      {/* Delete Playlist Confirmation Dialog */}
      <ConfirmDialog
        open={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        title="Delete playlist"
        description={
          <>
            Are you sure you want to delete &ldquo;{playlist.name}&rdquo;? This
            action cannot be undone.
          </>
        }
        confirmLabel="Delete"
        pending={deleteMutation.isPending}
        restoreFocusRef={deleteButtonRef}
        onConfirm={() => deleteMutation.mutate()}
      />
    </article>
  );
}

type PlaylistTracksListProps = {
  playlistId: number;
  tracks: PlaylistTrackType[];
  canEdit: boolean;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => Promise<unknown>;
  onRemoveTrack: (trackId: number) => void;
  onReorderTracks: (trackIds: number[]) => void;
};

// Threshold for using draggable list vs virtualized list
// Using draggable list for smaller playlists for better UX
const DRAGGABLE_TRACK_LIMIT = 200;

function PlaylistTracksList({
  playlistId,
  tracks,
  canEdit,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
  onRemoveTrack,
  onReorderTracks,
}: PlaylistTracksListProps) {
  const audioPlayer = useAudioPlayerActions();

  // Local optimistic order override keyed by track ids.
  const [optimisticTrackIds, setOptimisticTrackIds] = useState<number[] | null>(null);
  const trackMap = new Map(tracks.map((track) => [track.id, track]));
  const orderedTracks =
    optimisticTrackIds !== null &&
    optimisticTrackIds.length === tracks.length &&
    optimisticTrackIds.every((trackId) => trackMap.has(trackId))
      ? optimisticTrackIds
          .map((trackId) => trackMap.get(trackId))
          .filter((track): track is PlaylistTrackType => track !== undefined)
      : tracks;

  // playTrackFromList, not playTrack: it keeps the same toggle-on-repeat-click
  // contract but queues the raw rows, so each track keeps its own cover and
  // artist as the queue advances instead of inheriting the playlist's.
  const handlePlayTrack = (track: PlaylistTrackType) => {
    audioPlayer.playTrackFromList(
      orderedTracks.map(playlistTrackToPlayableData),
      track.id,
    );
  };

  // Handle reorder with optimistic update
  const handleReorder = (newTrackIds: number[]) => {
    // Optimistic update
    setOptimisticTrackIds(newTrackIds);

    // Call the API
    onReorderTracks(newTrackIds);
  };

  // Use draggable list for smaller playlists that user can edit
  const useDraggableList = canEdit && orderedTracks.length <= DRAGGABLE_TRACK_LIMIT && !hasNextPage;

  if (useDraggableList) {
    return (
      <>
        <Suspense
          fallback={
            <div className="flex justify-center rounded-xl border border-primary/10 bg-muted/30 py-12">
              <Spinner className="size-8 text-primary" />
            </div>
          }
        >
          <DraggableTrackList
            tracks={orderedTracks}
            canEdit={canEdit}
            onReorder={handleReorder}
            onPlayTrack={handlePlayTrack}
            onRemoveTrack={onRemoveTrack}
          />
        </Suspense>
        {isFetchingNextPage && (
          <div className="flex justify-center py-4">
            <Spinner className="size-6 text-primary" />
          </div>
        )}
      </>
    );
  }

  // Use virtualized list for large playlists or read-only
  return (
    <VirtualizedPlaylistTracksList
      playlistId={playlistId}
      tracks={orderedTracks}
      canEdit={canEdit}
      hasNextPage={hasNextPage}
      isFetchingNextPage={isFetchingNextPage}
      fetchNextPage={fetchNextPage}
      onPlayTrack={handlePlayTrack}
      onRemoveTrack={onRemoveTrack}
    />
  );
}

type VirtualizedPlaylistTracksListProps = {
  tracks: PlaylistTrackType[];
  playlistId: number;
  canEdit: boolean;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => Promise<unknown>;
  onPlayTrack: (track: PlaylistTrackType) => void;
  onRemoveTrack: (trackId: number) => void;
};

function VirtualizedPlaylistTracksList({
  tracks,
  playlistId,
  canEdit,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
  onPlayTrack,
  onRemoveTrack,
}: VirtualizedPlaylistTracksListProps) {
  "use no memo";

  const matchTrackPlayback = useTrackPlaybackMatcher();
  const { listRef, scrollMargin } = useWindowScrollMargin<HTMLDivElement>();

  const onChange = useVirtualizedInfiniteLoader({
    itemCount: tracks.length,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
    scopeKey: playlistId,
  });

  const virtualizer = useWindowVirtualizer({
    count: tracks.length,
    estimateSize: () => VIRTUAL_LIST_TRACK_HEIGHT,
    overscan: 5,
    scrollMargin,
    onChange,
  });

  const renderedVirtualItems = virtualizer.getVirtualItems();

  useEffect(() => {
    virtualizer.measure();
  }, [tracks.length, scrollMargin, virtualizer]);

  return (
    <div
      ref={listRef}
      className="overflow-hidden rounded-xl border border-primary/10 bg-muted/30"
    >
      <div
        style={{
          height: `${virtualizer.getTotalSize()}px`,
          width: "100%",
          position: "relative",
        }}
      >
        {renderedVirtualItems.map((virtualRow) => {
          const track = tracks[virtualRow.index];
          if (!track) return null;

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
              <TrackItem
                {...trackRowProps(track)}
                variant="playlist"
                {...matchTrackPlayback(track.id)}
                onPlay={() => onPlayTrack(track)}
                showActionsMenu
                canRemoveFromPlaylist={canEdit}
                onRemoveFromPlaylist={() => onRemoveTrack(track.id)}
              />
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
