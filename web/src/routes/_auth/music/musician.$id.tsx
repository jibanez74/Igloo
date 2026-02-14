import { createFileRoute, Link } from "@tanstack/react-router";
import {
  AlertCircle,
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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { useAudioPlayer } from "@/hooks/useAudioPlayer";
import TrackItem from "@/components/TrackItem";
import { formatDuration } from "@/lib/format";
import { convertToAudioTrack } from "@/lib/audio-utils";
import type {
  MusicianAlbumType,
  MusicianDetailsResponseType,
  MusicianTrackType,
} from "@/types";

export const Route = createFileRoute("/_auth/music/musician/$id")({
  loader: async ({ context, params }) => {
    const musicianId = parseInt(params.id, 10);
    const data = await context.queryClient.ensureQueryData(
      musicianDetailsQueryOpts(musicianId)
    );

    return data;
  },
  component: MusicianDetailsPage,
});

function MusicianDetailsPage() {
  const data = Route.useLoaderData();

  if (data.error) {
    return (
      <Alert
        variant="destructive"
        className="border-red-500/20 bg-red-500/10 text-red-400"
      >
        <AlertCircle className="size-4" aria-hidden="true" />
        <AlertTitle>Error</AlertTitle>
        <AlertDescription>
          {data.message ||
            "Failed to load musician details. Please try again later."}
        </AlertDescription>
      </Alert>
    );
  }

  if (!data.data?.musician) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-slate-300">
          Musician not found
        </h2>
      </div>
    );
  }

  return <MusicianDetailsContent {...data.data} />;
}

