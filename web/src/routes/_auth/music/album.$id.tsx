import { useRef, useState } from "react";
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
import {
  albumDetailsQueryOpts,
  authUserQueryOpts,
  likedTrackIdsQueryOpts,
} from "@/lib/query-opts";
import { deleteAlbum } from "@/lib/api";
import { unwrapString, unwrapInt, unwrapFloat } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useAudioPlayerState } from "@/hooks/useAudioPlayerState";
import TrackItem from "@/components/TrackItem";
import ConfirmDialog from "@/components/ConfirmDialog";
import { formatDate, formatDuration } from "@/lib/format";
import type {
  AlbumDetailsResponseType,
  ArtistType,
  AuthUser,
  TrackGenreType,
  TrackType,
} from "@/types";
import MediaNotFound from "@/components/MediaNotFound";
import AlbumDetailsBackdrop from "@/components/AlbumDetailsBackdrop";
import AlbumDetailsCoverBlock from "@/components/AlbumDetailsCoverBlock";
import AlbumDetailsSkipLinks from "@/components/AlbumDetailsSkipLinks";
import { SpotifyPopularityMeter } from "@/components/SpotifyPopularity";
import {
  ALBUM_DETAILS_KEY,
  ALBUMS_PAGINATED_KEY,
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  LATEST_ALBUMS_KEY,
  MUSIC_STATS_KEY,
  MOTION_LOADING_STATE_CLASS,
  SPOTIFY_BRAND_TEXT_CLASS,
  TRACKS_INFINITE_KEY,
  MOTION_MICRO_COLORS_CLASS,
  DETAIL_TRACK_LIST_CONTAINER_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

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
        <h2 className="text-xl font-semibold text-muted-foreground">
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
        backTo="/music"
        backLabel="Back to Music"
      />
    );
  }

  if (!data?.data?.album) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-muted-foreground">
          Album not found
        </h2>
      </div>
    );
  }

  return <AlbumDetailsContent key={albumId} {...data.data} />;
}

