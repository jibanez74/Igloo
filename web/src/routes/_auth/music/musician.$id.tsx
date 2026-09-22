import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  User,
  Disc3,
  Music,
  Clock,
  Play,
  Shuffle,
  ListOrdered,
} from "lucide-react";
import { musicianDetailsQueryOpts } from "@/lib/query-opts";
import { unwrapString, unwrapInt, unwrapFloat } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { parseRouteId } from "@/lib/route-id";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import MediaNotFound from "@/components/shared/MediaNotFound";
import DetailSkipLinks from "@/components/shared/DetailSkipLinks";
import AlbumCard from "@/components/music/AlbumCard";
import MusicDetailBackdrop from "@/components/music/MusicDetailBackdrop";
import MusicDetailBackNav from "@/components/music/MusicDetailBackNav";
import MusicDetailSkeleton from "@/components/music/MusicDetailSkeleton";
import {
  SpotifyGlyph,
  SpotifyPopularityMeter,
} from "@/components/music/SpotifyPopularity";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import TrackItem from "@/components/music/TrackItem";
import { formatDuration, pluralize } from "@/lib/format";
import { convertToAudioTrack } from "@/lib/audio-utils";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  DETAIL_RAIL_HEADING_CLASS,
  DETAIL_TRACK_LIST_CONTAINER_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  SPOTIFY_BRAND_ICON_CLASS,
  SPOTIFY_BRAND_TEXT_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type {
  MusicianAlbumType,
  MusicianDetailsResponseType,
  MusicianTrackType,
  PlayableTrackData,
} from "@/types";

export const Route = createFileRoute("/_auth/music/musician/$id")({
  loader: async ({ context, params }) => {
    const musicianId = parseRouteId(params.id);

    if (musicianId != null) {
      await context.queryClient.ensureQueryData(
        musicianDetailsQueryOpts(musicianId),
      );
    }
  },
  component: MusicianDetailsPage,
});

function MusicianDetailsPage() {
  const { id } = Route.useParams();
  const musicianId = parseRouteId(id);

  // A malformed id never reaches the API: the query options disable
  // themselves for the zero sentinel, and the page goes straight to
  // not-found rather than sitting on a skeleton.
  const { data, isPending, isError } = useQuery(
    musicianDetailsQueryOpts(musicianId ?? 0),
  );

  if (musicianId == null) {
    return (
      <MediaNotFound
        message="That musician link is not valid."
        backTo="/music"
        backLabel="Back to Music"
      />
    );
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

  if (isPending) {
    return <MusicDetailSkeleton variant="musician" />;
  }

  if (!data?.data?.musician) {
    return (
      <MediaNotFound
        message="Musician not found."
        backTo="/music"
        backLabel="Back to Music"
      />
    );
  }

  return <MusicianDetailsContent key={musicianId} {...data.data} />;
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

  // A musician's tracks span every album they appear on, so each row carries its
  // own album title and cover. Keep them as PlayableTrackData and hand them to
  // the player, or the queue-wide fallback shows the musician's name in the
  // album slot and their photo as the cover for every track. Only the artist is
  // genuinely queue-wide here, so that one is filled in from the page.
  const toPlayableData = (
    musicianTracks: MusicianTrackType[],
  ): PlayableTrackData[] =>
    musicianTracks.map((track) => ({
      id: track.id,
      title: track.title,
      duration: track.duration,
      codec: track.codec,
      bit_rate: track.bit_rate,
      album_id: track.album_id,
      album_title: track.album_title,
      album_cover: track.album_cover,
      musician_id: { Int64: musician.id, Valid: true },
      musician_name: { String: musician.name, Valid: true },
    }));

  const albumInfo = {
    cover: thumbUrl,
    title: musician.name,
    musician: musician.name,
  };

  const handlePlayAll = () => {
    if (tracks.length === 0) return;

    const rawTracks = toPlayableData(tracks);
    audioPlayer.playQueue(
      rawTracks.map(convertToAudioTrack),
      albumInfo,
      rawTracks,
    );
  };

  const handleShufflePlay = () => {
    if (tracks.length === 0) return;

    const rawTracks = toPlayableData(tracks);
    audioPlayer.shuffleQueue(
      rawTracks.map(convertToAudioTrack),
      albumInfo,
      rawTracks,
    );
  };

  // playTrackFromList (not playQueue) so a click on the current row toggles
  // play/pause instead of restarting — the row button is labeled "Pause X"
  // there. The list is rotated to start at the clicked track, which is what the
  // page has always done, and passing the raw rows keeps each track's own album
  // details as the queue advances.
  const handlePlayTrack = (track: MusicianTrackType) => {
    const trackIndex = tracks.findIndex((t) => t.id === track.id);
    if (trackIndex < 0) return;

    const rawTracks = toPlayableData([
      ...tracks.slice(trackIndex),
      ...tracks.slice(0, trackIndex),
    ]);

    audioPlayer.playTrackFromList(rawTracks, track.id);
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

      <DetailSkipLinks
        titleHref="#musician-name"
        titleLabel="Skip to musician info"
        sections={[
          albums.length > 0 && {
            href: "#discography-heading",
            label: "Skip to discography",
          },
          tracks.length > 0 && {
            href: "#tracks-heading",
            label: "Skip to all tracks",
          },
        ]}
      />

      <div className={cn(DETAIL_PAGE_CONTENT_ENTER_CLASS)}>
        <MusicDetailBackdrop imageUrl={thumbUrl ?? ""} fallbackIcon={User} />
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
                className={cn(
                  "max-w-full min-w-0 rounded-sm text-2xl font-bold text-balance wrap-break-word text-foreground sm:text-3xl md:text-4xl lg:text-5xl",
                  FOCUS_VISIBLE_RING_CLASS,
                )}
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
                    aria-label={`Play all ${pluralize(tracks.length, "track")} by ${musician.name}`}
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
                    aria-label={`Shuffle play all ${pluralize(tracks.length, "track")} by ${musician.name}`}
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
                className={cn(DETAIL_RAIL_HEADING_CLASS, "flex items-center gap-2")}
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
                className={cn(DETAIL_RAIL_HEADING_CLASS, "flex items-center gap-2")}
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

          <MusicDetailBackNav tab="musicians" label="Back to Musicians" />
        </div>
      </div>
    </article>
  );
}

function albumSubtitle(album: MusicianAlbumType) {
  const year = unwrapInt(album.year);
  const trackCount = `${album.track_count} ${album.track_count === 1 ? "track" : "tracks"}`;

  return year ? `${year} · ${trackCount}` : trackCount;
}