function MusicianDetailsContent({
  musician,
  albums,
  tracks,
  genres,
  total_duration,
}: MusicianDetailsResponseType) {
  const audioPlayer = useAudioPlayer();

  const thumbUrl = unwrapString(musician.thumb);
  const summary = unwrapString(musician.summary);
  const spotifyPopularityRaw = unwrapFloat(musician.spotify_popularity);
  const spotifyPopularity = spotifyPopularityRaw !== null ? Math.round(spotifyPopularityRaw) : null;
  const spotifyFollowers = unwrapInt(musician.spotify_followers);

  // React 19 document metadata - dynamic based on musician
  const pageTitle = `${musician.name} - Igloo`;
  const pageDescription = `Listen to ${musician.name} - ${albums.length} albums, ${tracks.length} tracks in your Igloo music library.`;

  // Format follower count for display
  const formatFollowers = (count: number) => {
    if (count >= 1_000_000) {
      return `${(count / 1_000_000).toFixed(1)}M`;
    }

    if (count >= 1_000) {
      return `${Math.floor(count / 1_000)}K`;
    }

    return count.toString();
  };

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
      })
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

  const isTrackPlaying = (track: MusicianTrackType) => {
    return audioPlayer.currentTrack?.id === track.id && audioPlayer.isPlaying;
  };

  // Screen reader announcement summarizing the page
  const pageAnnouncement = `${musician.name}. ${albums.length} ${albums.length === 1 ? "album" : "albums"}, ${tracks.length} ${tracks.length === 1 ? "track" : "tracks"}. Total duration: ${formatDuration(total_duration)}.${genres.length > 0 ? ` Genres: ${genres.join(", ")}.` : ""}`;

  return (
    <article
      className="animate-in overflow-x-hidden duration-300 fade-in"
      aria-labelledby="musician-name"
    >
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      {/* Screen reader announcement - focusable for Tab navigation */}
      <span
        tabIndex={0}
        className="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-50
          focus:rounded-md focus:bg-slate-800 focus:px-4 focus:py-2 focus:text-white focus:ring-2
          focus:ring-amber-400 focus:outline-none"
        aria-label={pageAnnouncement}
      >
        {musician.name} - {albums.length} albums, {tracks.length} tracks
      </span>

      {/* Header section */}
      <header className="mb-10 flex flex-col gap-8 md:flex-row">
        {/* Musician thumbnail */}
        <figure className="mx-auto shrink-0 md:mx-0">
          <div className="aspect-square w-48 overflow-hidden rounded-full border border-amber-500/20 shadow-2xl shadow-amber-500/10 md:w-56 lg:w-64">
            {thumbUrl ? (
              <img
                src={thumbUrl}
                alt={musician.name}
                className="size-full object-cover"
              />
            ) : (
              <div
                className="flex size-full items-center justify-center bg-slate-800"
                role="img"
                aria-label="No image available"
              >
                <User className="size-16 text-slate-600" aria-hidden="true" />
              </div>
            )}
          </div>
        </figure>

        {/* Musician info */}
        <div className="flex min-w-0 flex-1 flex-col text-center md:text-left">
          {/* Name */}
          <h1
            id="musician-name"
            className="truncate text-2xl font-bold text-white sm:text-3xl md:text-4xl lg:text-5xl"
            title={musician.name}
          >
            {musician.name}
          </h1>

          {/* Summary */}
          {summary && (
            <p className="mt-3 text-sm text-slate-400 sm:text-base md:max-w-2xl">{summary}</p>
          )}

          {/* Genre tags */}
          {genres.length > 0 && (
            <ul
              className="mt-4 flex flex-wrap justify-center gap-2 md:justify-start"
              aria-label={`Genres: ${genres.join(", ")}`}
            >
              {genres.map((genre) => (
                <li
                  key={genre}
                  className="rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm"
                >
                  {genre}
                </li>
              ))}
            </ul>
          )}

          {/* Stats row */}
          <ul
            className="mt-4 flex flex-wrap items-center justify-center gap-x-4 gap-y-2 text-sm text-slate-400 sm:text-base md:justify-start"
            aria-label="Musician statistics"
          >
            <li className="flex items-center gap-1.5">
              <Disc3 className="size-4 text-slate-400" aria-hidden="true" />
              <span>
                {albums.length} {albums.length === 1 ? "album" : "albums"}
              </span>
            </li>
            <li className="flex items-center gap-1.5">
              <Music className="size-4 text-slate-400" aria-hidden="true" />
              <span>
                {tracks.length} {tracks.length === 1 ? "track" : "tracks"}
              </span>
            </li>
            <li className="flex items-center gap-1.5">
              <Clock className="size-4 text-slate-400" aria-hidden="true" />
              <span>{formatDuration(total_duration)}</span>
            </li>
          </ul>

          {/* Play buttons */}
          {tracks.length > 0 && (
            <div className="mt-6 flex flex-col justify-center gap-3 sm:flex-row md:justify-start">
              <button
                onClick={handlePlayAll}
                className="inline-flex items-center justify-center gap-2 rounded-full bg-amber-500 px-6 py-3
                  font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400
                  focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                aria-label={`Play all ${tracks.length} tracks by ${musician.name}`}
              >
                <Play className="size-4 fill-current" aria-hidden="true" />
                Play All
              </button>
              <button
                onClick={handleShufflePlay}
                className="inline-flex items-center justify-center gap-2 rounded-full border border-slate-600 bg-slate-700
                  px-6 py-3 font-semibold text-white transition-colors hover:bg-slate-600
                  focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                aria-label={`Shuffle play all ${tracks.length} tracks by ${musician.name}`}
              >
                <Shuffle className="size-4" aria-hidden="true" />
                Shuffle
              </button>
            </div>
          )}

          {/* Spotify stats */}
          {(spotifyPopularity !== null || spotifyFollowers !== null) && (
            <div className="mt-4 flex flex-wrap items-center justify-center gap-x-4 gap-y-1 text-xs text-slate-400 sm:text-sm md:justify-start">
              <svg className="size-4 text-green-500" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.419 1.56-.299.421-1.02.599-1.559.3z"/>
              </svg>
              {spotifyPopularity !== null && (
                <span>
                  <span className="text-slate-400">Popularity:</span>{" "}
                  <span className="font-medium text-green-400">
                    {spotifyPopularity}
                  </span>
                </span>
              )}
              {spotifyFollowers !== null && (
                <span>
                  <span className="text-slate-400">Followers:</span>{" "}
                  <span className="font-medium text-green-400">
                    {formatFollowers(spotifyFollowers)}
                  </span>
                </span>
              )}
            </div>
          )}
        </div>
      </header>

      {/* Discography section */}
      {albums.length > 0 && (
        <section className="mb-10" aria-labelledby="discography-heading">
          <h2
            id="discography-heading"
            className="mb-4 flex items-center gap-2 text-xl font-semibold text-white"
          >
            <Disc3 className="size-5 text-amber-400" aria-hidden="true" />
            Discography
          </h2>

          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
            {albums.map((album) => (
              <AlbumCard key={album.id} album={album} />
            ))}
          </div>
        </section>
      )}

      {/* All Tracks section */}
      {tracks.length > 0 && (
        <section aria-labelledby="tracks-heading">
          <h2
            id="tracks-heading"
            className="mb-4 flex items-center gap-2 text-xl font-semibold text-white"
          >
            <ListOrdered className="size-5 text-amber-400" aria-hidden="true" />
            All Tracks
          </h2>

          <div className="overflow-hidden rounded-xl border border-amber-500/10 bg-slate-800/30">
            <div className="divide-y divide-slate-700/30">
              {tracks.map((track) => (
                <TrackItem
                  key={track.id}
                  id={track.id}
                  title={track.title}
                  duration={track.duration}
                  subtitle={unwrapString(track.album_title) ?? "Unknown Album"}
                  albumId={unwrapInt(track.album_id)}
                  variant="musician"
                  isPlaying={isTrackPlaying(track)}
                  isCurrentTrack={audioPlayer.currentTrack?.id === track.id}
                  onPlay={() => handlePlayTrack(track)}
                />
              ))}
            </div>
          </div>
        </section>
      )}

      {/* Back link */}
      <nav className="mt-8" aria-label="Page navigation">
        <Link
          to="/music"
          search={{ tab: "musicians" }}
          className="inline-flex items-center gap-2 rounded-md px-2 py-1 text-slate-400 transition-colors
            hover:text-amber-400 focus:text-amber-400 focus:ring-2 focus:ring-amber-400 focus:outline-none"
          aria-label="Back to Musicians library"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          Back to Musicians
        </Link>
      </nav>
    </article>
  );
}