function AlbumDetailsSkeleton() {
  return (
    <div
      className={MOTION_LOADING_STATE_CLASS}
      role="status"
      aria-label="Loading album details"
    >
      <span className="sr-only">Loading album details...</span>

      <div className="relative -mx-4 sm:-mx-6 lg:-mx-8" aria-hidden="true">
        <div className="h-44 w-full bg-muted sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48" />
        <div className="absolute inset-0 bg-linear-to-t from-background via-background/60 to-transparent" />
      </div>

      <div
        className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32"
        aria-hidden="true"
      >
        <div className="flex min-w-0 flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
          <div className="mx-auto shrink-0 lg:mx-0 lg:pt-1">
            <div className="aspect-square w-44 rounded-xl bg-muted sm:w-52 md:w-64 lg:w-72" />
          </div>

          <div className="min-w-0 flex-1 space-y-4 text-center lg:text-left">
            <div className="mx-auto h-10 max-w-lg rounded-sm bg-muted lg:mx-0" />
            <div className="mx-auto h-6 max-w-xs rounded-sm bg-muted lg:mx-0" />
            <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
              <div className="h-8 w-28 rounded-full bg-muted" />
              <div className="h-8 w-24 rounded-full bg-muted" />
              <div className="h-8 w-24 rounded-full bg-muted" />
            </div>
            <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
              <div className="h-7 w-20 rounded-full bg-muted" />
              <div className="h-7 w-24 rounded-full bg-muted" />
            </div>
            <div className="flex flex-col gap-3 pt-2 sm:flex-row sm:flex-wrap sm:justify-center lg:justify-start">
              <div className="h-12 w-full rounded-full bg-muted sm:w-32" />
              <div className="h-12 w-full rounded-full bg-muted sm:w-24" />
              <div className="mx-auto size-12 rounded-full bg-muted sm:mx-0" />
            </div>
          </div>
        </div>

        <div className="mt-10 space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <div
              key={i}
              className="flex h-14 items-center gap-4 rounded-lg bg-muted/50"
            >
              <div className="ml-4 h-4 w-6 rounded-sm bg-accent" />
              <div className="h-4 max-w-xs flex-1 rounded-sm bg-accent" />
              <div className="mr-4 h-4 w-16 rounded-sm bg-accent" />
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
  const audioPlayer = useAudioPlayerActions();
  const audioPlayerState = useAudioPlayerState();
  const navigate = Route.useNavigate();
  const queryClient = useQueryClient();

  const { data: likedIdsData } = useQuery(likedTrackIdsQueryOpts());
  const likedSet = new Set<number>(
    likedIdsData?.error === false ? (likedIdsData.data.liked_track_ids ?? []) : []
  );

  const { data: userData } = useQuery(authUserQueryOpts());
  const user: AuthUser | null =
    userData?.error === false && userData.data?.user
      ? (userData.data.user as AuthUser)
      : null;
  const isAdmin = user?.is_admin === true;

  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const moreOptionsButtonRef = useRef<HTMLButtonElement | null>(null);

  const coverUrl = getMediaImageUrl(unwrapString(album.cover));
  const releaseYear = unwrapInt(album.year);
  const musicianName = unwrapString(album.musician);
  const spotifyPopularity = unwrapFloat(album.spotify_popularity);

  // The album artist links to the musician page when it matches one of the
  // album's artists (which are musician rows server-side). A non-match (e.g.
  // "Various Artists") stays plain text.
  const linkedArtist = musicianName
    ? artists.find(a => a.name.toLowerCase() === musicianName.toLowerCase())
    : undefined;

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
        setIsDeleting(false);
        return;
      }

      showDeleted(
        "Album",
        `"${album.title}" has been removed from your library.`,
      );

      // Refresh music views that can include the deleted album or its tracks.
      queryClient.invalidateQueries({ queryKey: [ALBUMS_PAGINATED_KEY] });
      queryClient.invalidateQueries({ queryKey: [ALBUM_DETAILS_KEY, album.id] });
      queryClient.invalidateQueries({ queryKey: [LATEST_ALBUMS_KEY] });
      queryClient.invalidateQueries({ queryKey: [MUSIC_STATS_KEY] });
      queryClient.invalidateQueries({ queryKey: [TRACKS_INFINITE_KEY] });

      setIsDeleteDialogOpen(false);
      navigate({ to: "/music", search: { tab: "albums" } });
    } catch (error) {
      console.error("Failed to delete album:", error);
      showActionFailed(
        "delete album",
        "An unexpected error occurred. Please try again.",
      );
      setIsDeleting(false);
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

  // Audio quality summary: dominant codec, peak bitrate, and channel layout
  // when it's uniform across the album.
  const codecCounts = new Map<string, number>();
  tracks.forEach(track => {
    if (track.codec) {
      codecCounts.set(track.codec, (codecCounts.get(track.codec) ?? 0) + 1);
    }
  });
  let dominantCodec = "";
  let dominantCodecCount = 0;
  codecCounts.forEach((count, codec) => {
    if (count > dominantCodecCount) {
      dominantCodec = codec;
      dominantCodecCount = count;
    }
  });
  const maxBitRate = tracks.reduce(
    (max, track) => Math.max(max, track.bit_rate),
    0,
  );
  const uniformChannelLayout =
    tracks.length > 0 &&
    tracks[0].channel_layout &&
    tracks.every(track => track.channel_layout === tracks[0].channel_layout)
      ? tracks[0].channel_layout
      : null;
  const audioQuality = dominantCodec
    ? [
        dominantCodec.toUpperCase(),
        maxBitRate > 0 ? `${Math.round(maxBitRate / 1000)} kbps` : null,
        uniformChannelLayout,
      ]
        .filter(Boolean)
        .join(" · ")
    : null;

  // Screen reader announcement summarizing the page
  const pageAnnouncement = `${album.title}${musicianName ? ` by ${musicianName}` : ""}. ${tracks.length} ${tracks.length === 1 ? "track" : "tracks"}. Total duration: ${formatDuration(total_duration)}.${album_genres.length > 0 ? ` Genres: ${album_genres.join(", ")}.` : ""}`;
  const pageAnnouncementId = `album-${album.id}-summary`;

  // Check if a specific track is currently playing
  const isTrackPlaying = (track: TrackType) => {
    return (
      audioPlayerState.currentTrack?.id === track.id &&
      audioPlayerState.isPlaying
    );
  };

  // Handle playing/pausing a track
  const handleToggleTrack = (track: TrackType) => {
    if (audioPlayerState.currentTrack?.id === track.id) {
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
    if (tracks.length === 0) return;

    audioPlayer.playAlbum(tracks, {
      cover: coverUrl,
      title: album.title,
      musician: musicianName,
    });
  };

  // Handle shuffle play
  const handleShufflePlay = () => {
    if (tracks.length === 0) return;

    audioPlayer.shuffleAlbum(tracks, {
      cover: coverUrl,
      title: album.title,
      musician: musicianName,
    });
  };

  return (
    <article
      aria-labelledby="album-title"
      aria-describedby={pageAnnouncementId}
      className="w-full min-w-0 pb-6 sm:pb-10"
    >
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      {/* Screen reader announcement */}
      <span id={pageAnnouncementId} className="sr-only">
        {pageAnnouncement}
      </span>

      <AlbumDetailsSkipLinks />

      <div className={cn(DETAIL_PAGE_CONTENT_ENTER_CLASS)}>
        <AlbumDetailsBackdrop coverUrl={coverUrl} albumTitle={album.title} />
      </div>

      <div className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32">
        <div
          className={cn(
            DETAIL_PAGE_CONTENT_ENTER_CLASS,
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
                className="flex w-full max-w-full min-w-0 flex-col gap-1 text-2xl font-bold wrap-break-word text-foreground outline-hidden sm:text-3xl lg:text-4xl xl:text-5xl"
              >
                <span className="min-w-0 text-balance">{album.title}</span>
              </h1>

              {musicianName && (
                <p className="mt-2 text-lg font-medium text-primary sm:text-xl lg:text-2xl">
                  {linkedArtist ? (
                    <Link
                      to="/music/musician/$id"
                      params={{ id: String(linkedArtist.id) }}
                      className={cn(
                        MOTION_MICRO_COLORS_CLASS,
                        FOCUS_VISIBLE_RING_CLASS,
                        "rounded-sm hover:underline",
                      )}
                    >
                      {musicianName}
                    </Link>
                  ) : (
                    musicianName
                  )}
                </p>
              )}

              <ul
                className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
                aria-label="Album details"
              >
                {(album.release_date.Valid || releaseYear) && (
                  <li>
                    <Badge
                      variant="outline"
                      className="gap-1.5 border-border/40 bg-muted/90 px-3 py-1.5 text-sm font-normal text-foreground"
                    >
                      <Calendar
                        className="size-4 shrink-0 text-muted-foreground"
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
                    </Badge>
                  </li>
                )}
                <li>
                  <Badge
                    variant="outline"
                    className="gap-1.5 border-border/40 bg-muted/90 px-3 py-1.5 text-sm font-normal text-foreground"
                  >
                    <Music
                      className="size-4 shrink-0 text-muted-foreground"
                      aria-hidden="true"
                    />
                    <span>
                      {tracks.length} {tracks.length === 1 ? "track" : "tracks"}
                    </span>
                  </Badge>
                </li>
                <li>
                  <Badge
                    variant="outline"
                    className="gap-1.5 border-border/40 bg-muted/90 px-3 py-1.5 text-sm font-normal text-foreground"
                  >
                    <Clock
                      className="size-4 shrink-0 text-muted-foreground"
                      aria-hidden="true"
                    />
                    <time
                      dateTime={`PT${Math.round(total_duration / 1000)}S`}
                      aria-label={`Total duration ${formatDuration(total_duration)}`}
                    >
                      {formatDuration(total_duration)}
                    </time>
                  </Badge>
                </li>
              </ul>

              {album_genres.length > 0 && (
                <ul
                  className="mt-4 flex list-none flex-wrap justify-center gap-2 lg:justify-start"
                  aria-label={`Genres: ${album_genres.join(", ")}`}
                >
                  {album_genres.map(genre => (
                    <li key={genre}>
                      <Badge
                        variant="outline"
                        className="border-primary/30 bg-muted/80 px-3 py-1 text-sm font-normal text-primary backdrop-blur-sm"
                      >
                        {genre}
                      </Badge>
                    </li>
                  ))}
                </ul>
              )}

              {spotifyPopularity != null && (
                <SpotifyPopularityMeter score={spotifyPopularity} />
              )}

              <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:justify-center lg:justify-start">
                {tracks.length > 0 && (
                  <>
                    <Button
                      type="button"
                      variant="accent-pill"
                      size="lg"
                      onClick={handlePlayAlbum}
                      className="w-full font-semibold shadow-lg shadow-primary/20 sm:w-auto"
                    >
                      <Play
                        className="size-4 fill-current"
                        aria-hidden="true"
                      />
                      Play Album
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="lg"
                      onClick={handleShufflePlay}
                      className="w-full rounded-full font-semibold sm:w-auto"
                      aria-label="Shuffle play album"
                    >
                      <Shuffle className="size-4" aria-hidden="true" />
                      Shuffle
                    </Button>
                  </>
                )}

                {isAdmin && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        type="button"
                        ref={moreOptionsButtonRef}
                        variant="outline"
                        size="lg"
                        className="w-full rounded-full font-semibold sm:w-auto sm:px-4"
                        aria-label="More options"
                      >
                        <MoreHorizontal className="size-4" aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent
                      align="end"
                      className="border-border bg-muted"
                    >
                      <DropdownMenuItem
                        onClick={() => setIsDeleteDialogOpen(true)}
                        className="cursor-pointer text-destructive focus:bg-destructive/10 focus:text-destructive"
                      >
                        <Trash2 className="mr-2 size-4" aria-hidden="true" />
                        Delete Album
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>

              <ConfirmDialog
                open={isDeleteDialogOpen}
                onOpenChange={setIsDeleteDialogOpen}
                title="Delete Album"
                description={
                  <>
                    Are you sure you want to delete "{album.title}"? This action
                    cannot be undone and will permanently remove:
                  </>
                }
                confirmLabel="Delete Album"
                pending={isDeleting}
                restoreFocusRef={moreOptionsButtonRef}
                onConfirm={() => void handleDeleteAlbum()}
              >
                <ul className="ml-4 list-disc space-y-1 text-sm text-muted-foreground">
                  <li>The album and all its metadata</li>
                  <li>
                    All {tracks.length}{" "}
                    {tracks.length === 1 ? "track" : "tracks"} associated with
                    this album
                  </li>
                  <li>All genre and artist associations</li>
                </ul>
              </ConfirmDialog>

              {artists.length > 0 && (
                <section className="mt-6" aria-labelledby="artists-heading">
                  <h2
                    id="artists-heading"
                    className="mb-3 text-center text-sm font-semibold tracking-wide text-muted-foreground uppercase lg:text-left"
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
            DETAIL_PAGE_CONTENT_ENTER_CLASS,
            "space-y-8 delay-150 motion-reduce:delay-0 sm:space-y-10",
          )}
        >
          <section className="min-w-0" aria-labelledby="tracklist-heading">
            <h2
              id="tracklist-heading"
              tabIndex={-1}
              className="mb-4 flex items-center justify-center gap-2 text-lg font-semibold text-foreground outline-hidden sm:text-xl lg:justify-start"
            >
              <ListOrdered
                className="size-5 shrink-0 text-primary"
                aria-hidden="true"
              />
              Track List
            </h2>

            <div className={DETAIL_TRACK_LIST_CONTAINER_CLASS}>
              {tracks.length === 0 && (
                <div className="flex flex-col items-center gap-3 p-10 text-center">
                  <Music
                    className="size-8 text-muted-foreground opacity-60"
                    aria-hidden="true"
                  />
                  <p className="text-muted-foreground">
                    No tracks in this album
                  </p>
                </div>
              )}
              {discNumbers.map(discNum => (
                <div key={discNum}>
                  {hasMultipleDiscs && (
                    <div className="border-b border-border/50 bg-muted/50 px-4 py-2">
                      <span className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                        <Disc3
                          className="size-4 text-primary/70"
                          aria-hidden="true"
                        />
                        Disc {discNum}
                      </span>
                    </div>
                  )}
                  <div className="divide-y divide-border/30">
                    {tracksByDisc[discNum].map((track: TrackType) => (
                      <TrackItem
                        key={track.id}
                        id={track.id}
                        title={track.title}
                        duration={track.duration}
                        trackIndex={track.track_index}
                        genres={trackGenreMap.get(track.id) || []}
                        variant="album"
                        isLiked={likedSet.has(track.id)}
                        isPlaying={isTrackPlaying(track)}
                        isCurrentTrack={audioPlayerState.currentTrack?.id === track.id}
                        onPlay={() => handleToggleTrack(track)}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </section>

          <section
            className="rounded-xl border border-primary/10 bg-muted/30 p-4 sm:p-6"
            aria-labelledby="details-heading"
          >
            <h2
              id="details-heading"
              tabIndex={-1}
              className="mb-4 text-lg font-semibold text-foreground outline-hidden"
            >
              Album Details
            </h2>
            <dl className="grid grid-cols-1 gap-6 text-sm min-[480px]:grid-cols-2 lg:grid-cols-4">
              {album.release_date.Valid && (
                <div>
                  <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                    Release Date
                  </dt>
                  <dd className="mt-1 text-foreground">
                    {formatDate(album.release_date.String)}
                  </dd>
                </div>
              )}
              <div>
                <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                  Total Tracks
                </dt>
                <dd className="mt-1 text-foreground">{tracks.length}</dd>
              </div>
              <div>
                <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                  Total Duration
                </dt>
                <dd className="mt-1 text-foreground">
                  {formatDuration(total_duration)}
                </dd>
              </div>
              {musicianName && (
                <div>
                  <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                    Artist
                  </dt>
                  <dd className="mt-1 text-foreground">
                    {linkedArtist ? (
                      <Link
                        to="/music/musician/$id"
                        params={{ id: String(linkedArtist.id) }}
                        className={cn(
                          MOTION_MICRO_COLORS_CLASS,
                          FOCUS_VISIBLE_RING_CLASS,
                          "rounded-sm text-primary hover:underline",
                        )}
                      >
                        {musicianName}
                      </Link>
                    ) : (
                      musicianName
                    )}
                  </dd>
                </div>
              )}
              {album_genres.length > 0 && (
                <div>
                  <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                    Genres
                  </dt>
                  <dd className="mt-1 text-foreground">
                    {album_genres.join(", ")}
                  </dd>
                </div>
              )}
              {hasMultipleDiscs && (
                <div>
                  <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                    Discs
                  </dt>
                  <dd className="mt-1 text-foreground">{discNumbers.length}</dd>
                </div>
              )}
              {audioQuality && (
                <div>
                  <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                    Audio Quality
                  </dt>
                  <dd className="mt-1 text-foreground">{audioQuality}</dd>
                </div>
              )}
              {spotifyPopularity != null && (
                <div>
                  <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                    Spotify popularity
                  </dt>
                  <dd className="mt-1 flex items-baseline gap-2 text-foreground">
                    <span
                      className={cn(
                        "text-lg font-semibold tabular-nums",
                        SPOTIFY_BRAND_TEXT_CLASS,
                      )}
                    >
                      {Math.round(spotifyPopularity)}
                    </span>
                    <span className="text-muted-foreground">/ 100</span>
                  </dd>
                </div>
              )}
            </dl>
          </section>

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-6">
            <Link
              to="/music"
              search={{ tab: "albums" }}
              className={cn(
                MOTION_MICRO_COLORS_CLASS,
                FOCUS_VISIBLE_RING_CLASS,
                "inline-flex items-center justify-center gap-2 rounded-md px-2 py-1 text-muted-foreground hover:text-primary sm:justify-start",
              )}
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              Back to Music
            </Link>
            <Link
              to="/"
              className={cn(
                MOTION_MICRO_COLORS_CLASS,
                FOCUS_VISIBLE_RING_CLASS,
                "inline-flex items-center justify-center gap-2 rounded-md px-2 py-1 text-muted-foreground hover:text-primary sm:justify-start",
              )}
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
  const [thumbFailed, setThumbFailed] = useState(false);
  const thumbUrl = getMediaImageUrl(
    artist.thumb.Valid ? artist.thumb.String : null
  );
  const showThumb = thumbUrl && !thumbFailed;

  return (
    <Link
      to="/music/musician/$id"
      params={{ id: String(artist.id) }}
      className={cn(
        MOTION_MICRO_COLORS_CLASS,
        FOCUS_VISIBLE_RING_CLASS,
        "flex items-center gap-2 rounded-full border border-border/50 bg-muted/60 px-3 py-1.5 hover:border-primary/30",
      )}
    >
      {showThumb ? (
        <img
          src={thumbUrl}
          alt=""
          loading="lazy"
          decoding="async"
          className="size-6 rounded-full object-cover"
          onError={() => setThumbFailed(true)}
        />
      ) : (
        <div className="flex size-6 items-center justify-center rounded-full bg-accent">
          <User className="size-3 text-muted-foreground" aria-hidden="true" />
        </div>
      )}
      <span className="text-sm font-medium text-foreground">{artist.name}</span>
    </Link>
  );
}
