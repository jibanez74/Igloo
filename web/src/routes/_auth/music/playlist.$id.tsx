import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMutation } from "@tanstack/react-query";
import {
  showDeleted,
  showRemoved,
  showActionFailed,
} from "@/lib/toast-helpers";
import {
  AlertCircle,
  ListMusic,
  Music,
  Clock,
  User,
  Play,
  Shuffle,
  Pencil,
  Trash2,
  List,
  ArrowLeft,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import TrackItem from "@/components/TrackItem";
import EditPlaylistDialog from "@/components/EditPlaylistDialog";
import ConfirmDialog from "@/components/ConfirmDialog";

// Lazy load DraggableTrackList to reduce initial bundle size
// This component includes the heavy @dnd-kit packages
const DraggableTrackList = lazy(() => import("@/components/DraggableTrackList"));
import {
  playlistDetailsQueryOpts,
  playlistTracksInfiniteQueryOpts,
} from "@/lib/query-opts";
import { unwrapString, unwrapInt, unwrapStringOrUndefined } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { deletePlaylist, removeTrackFromPlaylist, reorderPlaylistTracks } from "@/lib/api";
import { convertToAudioTrack, shuffleArray } from "@/lib/audio-utils";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useAudioPlayerState } from "@/hooks/useAudioPlayerState";
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { useVirtualizedInfiniteLoader } from "@/hooks/useVirtualizedInfiniteLoader";
import { useWindowScrollMargin } from "@/hooks/useWindowScrollMargin";
import { formatDuration } from "@/lib/format";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
  VIRTUAL_LIST_TRACK_HEIGHT,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { PlaylistTrackType } from "@/types";

export const Route = createFileRoute("/_auth/music/playlist/$id")({
  loader: async ({ context, params }) => {
    const id = parseInt(params.id, 10);
    if (isNaN(id)) return;
    await context.queryClient.ensureQueryData(playlistDetailsQueryOpts(id));
  },
  component: PlaylistPage,
});

function PlaylistPage() {
  const { id } = Route.useParams();
  const playlistId = parseInt(id, 10);

  const { data, isLoading, error } = useQuery(
    playlistDetailsQueryOpts(playlistId)
  );

  if (isLoading) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <Spinner className="size-10 text-primary" />
      </div>
    );
  }

  if (error || !data || data.error) {
    return (
      <div className="py-12 text-center text-muted-foreground">
        <AlertCircle className="mx-auto mb-4 size-10" aria-hidden="true" />
        <p>Failed to load playlist. Please try again.</p>
        <Link
          to="/music"
          search={{ tab: "playlists" }}
          className="mt-4 inline-block text-primary hover:underline"
        >
          Back to Playlists
        </Link>
      </div>
    );
  }

  return <PlaylistContent playlistId={playlistId} data={data.data} />;
}

type PlaylistContentProps = {
  playlistId: number;
  data: {
    playlist: {
      id: number;
      user_id: number;
      name: string;
      description: { String: string; Valid: boolean };
      cover_image: { String: string; Valid: boolean };
      is_public: boolean;
      folder_id: { Int64: number; Valid: boolean };
      created_at: string;
      updated_at: string;
    };
    track_count: number;
    duration: number;
    is_owner: boolean;
    can_edit: boolean;
    collaborators: unknown[] | null;
  };
};

