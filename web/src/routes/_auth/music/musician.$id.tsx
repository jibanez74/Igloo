import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  User,
  Disc3,
  Music,
  Clock,
  Play,
  Shuffle,
  ListOrdered,
  ArrowLeft,
} from "lucide-react";
import { musicianDetailsQueryOpts } from "@/lib/query-opts";
import { unwrapString, unwrapInt, unwrapFloat } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import MediaNotFound from "@/components/shared/MediaNotFound";
import MusicianDetailsSkipLinks from "@/components/music/MusicianDetailsSkipLinks";
import AlbumCard from "@/components/music/AlbumCard";
import {
  SpotifyGlyph,
  SpotifyPopularityMeter,
} from "@/components/music/SpotifyPopularity";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import TrackItem from "@/components/music/TrackItem";
import { formatDuration } from "@/lib/format";
import { convertToAudioTrack } from "@/lib/audio-utils";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  DETAIL_TRACK_LIST_CONTAINER_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_LOADING_STATE_CLASS,
  SPOTIFY_BRAND_ICON_CLASS,
  SPOTIFY_BRAND_TEXT_CLASS,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type {
  MusicianAlbumType,
  MusicianDetailsResponseType,
  MusicianTrackType,
} from "@/types";

export const Route = createFileRoute("/_auth/music/musician/$id")({
  loader: async ({ context, params }) => {
    const musicianId = parseInt(params.id, 10);

    if (!Number.isNaN(musicianId) && musicianId > 0) {
      await context.queryClient.ensureQueryData(
        musicianDetailsQueryOpts(musicianId),
      );
    }
  },
  component: MusicianDetailsPage,
});

function MusicianDetailsPage() {
  const { id } = Route.useParams();
  const musicianId = parseInt(id, 10);
  const isValidId = !Number.isNaN(musicianId) && musicianId > 0;

  const { data, isPending, isError } = useQuery({
    ...musicianDetailsQueryOpts(musicianId),
    enabled: isValidId,
  });

  if (!isValidId) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-muted-foreground">
          Musician not found
        </h2>
      </div>
    );
  }

  if (isPending) {
    return <MusicianDetailsSkeleton />;
  }

  if (isError || data?.error) {
    return (
      <MediaNotFound
        message={
          data?.message ||
          "Failed to load musician details. Please try again later."
        }
        backTo="/music"
        backLabel="Back to Music"
      />
    );
  }

  if (!data?.data?.musician) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-muted-foreground">
          Musician not found
        </h2>
      </div>
    );
  }

  return <MusicianDetailsContent key={musicianId} {...data.data} />;
}

function MusicianDetailsSkeleton() {
  return (
    <div
      className={MOTION_LOADING_STATE_CLASS}
      role="status"
      aria-label="Loading musician details"
    >
      <span className="sr-only">Loading musician details...</span>

      <div className="relative -mx-4 sm:-mx-6 lg:-mx-8" aria-hidden="true">
        <div className="h-44 w-full bg-muted sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48" />
        <div className="absolute inset-0 bg-linear-to-t from-background via-background/60 to-transparent" />
      </div>

      <div
        className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32"
        aria-hidden="true"
      >
        <div className="flex flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
          <div className="mx-auto shrink-0 lg:mx-0">
            <div className="aspect-square w-48 rounded-full bg-muted md:w-56 lg:w-64" />
          </div>

          <div className="min-w-0 flex-1 space-y-4 text-center lg:text-left">
            <div className="mx-auto h-10 max-w-md rounded-sm bg-muted lg:mx-0" />
            <div className="mx-auto h-6 max-w-lg rounded-sm bg-muted lg:mx-0" />
            <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
              <div className="h-7 w-20 rounded-full bg-muted" />
              <div className="h-7 w-24 rounded-full bg-muted" />
            </div>
            <div className="flex flex-wrap justify-center gap-4 lg:justify-start">
              <div className="h-5 w-20 rounded-sm bg-muted" />
              <div className="h-5 w-20 rounded-sm bg-muted" />
              <div className="h-5 w-16 rounded-sm bg-muted" />
            </div>
            <div className="flex flex-col gap-3 sm:flex-row sm:justify-center lg:justify-start">
              <div className="h-12 w-full rounded-full bg-muted sm:w-32" />
              <div className="h-12 w-full rounded-full bg-muted sm:w-28" />
            </div>
          </div>
        </div>

        <div className="mt-10">
          <div className="mb-4 h-7 w-40 rounded-sm bg-muted" />
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <div
                key={i}
                className="overflow-hidden rounded-xl border border-border bg-card"
              >
                <div className="aspect-square bg-muted" />
                <div className="space-y-2 p-3">
                  <div className="h-4 w-3/4 rounded-sm bg-accent" />
                  <div className="h-3 w-1/2 rounded-sm bg-accent" />
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-10">
          <div className="mb-4 h-7 w-32 rounded-sm bg-muted" />
          <div className="space-y-2">
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
    </div>
  );
}

