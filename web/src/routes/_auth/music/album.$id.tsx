import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { showDeleted, showActionFailed } from "@/lib/toast-helpers";
import {
  Disc3,
  Calendar,
  Music,
  Clock,
  Play,
  Shuffle,
  MoreHorizontal,
  Trash2,
  ListOrdered,
  ArrowLeft,
  User,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { albumDetailsQueryOpts } from "@/lib/query-opts";
import { deleteAlbum } from "@/lib/api";
import { unwrapString, unwrapInt, unwrapFloat } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
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
import MediaNotFound from "@/components/MediaNotFound";
import AlbumDetailsBackdrop from "@/components/AlbumDetailsBackdrop";
import AlbumDetailsCoverBlock from "@/components/AlbumDetailsCoverBlock";
import { MOVIE_DETAILS_CONTENT_ENTER_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

function SpotifyGlyph({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.419 1.56-.299.421-1.02.599-1.559.3z" />
    </svg>
  );
}

function SpotifyPopularityMeter({ score }: { score: number }) {
  const pct = Math.max(0, Math.min(100, Math.round(score)));
  return (
    <div
      className="mx-auto mt-4 w-full max-w-md lg:mx-0"
      role="group"
      aria-label={`Spotify popularity ${pct} out of 100`}
    >
      <div className="flex items-center justify-between gap-2 text-sm">
        <span className="flex min-w-0 items-center gap-1.5 text-slate-400">
          <SpotifyGlyph className="size-4 shrink-0 text-green-500" />
          <span>Spotify popularity</span>
        </span>
        <span className="shrink-0 font-semibold text-green-400 tabular-nums">
          {pct}
        </span>
      </div>
      <div
        className="mt-2 h-2 overflow-hidden rounded-full bg-slate-700"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className="h-full rounded-full bg-green-500 transition-[width]"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

export const Route = createFileRoute("/_auth/music/album/$id")({
  loader: async ({ context, params }) => {
    const albumId = parseInt(params.id, 10);

    if (!Number.isNaN(albumId) && albumId > 0) {
      await context.queryClient.ensureQueryData(albumDetailsQueryOpts(albumId));
    }
  },
  component: AlbumDetailsPage,
});

function AlbumDetailsPage() {
  const { id } = Route.useParams();
  const albumId = parseInt(id, 10);

  const { data, isPending, isError } = useQuery(albumDetailsQueryOpts(albumId));

  if (Number.isNaN(albumId) || albumId <= 0) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-slate-300">
          Album not found
        </h2>
      </div>
    );
  }

  if (isPending) {
    return <AlbumDetailsSkeleton />;
  }

  if (isError || data?.error) {
    return (
      <MediaNotFound
        message={
          data?.message ||
          "Failed to load album details. Please try again later."
        }
      />
    );
  }

  if (!data?.data?.album) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-slate-300">
          Album not found
        </h2>
      </div>
    );
  }

  return <AlbumDetailsContent {...data.data} />;
}

