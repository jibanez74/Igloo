import { useRef, useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { showDeleted, showActionFailed } from "@/lib/toast-helpers";
import {
  Disc3,
  Calendar,
  Music,
  MoreHorizontal,
  Trash2,
  ListOrdered,
  User,
} from "lucide-react";
import {
  albumDetailsQueryOpts,
  authUserFrom,
  authUserQueryOpts,
} from "@/lib/query-opts";
import { deleteAlbum } from "@/lib/api";
import { invalidateMusicLibraryQueries } from "@/lib/music-library-cache";
import { parseRouteId } from "@/lib/route-id";
import { listenHead, routeHead } from "@/lib/route-head";
import { unwrapString, unwrapInt, unwrapFloat } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import TrackItem from "@/components/music/TrackItem";
import ConfirmDialog from "@/components/shared/ConfirmDialog";
import {
  formatBitRate,
  formatDate,
  formatDuration,
  nounForCount,
  pluralize,
} from "@/lib/format";
import type {
  AlbumDetailsResponseType,
  ArtistType,
  TrackGenreType,
  TrackType,
} from "@/types";
import MediaDetailGuard from "@/components/shared/MediaDetailGuard";
import MusicDetailBackdrop from "@/components/music/MusicDetailBackdrop";
import MusicDetailBackNav from "@/components/music/MusicDetailBackNav";
import MusicDetailSkeleton from "@/components/music/MusicDetailSkeleton";
import MusicDetailArt from "@/components/music/MusicDetailArt";
import MusicGenreList from "@/components/music/MusicGenreList";
import PlayShuffleButtons from "@/components/music/PlayShuffleButtons";
import MusicStatList, {
  MusicDurationChip,
  MusicStatChip,
} from "@/components/music/MusicStatList";
import DetailSkipLinks from "@/components/shared/DetailSkipLinks";
import { SpotifyPopularityMeter } from "@/components/music/SpotifyPopularity";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  DETAIL_SECTION_HEADING_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  SPOTIFY_BRAND_TEXT_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  DETAIL_TRACK_LIST_CONTAINER_CLASS,
  MUSIC_DETAIL_ACTIONS_CLASS,
  MUSIC_DETAIL_HERO_ROW_CLASS,
  MUSIC_DETAIL_SHELL_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type AlbumHeadData = {
  title: string;
  musician: string | null;
  trackCount: number;
};

const ALBUM_FALLBACK_HEAD = routeHead("Album");

function albumHead(album: AlbumHeadData | null | undefined) {
  if (!album) return ALBUM_FALLBACK_HEAD;

  const byline = album.musician ? ` by ${album.musician}` : "";

  return listenHead(
    `${album.title}${byline}`,
    pluralize(album.trackCount, "track"),
  );
}

export const Route = createFileRoute("/_auth/music/album/$id")({
  loader: async ({ context, params }) => {
    const albumId = parseRouteId(params.id);
    if (albumId == null) return { album: null };

    const res = await context.queryClient.ensureQueryData(
      albumDetailsQueryOpts(albumId),
    );
    if (res.error || !res.data.album) return { album: null };

    return {
      album: {
        title: res.data.album.title,
        musician: unwrapString(res.data.album.musician),
        trackCount: res.data.tracks.length,
      },
    };
  },
  head: ({ loaderData }) => albumHead(loaderData?.album),
  pendingComponent: AlbumDetailsSkeleton,
  component: AlbumDetailsPage,
});

// One skeleton serves the router's pending view, while the loader waits, and
// the guard's, so the two cannot drift apart (design-system §3.4).
function AlbumDetailsSkeleton() {
  // It holds a place for the admin "More options" button only when the loaded
  // page will show one; _auth's beforeLoad has already cached the user.
  const { data } = useQuery(authUserQueryOpts());
  const isAdmin = authUserFrom(data)?.is_admin === true;

  return <MusicDetailSkeleton variant="album" withMenu={isAdmin} />;
}

function AlbumDetailsPage() {
  const { id } = Route.useParams();
  const albumId = parseRouteId(id);

  // A malformed id never reaches the API: the query options disable
  // themselves for the zero sentinel, and the page goes straight to
  // not-found rather than sitting on a skeleton.
  const { data, isPending, isError } = useQuery(
    albumDetailsQueryOpts(albumId ?? 0),
  );

  return (
    <MediaDetailGuard
      id={albumId}
      noun="album"
      back="music"
      isPending={isPending}
      isError={isError}
      data={data}
      payload={data?.data?.album ? data.data : null}
      skeleton={<AlbumDetailsSkeleton />}
    >
      {(loaded, id) => <AlbumDetailsContent key={id} {...loaded} />}
    </MediaDetailGuard>
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
  const matchTrackPlayback = useTrackPlaybackMatcher();
  const navigate = Route.useNavigate();
  const queryClient = useQueryClient();

  const { data: userData } = useQuery(authUserQueryOpts());
  const user = authUserFrom(userData);
  const isAdmin = user?.is_admin === true;

  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const moreOptionsButtonRef = useRef<HTMLButtonElement | null>(null);

  const coverUrl = getMediaImageUrl(unwrapString(album.cover));
  const releaseDate = unwrapString(album.release_date);
  const releaseYear = unwrapInt(album.year);
  const musicianName = unwrapString(album.musician);
  const spotifyPopularity = unwrapFloat(album.spotify_popularity);

  // The album artist links to the musician page when it matches one of the
  // album's artists (which are musician rows server-side). A non-match (e.g.
  // "Various Artists") stays plain text.
  const linkedArtist = musicianName
    ? artists.find(a => a.name.toLowerCase() === musicianName.toLowerCase())
    : undefined;

  // Deleting an album also removes its tracks, which the musician pages, the
  // liked lists and any playlist may show, so every music query goes stale.
  const deleteMutation = useMutation({
    mutationFn: () => deleteAlbum(album.id),
    onSuccess: (result) => {
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
      invalidateMusicLibraryQueries(queryClient);
      setIsDeleteDialogOpen(false);
      navigate({ to: "/music", search: { tab: "albums" } });
    },
    onError: (error) => {
      console.error("Failed to delete album:", error);
      showActionFailed(
        "delete album",
        "An unexpected error occurred. Please try again.",
      );
    },
  });

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
        maxBitRate > 0 ? formatBitRate(maxBitRate) : null,
        uniformChannelLayout,
      ]
        .filter(Boolean)
        .join(" · ")
    : null;

  // Screen reader announcement summarizing the page
  const pageAnnouncement = `${album.title}${musicianName ? ` by ${musicianName}` : ""}. ${pluralize(tracks.length, "track")}. Total duration: ${formatDuration(total_duration)}.${album_genres.length > 0 ? ` Genres: ${album_genres.join(", ")}.` : ""}`;
  const pageAnnouncementId = `album-${album.id}-summary`;

  const albumInfo = {
    cover: coverUrl,
    title: album.title,
    musician: musicianName,
  };

  // playTrack toggles play/pause itself when the clicked track is current
  const handleToggleTrack = (track: TrackType) => {
    audioPlayer.playTrack(track, tracks, albumInfo);
  };

  // Play the album from the beginning, or shuffled
  const startAlbumQueue = (shuffle: boolean) => {
    if (tracks.length === 0) return;

    if (shuffle) {
      audioPlayer.shuffleQueue(tracks, albumInfo);
    } else {
      audioPlayer.playQueue(tracks, albumInfo);
    }
  };

  return (
    <article
      aria-labelledby="album-title"
      aria-describedby={pageAnnouncementId}
      className="w-full min-w-0 pb-6 sm:pb-10"
    >
      {/* Screen reader announcement */}
      <span id={pageAnnouncementId} className="sr-only">
        {pageAnnouncement}
      </span>

      <DetailSkipLinks
        titleHref="#album-title"
        titleLabel="Skip to album info"
        sections={[
          { href: "#tracklist-heading", label: "Skip to track list" },
          { href: "#details-heading", label: "Skip to album details" },
        ]}
      />

      <div className={cn(DETAIL_PAGE_CONTENT_ENTER_CLASS)}>
        <MusicDetailBackdrop imageUrl={coverUrl ?? ""} fallbackIcon={Disc3} />
      </div>

      <div className={MUSIC_DETAIL_SHELL_CLASS}>
        <div
          className={cn(
            DETAIL_PAGE_CONTENT_ENTER_CLASS,
            "delay-75 motion-reduce:delay-0",
          )}
        >
          <div className={MUSIC_DETAIL_HERO_ROW_CLASS}>
            <MusicDetailArt
              variant="album"
              src={coverUrl ?? ""}
              name={album.title}
            />

            <div className="min-w-0 flex-1 text-center lg:text-left">
              <h1
                id="album-title"
                tabIndex={-1}
                className={cn(
                  "flex w-full max-w-full min-w-0 flex-col gap-1 rounded-sm text-2xl font-bold wrap-break-word text-foreground sm:text-3xl lg:text-4xl xl:text-5xl",
                  FOCUS_VISIBLE_RING_CLASS,
                )}
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

              <MusicStatList label="Album details">
                {(releaseDate || releaseYear) && (
                  <MusicStatChip icon={Calendar}>
                    <time dateTime={releaseDate ?? String(releaseYear ?? "")}>
                      {releaseDate ? formatDate(releaseDate) : releaseYear}
                    </time>
                  </MusicStatChip>
                )}
                <MusicStatChip icon={Music}>
                  {pluralize(tracks.length, "track")}
                </MusicStatChip>
                <MusicDurationChip ms={total_duration} />
              </MusicStatList>

              <MusicGenreList genres={album_genres} />

              {spotifyPopularity != null && (
                <SpotifyPopularityMeter score={spotifyPopularity} />
              )}

              <div className={MUSIC_DETAIL_ACTIONS_CLASS}>
                {tracks.length > 0 && (
                  <PlayShuffleButtons
                    playLabel="Play Album"
                    shuffleAriaLabel="Shuffle play album"
                    onPlay={() => startAlbumQueue(false)}
                    onShuffle={() => startAlbumQueue(true)}
                  />
                )}

                {isAdmin && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        type="button"
                        ref={moreOptionsButtonRef}
                        variant="outline"
                        size="lg"
                        className="self-center rounded-full font-semibold sm:self-auto sm:px-4"
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
                pending={deleteMutation.isPending}
                restoreFocusRef={moreOptionsButtonRef}
                onConfirm={() => deleteMutation.mutate()}
              >
                <ul className="ml-4 list-disc space-y-1 text-sm text-muted-foreground">
                  <li>The album and all its metadata</li>
                  <li>
                    All {pluralize(tracks.length, "track")} associated with
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
                    {nounForCount(artists.length, {
                      singular: "Artist",
                      plural: "Artists",
                    })}
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
              className={cn(
                DETAIL_SECTION_HEADING_CLASS,
                "mb-4 flex items-center justify-center gap-2 lg:justify-start",
              )}
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
                        musicianId={linkedArtist?.id}
                        {...matchTrackPlayback(track.id)}
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
              className={cn(DETAIL_SECTION_HEADING_CLASS, "mb-4")}
            >
              Album Details
            </h2>
            <dl className="grid grid-cols-1 gap-6 text-sm min-[480px]:grid-cols-2 lg:grid-cols-4">
              {releaseDate && (
                <div>
                  <dt className="font-semibold tracking-wide text-primary/70 uppercase">
                    Release Date
                  </dt>
                  <dd className="mt-1 text-foreground">{formatDate(releaseDate)}</dd>
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

          <MusicDetailBackNav tab="albums" label="Back to Albums" />
        </div>
      </div>
    </article>
  );
}

function ArtistBadge({ artist }: { artist: ArtistType }) {
  const thumbUrl = getMediaImageUrl(unwrapString(artist.thumb)) ?? "";
  const { showPoster: showThumb, onError } = usePosterFallback(thumbUrl);

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
          onError={onError}
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
