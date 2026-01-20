import { useState } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  AlertCircle,
  Disc3,
  Calendar,
  Music,
  Clock,
  Play,
  Shuffle,
  MoreHorizontal,
  Trash2,
  Loader2,
  ListOrdered,
  ArrowLeft,
  User,
} from "lucide-react";
import { albumDetailsQueryOpts } from "@/lib/query-opts";
import { deleteAlbum } from "@/lib/api";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useAudioPlayer } from "@/hooks/useAudioPlayer";
import TrackItem from "@/components/TrackItem";
import { formatDate, formatDuration } from "@/lib/format";
import type {
  AlbumDetailsResponseType,
  ArtistType,
  TrackGenreType,
  TrackType,
} from "@/types";

export const Route = createFileRoute("/_auth/music/album/$id")({
  loader: async ({ context, params }) => {
    const albumId = parseInt(params.id, 10);
    const data = await context.queryClient.ensureQueryData(
      albumDetailsQueryOpts(albumId)
    );

    return data;
  },
  component: AlbumDetailsPage,
});

function AlbumDetailsPage() {
  const data = Route.useLoaderData();

  if (data.error) {
    return (
      <Alert
        variant='destructive'
        className='border-red-500/20 bg-red-500/10 text-red-400'
      >
        <AlertCircle className="size-4" aria-hidden="true" />
        <AlertTitle>Error</AlertTitle>
        <AlertDescription>
          {data.message ||
            "Failed to load album details. Please try again later."}
        </AlertDescription>
      </Alert>
    );
  }

  if (!data.data?.album) {
    return (
      <div className='py-12 text-center'>
        <h2 className='text-xl font-semibold text-slate-300'>
          Album not found
        </h2>
      </div>
    );
  }

  return <AlbumDetailsContent {...data.data} />;
}