function PlaylistContent({ playlistId, data }: PlaylistContentProps) {
  const navigate = Route.useNavigate();
  const queryClient = useQueryClient();
  const audioPlayer = useAudioPlayerActions();
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const editButtonRef = useRef<HTMLButtonElement | null>(null);
  const deleteButtonRef = useRef<HTMLButtonElement | null>(null);

  const { playlist, track_count, duration, is_owner, can_edit } = data;
  const coverUrl = getMediaImageUrl(
    playlist.cover_image?.Valid ? playlist.cover_image.String : null
  );
  const description = playlist.description?.Valid
    ? playlist.description.String
    : null;

  // React 19 document metadata - dynamic based on playlist
  const pageTitle = `${playlist.name} - Igloo`;
  const pageDescription = `Listen to ${playlist.name} - ${track_count} tracks, ${formatDuration(duration)} in your Igloo playlist.`;

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
      navigate({ to: "/music", search: { tab: "playlists" } });
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

  const handlePlayAll = () => {
    if (!allTracks.length) return;
    const audioTracks = allTracks.map((track) =>
      convertToAudioTrack({
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
      })
    );
    audioPlayer.playTrack(audioTracks[0], audioTracks, {
      cover: coverUrl,
      title: playlist.name,
      musician: null,
    });
  };

  const handleShuffle = () => {
    if (!allTracks.length) return;
    const shuffled = shuffleArray(allTracks);
    const audioTracks = shuffled.map((track) =>
      convertToAudioTrack({
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
      })
    );
    audioPlayer.playTrack(audioTracks[0], audioTracks, {
      cover: coverUrl,
      title: playlist.name,
      musician: null,
    });
  };

  const handleDeletePlaylist = () => {
    setShowDeleteDialog(true);
  };

  // Page announcement for screen readers
  const pageAnnouncement = `${playlist.name}. ${track_count} ${track_count === 1 ? "track" : "tracks"}. Total duration: ${formatDuration(duration)}.`;

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

      {/* Screen reader announcement */}
      <span
        tabIndex={0}
        className="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-50 focus:rounded-md focus:bg-muted focus:px-4 focus:py-2 focus:text-foreground focus:ring-2 focus:ring-ring focus:outline-none"
        aria-label={pageAnnouncement}
      >
        {playlist.name} - {track_count} tracks
      </span>

      {/* Header section */}
      <header className="mb-8 flex flex-col gap-6 sm:mb-10 sm:gap-8 lg:flex-row">
        {/* Playlist cover */}
        <figure className="mx-auto shrink-0 lg:mx-0">
          <div className="aspect-square w-40 overflow-hidden rounded-xl border border-primary/20 bg-muted shadow-2xl shadow-primary/10 sm:w-48 lg:w-56 xl:w-64">
            {coverUrl ? (
              <img
                src={coverUrl}
                alt={playlist.name}
                className="size-full object-cover"
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
              <span>
                {track_count} {track_count === 1 ? "track" : "tracks"}
              </span>
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

          {/* Play buttons */}
          {track_count > 0 && (
            <div className="mt-5 flex flex-col justify-center gap-2 sm:mt-6 sm:flex-row sm:gap-3 lg:justify-start">
              <button
                type="button"
                onClick={handlePlayAll}
                className={cn(
                  MOTION_MICRO_COLORS_CLASS,
                  "inline-flex items-center justify-center gap-2 rounded-full bg-primary px-5 py-2.5 text-sm font-semibold text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none sm:px-6 sm:py-3 sm:text-base",
                )}
                aria-label={`Play all ${track_count} tracks`}
              >
                <Play className="size-4 fill-current" aria-hidden="true" />
                Play All
              </button>
              <button
                type="button"
                onClick={handleShuffle}
                className={cn(
                  MOTION_MICRO_COLORS_CLASS,
                  "inline-flex items-center justify-center gap-2 rounded-full border border-border bg-accent px-5 py-2.5 text-sm font-semibold text-foreground hover:bg-accent focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none sm:px-6 sm:py-3 sm:text-base",
                )}
                aria-label={`Shuffle all ${track_count} tracks`}
              >
                <Shuffle className="size-4" aria-hidden="true" />
                Shuffle
              </button>
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
                  "inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-primary focus:text-primary focus:outline-none sm:gap-2 sm:text-sm",
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
                  "inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-destructive focus:text-destructive focus:outline-none disabled:opacity-50 sm:gap-2 sm:text-sm",
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
            playlistName={playlist.name}
            coverUrl={coverUrl}
          />
        )}
      </section>

      {/* Back navigation */}
      <nav className="mt-8" aria-label="Page navigation">
        <Link
          to="/music"
          search={{ tab: "playlists" }}
          className={cn(
            MOTION_MICRO_COLORS_CLASS,
            "inline-flex items-center gap-2 text-muted-foreground hover:text-foreground focus:text-primary focus:ring-2 focus:ring-ring focus:outline-none",
          )}
          aria-label="Back to Playlists"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          Back to Playlists
        </Link>
      </nav>

      {/* Edit Playlist Dialog */}
      {is_owner && (
        <EditPlaylistDialog
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
  playlistName: string;
  coverUrl: string | null;
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
  playlistName,
  coverUrl,
}: PlaylistTracksListProps) {
  const audioPlayer = useAudioPlayerActions();
  const audioPlayerState = useAudioPlayerState();

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

  const handlePlayTrack = (track: PlaylistTrackType) => {
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

    // Create playlist of all tracks for continuous playback
    const allAudioTracks = orderedTracks.map((t) =>
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
      cover: coverUrl,
      title: playlistName,
      musician: null,
    });
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
            playlistId={playlistId}
            playlistName={playlistName}
            coverUrl={coverUrl}
            canEdit={canEdit}
            onReorder={handleReorder}
            onPlayTrack={handlePlayTrack}
            onRemoveTrack={onRemoveTrack}
            currentTrackId={audioPlayerState.currentTrack?.id}
            isPlaying={audioPlayerState.isPlaying}
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
      tracks={orderedTracks}
      playlistId={playlistId}
      canEdit={canEdit}
      hasNextPage={hasNextPage}
      isFetchingNextPage={isFetchingNextPage}
      fetchNextPage={fetchNextPage}
      onPlayTrack={handlePlayTrack}
      onRemoveTrack={onRemoveTrack}
      currentTrackId={audioPlayerState.currentTrack?.id}
      isPlaying={audioPlayerState.isPlaying}
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
  currentTrackId: number | undefined;
  isPlaying: boolean;
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
  currentTrackId,
  isPlaying,
}: VirtualizedPlaylistTracksListProps) {
  "use no memo";

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
                id={track.id}
                title={track.title}
                duration={track.duration}
                subtitle={unwrapString(track.musician_name) ?? "Unknown Artist"}
                albumId={unwrapInt(track.album_id)}
                albumTitle={unwrapStringOrUndefined(track.album_title)}
                musicianId={unwrapInt(track.musician_id)}
                musicianName={unwrapStringOrUndefined(track.musician_name)}
                variant="playlist"
                isPlaying={currentTrackId === track.id && isPlaying}
                isCurrentTrack={currentTrackId === track.id}
                onPlay={() => onPlayTrack(track)}
                showActionsMenu
                playlistId={playlistId}
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