// Format follower count for display
function formatFollowers(count: number) {
  if (count >= 1_000_000) {
    return `${(count / 1_000_000).toFixed(1)}M`;
  }

  if (count >= 1_000) {
    return `${Math.floor(count / 1_000)}K`;
  }

  return count.toString();
}

function MusicianDetailsContent({
  musician,
  albums,
  tracks,
  genres,
  total_duration,
}: MusicianDetailsResponseType) {
  const audioPlayer = useAudioPlayerActions();
  const matchTrackPlayback = useTrackPlaybackMatcher();

  const [thumbFailed, setThumbFailed] = useState(false);

  const thumbUrl = getMediaImageUrl(unwrapString(musician.thumb));
  const showThumb = thumbUrl && !thumbFailed;
  const summary = unwrapString(musician.summary);
  const spotifyPopularityRaw = unwrapFloat(musician.spotify_popularity);
  const spotifyPopularity =
    spotifyPopularityRaw !== null ? Math.round(spotifyPopularityRaw) : null;
  const spotifyFollowers = unwrapInt(musician.spotify_followers);

  // React 19 document metadata - dynamic based on musician
  const pageTitle = `${musician.name} - Igloo`;
  const pageDescription = `Listen to ${musician.name} - ${albums.length} albums, ${tracks.length} tracks in your Igloo music library.`;

  const convertTracksForPlayer = (musicianTracks: MusicianTrackType[]) => {
    return musicianTracks.map((track) =>
      convertToAudioTrack({
        id: track.id,
        title: track.title,
        duration: track.duration,
        file_path: track.file_path,
        codec: track.codec,
        bit_rate: track.bit_rate,
        album_id: track.album_id,
        musician_id: { Int64: musician.id, Valid: true },
        album_cover: track.album_cover,
        musician_name: { String: musician.name, Valid: true },
      }),
    );
  };

  const handlePlayAll = () => {
    if (tracks.length === 0) return;

    const playerTracks = convertTracksForPlayer(tracks);
    audioPlayer.playAlbum(playerTracks, {
      cover: thumbUrl,
      title: musician.name,
      musician: musician.name,
    });
  };

  const handleShufflePlay = () => {
    if (tracks.length === 0) return;

    const playerTracks = convertTracksForPlayer(tracks);
    audioPlayer.shuffleAlbum(playerTracks, {
      cover: thumbUrl,
      title: musician.name,
      musician: musician.name,
    });
  };

  const handlePlayTrack = (track: MusicianTrackType) => {
    // Row buttons are labeled "Pause X" for the current track; playAlbum is a
    // start-over entry point, so toggle here instead (matches the album route).
    if (matchTrackPlayback(track.id).isCurrentTrack) {
      audioPlayer.togglePlay();
      return;
    }

    const playerTracks = convertTracksForPlayer(tracks);
    const trackIndex = tracks.findIndex((t) => t.id === track.id);
    const reorderedTracks = [
      ...playerTracks.slice(trackIndex),
      ...playerTracks.slice(0, trackIndex),
    ];

    audioPlayer.playAlbum(reorderedTracks, {
      cover: unwrapString(track.album_cover) ?? thumbUrl,
      title: musician.name,
      musician: musician.name,
    });
  };

  // Screen reader announcement summarizing the page
  const pageAnnouncement = `${musician.name}. ${albums.length} ${albums.length === 1 ? "album" : "albums"}, ${tracks.length} ${tracks.length === 1 ? "track" : "tracks"}. Total duration: ${formatDuration(total_duration)}.${genres.length > 0 ? ` Genres: ${genres.join(", ")}.` : ""}`;
  const pageAnnouncementId = `musician-${musician.id}-summary`;

  return (
    <article
      className="w-full min-w-0 pb-6 sm:pb-10"
      aria-labelledby="musician-name"
      aria-describedby={pageAnnouncementId}
    >
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      {/* Screen reader announcement */}
      <span id={pageAnnouncementId} className="sr-only">
        {pageAnnouncement}
      </span>

      <MusicianDetailsSkipLinks
        hasDiscography={albums.length > 0}
        hasTracks={tracks.length > 0}
      />

      <div className={cn(DETAIL_PAGE_CONTENT_ENTER_CLASS)}>
        <MusicianDetailsBackdrop thumbUrl={thumbUrl} name={musician.name} />
      </div>

      <div className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32">
        <div
          className={cn(
            DETAIL_PAGE_CONTENT_ENTER_CLASS,
            "delay-75 motion-reduce:delay-0",
          )}
        >
          {/* Header section */}
          <header className="mb-10 flex flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
            {/* Musician thumbnail */}
            <figure className="mx-auto shrink-0 lg:mx-0">
              <div className="aspect-square w-48 overflow-hidden rounded-full border border-primary/20 shadow-2xl shadow-primary/10 md:w-56 lg:w-64">
                {showThumb ? (
                  <img
                    src={thumbUrl}
                    alt={musician.name}
                    loading="lazy"
                    decoding="async"
                    fetchPriority="low"
                    className="size-full object-cover"
                    onError={() => setThumbFailed(true)}
                  />
                ) : (
                  <div
                    className="flex size-full items-center justify-center bg-muted"
                    role="img"
                    aria-label="No image available"
                  >
                    <User
                      className="size-16 text-muted-foreground"
                      aria-hidden="true"
                    />
                  </div>
                )}
              </div>
            </figure>

            {/* Musician info */}
            <div className="flex min-w-0 flex-1 flex-col text-center lg:text-left">
              {/* Name */}
              <h1
                id="musician-name"
                tabIndex={-1}
                className="max-w-full min-w-0 text-2xl font-bold text-balance wrap-break-word text-foreground outline-hidden sm:text-3xl md:text-4xl lg:text-5xl"
                title={musician.name}
              >
                {musician.name}
              </h1>

              {/* Summary */}
              {summary && (
                <p className="mt-3 text-sm text-muted-foreground sm:text-base lg:max-w-2xl">
                  {summary}
                </p>
              )}

              {/* Genre tags */}
              {genres.length > 0 && (
                <ul
                  className="mt-4 flex list-none flex-wrap justify-center gap-2 lg:justify-start"
                  aria-label={`Genres: ${genres.join(", ")}`}
                >
                  {genres.map((genre) => (
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

              {/* Stats row */}
              <ul
                className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
                aria-label="Musician statistics"
              >
                <li>
                  <Badge
                    variant="outline"
                    className="gap-1.5 border-border/40 bg-muted/90 px-3 py-1.5 text-sm font-normal text-foreground"
                  >
                    <Disc3
                      className="size-4 shrink-0 text-muted-foreground"
                      aria-hidden="true"
                    />
                    <span>
                      {albums.length} {albums.length === 1 ? "album" : "albums"}
                    </span>
                  </Badge>
                </li>
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

              {/* Play buttons */}
              {tracks.length > 0 && (
                <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:justify-center lg:justify-start">
                  <Button
                    type="button"
                    variant="accent-pill"
                    size="lg"
                    onClick={handlePlayAll}
                    className="w-full font-semibold shadow-lg shadow-primary/20 sm:w-auto"
                    aria-label={`Play all ${tracks.length} tracks by ${musician.name}`}
                  >
                    <Play className="size-4 fill-current" aria-hidden="true" />
                    Play All
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="lg"
                    onClick={handleShufflePlay}
                    className="w-full rounded-full font-semibold sm:w-auto"
                    aria-label={`Shuffle play all ${tracks.length} tracks by ${musician.name}`}
                  >
                    <Shuffle className="size-4" aria-hidden="true" />
                    Shuffle
                  </Button>
                </div>
              )}

              {/* Spotify stats */}
              {(spotifyPopularity !== null || spotifyFollowers !== null) && (
                <div className="mt-4">
                  {spotifyPopularity !== null && (
                    <SpotifyPopularityMeter score={spotifyPopularity} />
                  )}
                  {spotifyFollowers !== null && (
                    <div className="mt-3 flex items-center justify-center gap-1.5 text-sm text-muted-foreground lg:justify-start">
                      <SpotifyGlyph
                        className={cn("size-4 shrink-0", SPOTIFY_BRAND_ICON_CLASS)}
                      />
                      <span>Followers</span>
                      <span
                        className={cn(
                          "font-semibold tabular-nums",
                          SPOTIFY_BRAND_TEXT_CLASS,
                        )}
                      >
                        {formatFollowers(spotifyFollowers)}
                      </span>
                    </div>
                  )}
                </div>
              )}
            </div>
          </header>
        </div>

        <div
          className={cn(
            DETAIL_PAGE_CONTENT_ENTER_CLASS,
            "space-y-8 delay-150 motion-reduce:delay-0 sm:space-y-10",
          )}
        >
          {/* Discography section */}
          {albums.length > 0 && (
            <section aria-labelledby="discography-heading">
              <h2
                id="discography-heading"
                tabIndex={-1}
                className="mb-4 flex items-center gap-2 text-xl font-semibold text-foreground outline-hidden"
              >
                <Disc3 className="size-5 text-primary" aria-hidden="true" />
                Discography
              </h2>

              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
                {albums.map((album) => (
                  <AlbumCard
                    key={album.id}
                    album={{
                      id: album.id,
                      title: album.title,
                      cover: album.cover,
                      musician: { String: musician.name, Valid: true },
                      year: album.year,
                    }}
                    subtitle={albumSubtitle(album)}
                  />
                ))}
              </div>
            </section>
          )}

          {/* All Tracks section */}
          {tracks.length > 0 && (
            <section aria-labelledby="tracks-heading">
              <h2
                id="tracks-heading"
                tabIndex={-1}
                className="mb-4 flex items-center gap-2 text-xl font-semibold text-foreground outline-hidden"
              >
                <ListOrdered
                  className="size-5 text-primary"
                  aria-hidden="true"
                />
                All Tracks
              </h2>

              <div className={DETAIL_TRACK_LIST_CONTAINER_CLASS}>
                <div className="divide-y divide-border/30">
                  {tracks.map((track) => (
                    <TrackItem
                      key={track.id}
                      id={track.id}
                      title={track.title}
                      duration={track.duration}
                      subtitle={
                        unwrapString(track.album_title) ?? "Unknown Album"
                      }
                      albumId={unwrapInt(track.album_id)}
                      variant="musician"
                      {...matchTrackPlayback(track.id)}
                      onPlay={() => handlePlayTrack(track)}
                    />
                  ))}
                </div>
              </div>
            </section>
          )}

          {/* Back link */}
          <nav aria-label="Page navigation">
            <Link
              to="/music"
              search={{ tab: "musicians" }}
              className={cn(
                MOTION_MICRO_COLORS_CLASS,
                FOCUS_VISIBLE_RING_CLASS,
                "inline-flex items-center gap-2 rounded-md px-2 py-1 text-muted-foreground hover:text-primary",
              )}
              aria-label="Back to Musicians library"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              Back to Musicians
            </Link>
          </nav>
        </div>
      </div>
    </article>
  );
}

function MusicianDetailsBackdrop({
  thumbUrl,
  name,
}: {
  thumbUrl: string | null;
  name: string;
}) {
  const [failed, setFailed] = useState(false);
  const showImage = thumbUrl && !failed;

  return (
    <div className="relative -mx-4 sm:-mx-6 lg:-mx-8" aria-hidden="true">
      {showImage ? (
        <img
          src={thumbUrl}
          alt=""
          loading="lazy"
          decoding="async"
          fetchPriority="low"
          className="h-44 w-full object-cover object-center sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48"
          onError={() => setFailed(true)}
        />
      ) : (
        <div className="flex h-44 w-full items-center justify-center bg-muted sm:h-52 md:aspect-21/9 md:min-h-48">
          <User
            className="size-16 text-muted-foreground opacity-40"
            aria-hidden="true"
          />
          <span className="sr-only">{name}</span>
        </div>
      )}
      <div
        className="absolute inset-0 bg-linear-to-t from-background via-background/60 to-transparent"
        aria-hidden="true"
      />
    </div>
  );
}

function albumSubtitle(album: MusicianAlbumType) {
  const year = unwrapInt(album.year);
  const trackCount = `${album.track_count} ${album.track_count === 1 ? "track" : "tracks"}`;

  return year ? `${year} · ${trackCount}` : trackCount;
}