function AlbumDetailsContent({
  album,
  tracks,
  artists,
  track_genres,
  album_genres,
  total_duration,
}: AlbumDetailsResponseType) {
  const audioPlayer = useAudioPlayer();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  const coverUrl = album.cover.Valid ? album.cover.String : null;
  const releaseYear = album.year.Valid ? album.year.Int64 : null;
  const musicianName = album.musician.Valid ? album.musician.String : null;

  const handleDeleteAlbum = async () => {
    setIsDeleting(true);

    try {
      const result = await deleteAlbum(album.id);

      if (result.error) {
        toast.error("Delete failed", {
          description: result.message || "Unable to delete album. Please try again.",
        });
        return;
      }

      toast.success("Album deleted", {
        description: `"${album.title}" has been removed from your library.`,
      });

      // Invalidate album queries to refresh the list
      queryClient.invalidateQueries({ queryKey: ["albums"] });
      queryClient.invalidateQueries({ queryKey: ["music-stats"] });

      // Navigate back to music page
      navigate({ to: "/music", search: { tab: "albums" } });
    } catch (error) {
      console.error("Failed to delete album:", error);
      toast.error("Delete failed", {
        description: "An unexpected error occurred. Please try again.",
      });
    } finally {
      setIsDeleting(false);
      setIsDeleteDialogOpen(false);
    }
  };

  // Build a map of track_id -> genre tags for easy lookup
  const trackGenreMap = new Map<number, string[]>();
  track_genres.forEach((tg: TrackGenreType) => {
    const existing = trackGenreMap.get(tg.track_id) || [];
    existing.push(tg.tag);
    trackGenreMap.set(tg.track_id, existing);
  });

  // Group tracks by disc
  const tracksByDisc = tracks.reduce(
    (acc, track) => {
      const disc = track.disc || 1;
      if (!acc[disc]) acc[disc] = [];
      acc[disc].push(track);
      return acc;
    },
    {} as Record<number, TrackType[]>
  );

  const discNumbers = Object.keys(tracksByDisc)
    .map(Number)
    .sort((a, b) => a - b);
  const hasMultipleDiscs = discNumbers.length > 1;

  // Check if a specific track is currently playing
  const isTrackPlaying = (track: TrackType) => {
    return audioPlayer.currentTrack?.id === track.id && audioPlayer.isPlaying;
  };

  // Handle playing/pausing a track
  const handleToggleTrack = (track: TrackType) => {
    if (audioPlayer.currentTrack?.id === track.id) {
      // Toggle play/pause for the current track
      audioPlayer.togglePlay();
    } else {
      // Play a new track
      audioPlayer.playTrack(track, tracks, {
        cover: album.cover?.Valid ? album.cover.String : null,
        title: album.title,
        musician: musicianName,
      });
    }
  };

  // Handle playing the album from the beginning
  const handlePlayAlbum = () => {
    audioPlayer.playAlbum(tracks, {
      cover: album.cover?.Valid ? album.cover.String : null,
      title: album.title,
      musician: musicianName,
    });
  };

  // Handle shuffle play
  const handleShufflePlay = () => {
    audioPlayer.shuffleAlbum(tracks, {
      cover: album.cover?.Valid ? album.cover.String : null,
      title: album.title,
      musician: musicianName,
    });
  };

  return (
    <article aria-labelledby='album-title'>
      {/* Header section with cover and album info */}
      <header className='mb-10 flex flex-col gap-8 md:flex-row'>
        {/* Album cover */}
        <figure className='mx-auto shrink-0 md:mx-0'>
          <div className='w-64 overflow-hidden rounded-xl border border-amber-500/20 shadow-2xl shadow-amber-500/10 md:w-72 lg:w-80'>
            {coverUrl ? (
              <img
                src={coverUrl}
                alt={`Album cover for ${album.title}`}
                className='aspect-square w-full object-cover'
              />
            ) : (
              <div
                className='flex aspect-square w-full items-center justify-center bg-slate-800'
                role='img'
                aria-label='No cover available'
              >
                <Disc3 className="size-16 text-slate-600" aria-hidden="true" />
              </div>
            )}
          </div>
        </figure>

        {/* Album info */}
        <div className='flex flex-1 flex-col'>
          {/* Title */}
          <h1
            id='album-title'
            className='text-3xl font-bold text-white md:text-4xl lg:text-5xl'
          >
            {album.title}
          </h1>

          {/* Artist name */}
          {musicianName && (
            <p className='mt-2 text-xl font-medium text-amber-400'>
              {musicianName}
            </p>
          )}

          {/* Meta info row */}
          <ul
            className='mt-4 flex flex-wrap items-center gap-4 text-slate-400'
            aria-label='Album details'
          >
            {(album.release_date.Valid || releaseYear) && (
              <li className='flex items-center gap-1.5'>
                <Calendar className="size-4 text-slate-500" aria-hidden="true" />
                <time
                  dateTime={album.release_date.String || String(releaseYear)}
                >
                  {album.release_date.Valid
                    ? formatDate(album.release_date.String)
                    : releaseYear}
                </time>
              </li>
            )}
            <li className='flex items-center gap-1.5'>
              <Music className="size-4 text-slate-500" aria-hidden="true" />
              <span>
                {tracks.length} {tracks.length === 1 ? "track" : "tracks"}
              </span>
            </li>
            <li className='flex items-center gap-1.5'>
              <Clock className="size-4 text-slate-500" aria-hidden="true" />
              <span>{formatDuration(total_duration)}</span>
            </li>
            {album.spotify_popularity.Valid && (
              <li
                className='flex items-center gap-1.5'
                aria-label='Spotify Popularity'
              >
                <svg className="size-4 text-green-500" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.419 1.56-.299.421-1.02.599-1.559.3z"/>
                </svg>
                <span>
                  <span className='text-slate-500'>Popularity:</span>{" "}
                  <span className='font-medium text-green-400'>
                    {Math.round(album.spotify_popularity.Float64)}
                  </span>
                </span>
              </li>
            )}
          </ul>

          {/* Genre tags */}
          {album_genres.length > 0 && (
            <ul
              className='mt-4 flex flex-wrap gap-2'
              aria-label={`Genres: ${album_genres.join(", ")}`}
            >
              {album_genres.map(genre => (
                <li
                  key={genre}
                  className='rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm'
                >
                  {genre}
                </li>
              ))}
            </ul>
          )}

          {/* Play Album, Shuffle, and More buttons */}
          <div className='mt-6 flex flex-wrap gap-3'>
            <button
              onClick={handlePlayAlbum}
              className='inline-flex items-center gap-2 rounded-full bg-amber-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400'
            >
              <Play className="size-4 fill-current" aria-hidden="true" />
              Play Album
            </button>
            <button
              onClick={handleShufflePlay}
              className='inline-flex items-center gap-2 rounded-full border border-slate-600 bg-slate-700 px-6 py-3 font-semibold text-white transition-colors hover:bg-slate-600'
              aria-label='Shuffle play album'
            >
              <Shuffle className="size-4" aria-hidden="true" />
              Shuffle
            </button>

            {/* More options dropdown */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  className='inline-flex items-center gap-2 rounded-full border border-slate-600 bg-slate-700 px-4 py-3 font-semibold text-white transition-colors hover:bg-slate-600'
                  aria-label='More options'
                >
                  <MoreHorizontal className="size-4" aria-hidden="true" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align='end'
                className='border-slate-700 bg-slate-800'
              >
                <DropdownMenuItem
                  onClick={() => setIsDeleteDialogOpen(true)}
                  className='cursor-pointer text-red-400 focus:bg-red-500/10 focus:text-red-400'
                >
                  <Trash2 className="mr-2 size-4" aria-hidden="true" />
                  Delete Album
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          {/* Delete confirmation dialog */}
          <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
            <DialogContent className='border-slate-700 bg-slate-900 text-white'>
              <DialogHeader>
                <DialogTitle className='text-white'>Delete Album</DialogTitle>
                <DialogDescription className='text-slate-400'>
                  Are you sure you want to delete "{album.title}"? This action
                  cannot be undone and will permanently remove:
                </DialogDescription>
              </DialogHeader>

              <ul className='ml-4 list-disc space-y-1 text-sm text-slate-300'>
                <li>The album and all its metadata</li>
                <li>
                  All {tracks.length} {tracks.length === 1 ? "track" : "tracks"}{" "}
                  associated with this album
                </li>
                <li>All genre and artist associations</li>
              </ul>

              <DialogFooter className='gap-2 sm:gap-0'>
                <button
                  onClick={() => setIsDeleteDialogOpen(false)}
                  disabled={isDeleting}
                  className='rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 font-medium text-white transition-colors hover:bg-slate-600 disabled:opacity-50'
                >
                  Cancel
                </button>
                <button
                  onClick={handleDeleteAlbum}
                  disabled={isDeleting}
                  className='rounded-lg bg-red-600 px-4 py-2 font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50'
                >
                  {isDeleting ? (
                    <>
                      <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
                      Deleting...
                    </>
                  ) : (
                    "Delete Album"
                  )}
                </button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Artists section */}
          {artists.length > 0 && (
            <section className='mt-6' aria-labelledby='artists-heading'>
              <h2
                id='artists-heading'
                className='mb-3 text-sm font-semibold tracking-wide text-slate-400 uppercase'
              >
                {artists.length === 1 ? "Artist" : "Artists"}
              </h2>
              <div className='flex flex-wrap gap-3'>
                {artists.map((artist: ArtistType) => (
                  <ArtistBadge key={artist.id} artist={artist} />
                ))}
              </div>
            </section>
          )}
        </div>
      </header>

      {/* Track list */}
      <section aria-labelledby='tracklist-heading'>
        <h2
          id='tracklist-heading'
          className='mb-4 flex items-center gap-2 text-xl font-semibold text-white'
        >
          <ListOrdered className="size-5 text-amber-400" aria-hidden="true" />
          Track List
        </h2>

        <div className='overflow-hidden rounded-xl border border-amber-500/10 bg-slate-800/30'>
          {discNumbers.map(discNum => (
            <div key={discNum}>
              {hasMultipleDiscs && (
                <div className='border-b border-slate-700/50 bg-slate-800/50 px-4 py-2'>
                  <span className='flex items-center gap-2 text-sm font-medium text-slate-400'>
                    <Disc3 className="size-4 text-amber-400/70" aria-hidden="true" />
                    Disc {discNum}
                  </span>
                </div>
              )}
              <div className='divide-y divide-slate-700/30'>
                {tracksByDisc[discNum].map((track: TrackType) => (
                  <TrackItem
                    key={track.id}
                    id={track.id}
                    title={track.title}
                    duration={track.duration}
                    trackIndex={track.track_index}
                    genres={trackGenreMap.get(track.id) || []}
                    variant='album'
                    isPlaying={isTrackPlaying(track)}
                    isCurrentTrack={audioPlayer.currentTrack?.id === track.id}
                    onPlay={() => handleToggleTrack(track)}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Album metadata */}
      <section
        className='mt-10 rounded-xl border border-amber-500/10 bg-slate-800/30 p-4'
        aria-labelledby='details-heading'
      >
        <h2
          id='details-heading'
          className='mb-4 text-lg font-semibold text-white'
        >
          Album Details
        </h2>
        <dl className='grid grid-cols-2 gap-6 text-sm sm:grid-cols-3 lg:grid-cols-4'>
          {album.release_date.Valid && (
            <div>
              <dt className='font-semibold tracking-wide text-amber-300/70 uppercase'>
                Release Date
              </dt>
              <dd className='mt-1 text-white'>
                {formatDate(album.release_date.String)}
              </dd>
            </div>
          )}
          <div>
            <dt className='font-semibold tracking-wide text-amber-300/70 uppercase'>
              Total Tracks
            </dt>
            <dd className='mt-1 text-white'>{tracks.length}</dd>
          </div>
          <div>
            <dt className='font-semibold tracking-wide text-amber-300/70 uppercase'>
              Total Duration
            </dt>
            <dd className='mt-1 text-white'>
              {formatDuration(total_duration)}
            </dd>
          </div>
        </dl>
      </section>

      {/* Back link */}
      <div className='mt-8'>
        <Link
          to='/'
          className='inline-flex items-center gap-2 text-slate-400 transition-colors hover:text-amber-400'
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          Back to Home
        </Link>
      </div>
    </article>
  );
}

function ArtistBadge({ artist }: { artist: ArtistType }) {
  return (
    <div className='flex items-center gap-2 rounded-full border border-slate-700/50 bg-slate-800/60 px-3 py-1.5 transition-colors hover:border-amber-500/30'>
      {artist.thumb.Valid ? (
        <img
          src={artist.thumb.String}
          alt=''
          className='h-6 w-6 rounded-full object-cover'
        />
      ) : (
        <div className='flex h-6 w-6 items-center justify-center rounded-full bg-slate-700'>
          <User className="size-3 text-slate-500" aria-hidden="true" />
        </div>
      )}
      <span className='text-sm font-medium text-white'>{artist.name}</span>
    </div>
  );
}