function AlbumCard({ album }: { album: MusicianAlbumType }) {
  const coverUrl = unwrapString(album.cover);
  const year = unwrapInt(album.year);

  return (
    <article className="group animate-in duration-300 fade-in">
      <Link
        to="/music/album/$id"
        params={{ id: album.id.toString() }}
        className="block overflow-hidden rounded-lg border border-slate-800 bg-slate-900 transition-all
          hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20
          focus:border-amber-400 focus:ring-2 focus:ring-amber-400 focus:outline-none"
        aria-label={`${album.title}${year ? `, ${year}` : ""}, ${album.track_count} ${album.track_count === 1 ? "track" : "tracks"}`}
      >
        {/* Album cover */}
        <div className="aspect-square overflow-hidden bg-slate-800">
          {coverUrl ? (
            <img
              src={coverUrl}
              alt={album.title}
              className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <Disc3 className="size-10 text-slate-600" aria-hidden="true" />
            </div>
          )}
        </div>

        {/* Album info */}
        <div className="p-3">
          <h3 className="truncate text-sm font-semibold text-white">
            {album.title}
          </h3>
          <p className="mt-0.5 text-xs text-slate-400">
            {year && <span>{year} · </span>}
            {album.track_count} {album.track_count === 1 ? "track" : "tracks"}
          </p>
        </div>
      </Link>
    </article>
  );
}

