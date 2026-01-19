import { createFileRoute, Link } from "@tanstack/react-router";
import { musicianDetailsQueryOpts } from "@/lib/query-opts";
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
        <i className="fa-solid fa-circle-exclamation" aria-hidden="true" />
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

  const thumbUrl = musician.thumb?.Valid ? musician.thumb.String : null;
  const summary = musician.summary?.Valid ? musician.summary.String : null;
  const spotifyPopularity = musician.spotify_popularity?.Valid
    ? Math.round(musician.spotify_popularity.Float64)
    : null;
  const spotifyFollowers = musician.spotify_followers?.Valid
    ? musician.spotify_followers.Int64
    : null;

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
      cover: track.album_cover?.Valid ? track.album_cover.String : thumbUrl,
      title: musician.name,
      musician: musician.name,
    });
  };

  const isTrackPlaying = (track: MusicianTrackType) => {
    return audioPlayer.currentTrack?.id === track.id && audioPlayer.isPlaying;
  };

  return (
    <article
      className="animate-in duration-300 fade-in"
      aria-labelledby="musician-name"
    >
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
                <i
                  className="fa-solid fa-user text-6xl text-slate-600"
                  aria-hidden="true"
                />
              </div>
            )}
          </div>
        </figure>

        {/* Musician info */}
        <div className="flex flex-1 flex-col text-center md:text-left">
          {/* Name */}
          <h1
            id="musician-name"
            className="text-3xl font-bold text-white md:text-4xl lg:text-5xl"
          >
            {musician.name}
          </h1>

          {/* Summary */}
          {summary && (
            <p className="mt-3 max-w-2xl text-slate-400">{summary}</p>
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
            className="mt-4 flex flex-wrap items-center justify-center gap-4 text-slate-400 md:justify-start"
            aria-label="Musician statistics"
          >
            <li className="flex items-center gap-1.5">
              <i
                className="fa-solid fa-compact-disc text-slate-500"
                aria-hidden="true"
              />
              <span>
                {albums.length} {albums.length === 1 ? "album" : "albums"}
              </span>
            </li>
            <li className="flex items-center gap-1.5">
              <i
                className="fa-solid fa-music text-slate-500"
                aria-hidden="true"
              />
              <span>
                {tracks.length} {tracks.length === 1 ? "track" : "tracks"}
              </span>
            </li>
            <li className="flex items-center gap-1.5">
              <i
                className="fa-regular fa-clock text-slate-500"
                aria-hidden="true"
              />
              <span>{formatDuration(total_duration)}</span>
            </li>
          </ul>

          {/* Play buttons */}
          {tracks.length > 0 && (
            <div className="mt-6 flex flex-wrap justify-center gap-3 md:justify-start">
              <button
                onClick={handlePlayAll}
                className="inline-flex items-center gap-2 rounded-full bg-amber-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400"
              >
                <i className="fa-solid fa-play" aria-hidden="true" />
                Play All
              </button>
              <button
                onClick={handleShufflePlay}
                className="inline-flex items-center gap-2 rounded-full border border-slate-600 bg-slate-700 px-6 py-3 font-semibold text-white transition-colors hover:bg-slate-600"
                aria-label="Shuffle play all tracks"
              >
                <i className="fa-solid fa-shuffle" aria-hidden="true" />
                Shuffle
              </button>
            </div>
          )}

          {/* Spotify stats */}
          {(spotifyPopularity !== null || spotifyFollowers !== null) && (
            <div className="mt-4 flex flex-wrap items-center justify-center gap-4 text-sm text-slate-500 md:justify-start">
              <i
                className="fa-brands fa-spotify text-green-500"
                aria-hidden="true"
              />
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
            <i
              className="fa-solid fa-compact-disc text-amber-400"
              aria-hidden="true"
            />
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
            <i
              className="fa-solid fa-list-ol text-amber-400"
              aria-hidden="true"
            />
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
                  subtitle={
                    track.album_title?.Valid
                      ? track.album_title.String
                      : "Unknown Album"
                  }
                  albumId={track.album_id?.Valid ? Number(track.album_id.Int64) : null}
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
      <div className="mt-8">
        <Link
          to="/music"
          search={{ tab: "musicians" }}
          className="inline-flex items-center gap-2 text-slate-400 transition-colors hover:text-amber-400"
        >
          <i className="fa-solid fa-arrow-left" aria-hidden="true" />
          Back to Musicians
        </Link>
      </div>
    </article>
  );
}

function AlbumCard({ album }: { album: MusicianAlbumType }) {
  const coverUrl = album.cover?.Valid ? album.cover.String : null;
  const year = album.year?.Valid ? album.year.Int64 : null;

  return (
    <article className="group animate-in duration-300 fade-in">
      <Link
        to="/music/album/$id"
        params={{ id: album.id.toString() }}
        className="block overflow-hidden rounded-lg border border-slate-800 bg-slate-900 transition-all hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20"
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
              <i
                className="fa-solid fa-compact-disc text-4xl text-slate-600"
                aria-hidden="true"
              />
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