function AlbumDetailsSkeleton() {
  return (
    <div
      className="animate-pulse"
      role="status"
      aria-label="Loading album details"
    >
      <span className="sr-only">Loading album details...</span>

      <div className="relative -mx-4 sm:-mx-6 lg:-mx-8" aria-hidden="true">
        <div className="h-44 w-full bg-slate-800 sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48" />
        <div className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent" />
      </div>

      <div
        className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32"
        aria-hidden="true"
      >
        <div className="flex min-w-0 flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
          <div className="mx-auto shrink-0 lg:mx-0 lg:pt-1">
            <div className="aspect-square w-44 rounded-xl bg-slate-800 sm:w-52 md:w-64 lg:w-72" />
          </div>

          <div className="min-w-0 flex-1 space-y-4 text-center lg:text-left">
            <div className="mx-auto h-10 max-w-lg rounded-sm bg-slate-800 lg:mx-0" />
            <div className="mx-auto h-6 max-w-xs rounded-sm bg-slate-800 lg:mx-0" />
            <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
              <div className="h-8 w-28 rounded-full bg-slate-800" />
              <div className="h-8 w-24 rounded-full bg-slate-800" />
              <div className="h-8 w-24 rounded-full bg-slate-800" />
            </div>
            <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
              <div className="h-7 w-20 rounded-full bg-slate-800" />
              <div className="h-7 w-24 rounded-full bg-slate-800" />
            </div>
            <div className="flex flex-col gap-3 pt-2 sm:flex-row sm:flex-wrap sm:justify-center lg:justify-start">
              <div className="h-12 w-full rounded-full bg-slate-800 sm:w-32" />
              <div className="h-12 w-full rounded-full bg-slate-800 sm:w-24" />
              <div className="mx-auto size-12 rounded-full bg-slate-800 sm:mx-0" />
            </div>
          </div>
        </div>

        <div className="mt-10 space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <div
              key={i}
              className="flex h-14 items-center gap-4 rounded-lg bg-slate-800/50"
            >
              <div className="ml-4 h-4 w-6 rounded-sm bg-slate-700" />
              <div className="h-4 max-w-xs flex-1 rounded-sm bg-slate-700" />
              <div className="mr-4 h-4 w-16 rounded-sm bg-slate-700" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
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
  const navigate = Route.useNavigate();
  const queryClient = useQueryClient();

  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  const coverUrl = getMediaImageUrl(unwrapString(album.cover));
  const releaseYear = unwrapInt(album.year);
  const musicianName = unwrapString(album.musician);
  const spotifyPopularity = unwrapFloat(album.spotify_popularity);

  // React 19 document metadata - dynamic based on album
  const pageTitle = musicianName
    ? `${album.title} by ${musicianName} - Igloo`
    : `${album.title} - Igloo`;
  const pageDescription = `Listen to ${album.title}${musicianName ? ` by ${musicianName}` : ""} - ${tracks.length} tracks in your Igloo music library.`;

  const handleDeleteAlbum = async () => {
    setIsDeleting(true);

    try {
      const result = await deleteAlbum(album.id);

      if (result.error) {
        showActionFailed(
          "delete album",
          result.message || "Unable to delete album. Please try again.",
        );
        return;
      }

      showDeleted(
        "Album",
        `"${album.title}" has been removed from your library.`,
      );

      // Invalidate album queries to refresh the list
      queryClient.invalidateQueries({ queryKey: ["albums"] });
      queryClient.invalidateQueries({ queryKey: ["music-stats"] });

      // Navigate back to music page
      navigate({ to: "/music", search: { tab: "albums" } });
    } catch (error) {
      console.error("Failed to delete album:", error);
      showActionFailed(
        "delete album",
        "An unexpected error occurred. Please try again.",
      );
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
    {} as Record<number, TrackType[]>,
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
        cover: coverUrl,
        title: album.title,
        musician: musicianName,
      });
    }
  };

  // Handle playing the album from the beginning
  const handlePlayAlbum = () => {
    audioPlayer.playAlbum(tracks, {
      cover: coverUrl,
      title: album.title,
      musician: musicianName,
    });
  };

  // Handle shuffle play
  const handleShufflePlay = () => {
    audioPlayer.shuffleAlbum(tracks, {
      cover: coverUrl,
      title: album.title,
      musician: musicianName,
    });
  };

  return (
    <article aria-labelledby="album-title" className="pb-6 sm:pb-10">
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <div className={cn(MOVIE_DETAILS_CONTENT_ENTER_CLASS)}>
        <AlbumDetailsBackdrop coverUrl={coverUrl} albumTitle={album.title} />
      </div>

      <div className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32">
        <div
          className={cn(
            MOVIE_DETAILS_CONTENT_ENTER_CLASS,
            "delay-75 motion-reduce:delay-0",
          )}
        >
          <div className="flex min-w-0 flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
            <AlbumDetailsCoverBlock
              coverUrl={coverUrl}
              albumTitle={album.title}
            />

            <div className="min-w-0 flex-1 text-center lg:text-left">
              <h1
                id="album-title"
                tabIndex={-1}
                className="flex w-full max-w-full min-w-0 flex-col gap-1 text-2xl font-bold wrap-break-word text-white outline-none sm:text-3xl lg:text-4xl xl:text-5xl"
              >
                <span className="min-w-0 text-balance">{album.title}</span>
              </h1>

              {musicianName && (
                <p className="mt-2 text-lg font-medium text-amber-400 sm:text-xl lg:text-2xl">
                  {musicianName}
                </p>
              )}

              <ul
                className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
                aria-label="Album details"
              >
                {(album.release_date.Valid || releaseYear) && (
                  <li className="flex items-center gap-1.5 rounded-full border border-slate-600/40 bg-slate-800/90 px-3 py-1.5 text-sm text-slate-200">
                    <Calendar
                      className="size-4 shrink-0 text-slate-400"
                      aria-hidden="true"
                    />
                    <time
                      dateTime={
                        album.release_date.String || String(releaseYear ?? "")
                      }
                    >
                      {album.release_date.Valid
                        ? formatDate(album.release_date.String)
                        : releaseYear}
                    </time>
                  </li>
                )}
                <li className="flex items-center gap-1.5 rounded-full border border-slate-600/40 bg-slate-800/90 px-3 py-1.5 text-sm text-slate-200">
                  <Music
                    className="size-4 shrink-0 text-slate-400"
                    aria-hidden="true"
                  />
                  <span>
                    {tracks.length} {tracks.length === 1 ? "track" : "tracks"}
                  </span>
                </li>
                <li className="flex items-center gap-1.5 rounded-full border border-slate-600/40 bg-slate-800/90 px-3 py-1.5 text-sm text-slate-200">
                  <Clock
                    className="size-4 shrink-0 text-slate-400"
                    aria-hidden="true"
                  />
                  <time
                    dateTime={`PT${total_duration}S`}
                    aria-label={`Total duration ${formatDuration(total_duration)}`}
                  >
                    {formatDuration(total_duration)}
                  </time>
                </li>
              </ul>

              {album_genres.length > 0 && (
                <ul
                  className="mt-4 flex list-none flex-wrap justify-center gap-2 lg:justify-start"
                  aria-label={`Genres: ${album_genres.join(", ")}`}
                >
                  {album_genres.map(genre => (
                    <li
                      key={genre}
                      className="rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm"
                    >
                      {genre}
                    </li>
                  ))}
                </ul>
              )}

              {spotifyPopularity != null && (
                <SpotifyPopularityMeter score={spotifyPopularity} />
              )}

              <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:justify-center lg:justify-start">
                <button
                  onClick={handlePlayAlbum}
                  className="inline-flex w-full items-center justify-center gap-2 rounded-full bg-amber-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 sm:w-auto"
                >
                  <Play className="size-4 fill-current" aria-hidden="true" />
                  Play Album
                </button>
                <button
                  onClick={handleShufflePlay}
                  className="inline-flex w-full items-center justify-center gap-2 rounded-full border border-slate-600 bg-slate-700 px-6 py-3 font-semibold text-white transition-colors hover:bg-slate-600 sm:w-auto"
                  aria-label="Shuffle play album"
                >
                  <Shuffle className="size-4" aria-hidden="true" />
                  Shuffle
                </button>

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      className="inline-flex w-full items-center justify-center gap-2 rounded-full border border-slate-600 bg-slate-700 px-4 py-3 font-semibold text-white transition-colors hover:bg-slate-600 sm:w-auto"
                      aria-label="More options"
                    >
                      <MoreHorizontal className="size-4" aria-hidden="true" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    align="end"
                    className="border-slate-700 bg-slate-800"
                  >
                    <DropdownMenuItem
                      onClick={() => setIsDeleteDialogOpen(true)}
                      className="cursor-pointer text-red-400 focus:bg-red-500/10 focus:text-red-400"
                    >
                      <Trash2 className="mr-2 size-4" aria-hidden="true" />
                      Delete Album
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>

              <Dialog
                open={isDeleteDialogOpen}
                onOpenChange={setIsDeleteDialogOpen}
              >
                <DialogContent className="border-slate-700 bg-slate-900 text-white">
                  <DialogHeader>
                    <DialogTitle className="text-white">Delete Album</DialogTitle>
                    <DialogDescription className="text-slate-400">
                      Are you sure you want to delete "{album.title}"? This
                      action cannot be undone and will permanently remove:
                    </DialogDescription>
                  </DialogHeader>

                  <ul className="ml-4 list-disc space-y-1 text-sm text-slate-300">
                    <li>The album and all its metadata</li>
                    <li>
                      All {tracks.length}{" "}
                      {tracks.length === 1 ? "track" : "tracks"} associated with
                      this album
                    </li>
                    <li>All genre and artist associations</li>
                  </ul>

                  <DialogFooter className="gap-2 sm:gap-0">
                    <button
                      onClick={() => setIsDeleteDialogOpen(false)}
                      disabled={isDeleting}
                      className="rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 font-medium text-white transition-colors hover:bg-slate-600 disabled:opacity-50"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={handleDeleteAlbum}
                      disabled={isDeleting}
                      className="rounded-lg bg-red-600 px-4 py-2 font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50"
                    >
                      {isDeleting ? (
                        <>
                          <Spinner className="mr-2 size-4" />
                          Deleting...
                        </>
                      ) : (
                        "Delete Album"
                      )}
                    </button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>

              {artists.length > 0 && (
                <section className="mt-6" aria-labelledby="artists-heading">
                  <h2
                    id="artists-heading"
                    className="mb-3 text-center text-sm font-semibold tracking-wide text-slate-400 uppercase lg:text-left"
                  >
                    {artists.length === 1 ? "Artist" : "Artists"}
                  </h2>
                  <div className="flex flex-wrap justify-center gap-3 lg:justify-start">
                    {artists.map((artist: ArtistType) => (
                      <ArtistBadge key={artist.id} artist={artist} />
                    ))}
                  </div>
                </section>
              )}
            </div>
          </div>
        </div>

        <div
          className={cn(
            MOVIE_DETAILS_CONTENT_ENTER_CLASS,
            "space-y-8 delay-150 motion-reduce:delay-0 sm:space-y-10",
          )}
        >
          <section className="min-w-0" aria-labelledby="tracklist-heading">
            <h2
              id="tracklist-heading"
              className="mb-4 flex items-center justify-center gap-2 text-lg font-semibold text-white sm:text-xl lg:justify-start"
            >
              <ListOrdered
                className="size-5 shrink-0 text-amber-400"
                aria-hidden="true"
              />
              Track List
            </h2>

            <div className="overflow-hidden rounded-xl border border-amber-500/10 bg-slate-800/30">
              {discNumbers.map(discNum => (
                <div key={discNum}>
                  {hasMultipleDiscs && (
                    <div className="border-b border-slate-700/50 bg-slate-800/50 px-4 py-2">
                      <span className="flex items-center gap-2 text-sm font-medium text-slate-400">
                        <Disc3
                          className="size-4 text-amber-400/70"
                          aria-hidden="true"
                        />
                        Disc {discNum}
                      </span>
                    </div>
                  )}
                  <div className="divide-y divide-slate-700/30">
                    {tracksByDisc[discNum].map((track: TrackType) => (
                      <TrackItem
                        key={track.id}
                        id={track.id}
                        title={track.title}
                        duration={track.duration}
                        trackIndex={track.track_index}
                        genres={trackGenreMap.get(track.id) || []}
                        variant="album"
                        isPlaying={isTrackPlaying(track)}
                        isCurrentTrack={
                          audioPlayer.currentTrack?.id === track.id
                        }
                        onPlay={() => handleToggleTrack(track)}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </section>

          <section
            className="rounded-xl border border-amber-500/10 bg-slate-800/30 p-4 sm:p-6"
            aria-labelledby="details-heading"
          >
            <h2
              id="details-heading"
              className="mb-4 text-lg font-semibold text-white"
            >
              Album Details
            </h2>
            <dl className="grid grid-cols-1 gap-6 text-sm min-[480px]:grid-cols-2 lg:grid-cols-4">
              {album.release_date.Valid && (
                <div>
                  <dt className="font-semibold tracking-wide text-amber-300/70 uppercase">
                    Release Date
                  </dt>
                  <dd className="mt-1 text-white">
                    {formatDate(album.release_date.String)}
                  </dd>
                </div>
              )}
              <div>
                <dt className="font-semibold tracking-wide text-amber-300/70 uppercase">
                  Total Tracks
                </dt>
                <dd className="mt-1 text-white">{tracks.length}</dd>
              </div>
              <div>
                <dt className="font-semibold tracking-wide text-amber-300/70 uppercase">
                  Total Duration
                </dt>
                <dd className="mt-1 text-white">
                  {formatDuration(total_duration)}
                </dd>
              </div>
              {spotifyPopularity != null && (
                <div>
                  <dt className="font-semibold tracking-wide text-amber-300/70 uppercase">
                    Spotify popularity
                  </dt>
                  <dd className="mt-1 flex items-baseline gap-2 text-white">
                    <span className="text-lg font-semibold text-green-400 tabular-nums">
                      {Math.round(spotifyPopularity)}
                    </span>
                    <span className="text-slate-500">/ 100</span>
                  </dd>
                </div>
              )}
            </dl>
          </section>

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-6">
            <Link
              to="/music"
              search={{ tab: "albums" }}
              className="inline-flex items-center justify-center gap-2 text-slate-400 transition-colors hover:text-amber-400 sm:justify-start"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              Back to Music
            </Link>
            <Link
              to="/"
              className="inline-flex items-center justify-center gap-2 text-slate-500 transition-colors hover:text-amber-400/90 sm:justify-start"
            >
              Home
            </Link>
          </div>
        </div>
      </div>
    </article>
  );
}

function ArtistBadge({ artist }: { artist: ArtistType }) {
  const thumbUrl = getMediaImageUrl(
    artist.thumb.Valid ? artist.thumb.String : null
  );
  return (
    <div className="flex items-center gap-2 rounded-full border border-slate-700/50 bg-slate-800/60 px-3 py-1.5 transition-colors hover:border-amber-500/30">
      {thumbUrl ? (
        <img
          src={thumbUrl}
          alt=""
          className="size-6 rounded-full object-cover"
        />
      ) : (
        <div className="flex size-6 items-center justify-center rounded-full bg-slate-700">
          <User className="size-3 text-slate-400" aria-hidden="true" />
        </div>
      )}
      <span className="text-sm font-medium text-white">{artist.name}</span>
    </div>
  );
}
