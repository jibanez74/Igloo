import { createFileRoute, Link } from "@tanstack/react-router";
import { albumDetailsQueryOpts } from "@/lib/query-opts";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { useAudioPlayer } from "@/context/AudioPlayerContext";
import {
  formatDate,
  formatDuration,
  formatTrackDuration,
  formatBitRate,
} from "@/lib/format";
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
        className='bg-red-500/10 border-red-500/20 text-red-400'
      >
        <i className='fa-solid fa-circle-exclamation' aria-hidden='true'></i>
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
      <div className='text-center py-12'>
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

  const coverUrl = album.cover.Valid ? album.cover.String : null;
  const releaseYear = album.year.Valid ? album.year.Int64 : null;
  const musicianName = album.musician.Valid ? album.musician.String : null;

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
      <header className='flex flex-col md:flex-row gap-8 mb-10'>
        {/* Album cover */}
        <figure className='shrink-0 mx-auto md:mx-0'>
          <div className='w-64 md:w-72 lg:w-80 rounded-xl overflow-hidden shadow-2xl shadow-amber-500/10 border border-amber-500/20'>
            {coverUrl ? (
              <img
                src={coverUrl}
                alt={`Album cover for ${album.title}`}
                className='w-full aspect-square object-cover'
              />
            ) : (
              <div
                className='w-full aspect-square bg-slate-800 flex items-center justify-center'
                role='img'
                aria-label='No cover available'
              >
                <i
                  className='fa-solid fa-compact-disc text-slate-600 text-6xl'
                  aria-hidden='true'
                />
              </div>
            )}
          </div>
        </figure>

        {/* Album info */}
        <div className='flex-1 flex flex-col'>
          {/* Title */}
          <h1
            id='album-title'
            className='text-3xl md:text-4xl lg:text-5xl font-bold text-white'
          >
            {album.title}
          </h1>

          {/* Artist name */}
          {musicianName && (
            <p className='text-xl text-amber-400 mt-2 font-medium'>
              {musicianName}
            </p>
          )}

          {/* Meta info row */}
          <ul
            className='flex flex-wrap items-center gap-4 mt-4 text-slate-400'
            aria-label='Album details'
          >
            {(album.release_date.Valid || releaseYear) && (
              <li className='flex items-center gap-1.5'>
                <i
                  className='fa-regular fa-calendar text-slate-500'
                  aria-hidden='true'
                />
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
              <i
                className='fa-solid fa-music text-slate-500'
                aria-hidden='true'
              />
              <span>
                {tracks.length} {tracks.length === 1 ? "track" : "tracks"}
              </span>
            </li>
            <li className='flex items-center gap-1.5'>
              <i
                className='fa-regular fa-clock text-slate-500'
                aria-hidden='true'
              />
              <span>{formatDuration(total_duration)}</span>
            </li>
            {album.spotify_popularity.Valid && (
              <li
                className='flex items-center gap-1.5'
                aria-label='Spotify Popularity'
              >
                <i
                  className='fa-brands fa-spotify text-green-500'
                  aria-hidden='true'
                />
                <span>
                  <span className='text-slate-500'>Popularity:</span>{" "}
                  <span className='text-green-400 font-medium'>
                    {Math.round(album.spotify_popularity.Float64)}
                  </span>
                </span>
              </li>
            )}
          </ul>

          {/* Genre tags */}
          {album_genres.length > 0 && (
            <ul
              className='flex flex-wrap gap-2 mt-4'
              aria-label={`Genres: ${album_genres.join(", ")}`}
            >
              {album_genres.map(genre => (
                <li
                  key={genre}
                  className='px-3 py-1 bg-slate-800/80 text-amber-200 text-sm rounded-full border border-amber-500/30 backdrop-blur-sm'
                >
                  {genre}
                </li>
              ))}
            </ul>
          )}

          {/* Play Album and Shuffle buttons */}
          <div className='mt-6 flex flex-wrap gap-3'>
            <button
              onClick={handlePlayAlbum}
              className='inline-flex items-center gap-2 px-6 py-3 bg-amber-500 text-slate-900 font-semibold rounded-full hover:bg-amber-400 transition-colors shadow-lg shadow-amber-500/20'
            >
              <i className='fa-solid fa-play' aria-hidden='true' />
              Play Album
            </button>
            <button
              onClick={handleShufflePlay}
              className='inline-flex items-center gap-2 px-6 py-3 bg-slate-700 text-white font-semibold rounded-full hover:bg-slate-600 transition-colors border border-slate-600'
              aria-label='Shuffle play album'
            >
              <i className='fa-solid fa-shuffle' aria-hidden='true' />
              Shuffle
            </button>
          </div>

          {/* Artists section */}
          {artists.length > 0 && (
            <section className='mt-6' aria-labelledby='artists-heading'>
              <h2
                id='artists-heading'
                className='text-sm font-semibold text-slate-400 uppercase tracking-wide mb-3'
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
          className='text-xl font-semibold text-white mb-4 flex items-center gap-2'
        >
          <i
            className='fa-solid fa-list-ol text-amber-400'
            aria-hidden='true'
          />
          Track List
        </h2>

        <div className='bg-slate-800/30 rounded-xl border border-amber-500/10 overflow-hidden'>
          {discNumbers.map(discNum => (
            <div key={discNum}>
              {hasMultipleDiscs && (
                <div className='px-4 py-2 bg-slate-800/50 border-b border-slate-700/50'>
                  <span className='text-sm font-medium text-slate-400'>
                    <i
                      className='fa-solid fa-compact-disc mr-2 text-amber-400/70'
                      aria-hidden='true'
                    />
                    Disc {discNum}
                  </span>
                </div>
              )}
              <ul className='divide-y divide-slate-700/30'>
                {tracksByDisc[discNum].map((track: TrackType) => (
                  <TrackRow
                    key={track.id}
                    track={track}
                    genres={trackGenreMap.get(track.id) || []}
                    isPlaying={isTrackPlaying(track)}
                    isCurrentTrack={audioPlayer.currentTrack?.id === track.id}
                    onToggle={() => handleToggleTrack(track)}
                  />
                ))}
              </ul>
            </div>
          ))}
        </div>
      </section>

      {/* Album metadata */}
      <section
        className='mt-10 p-4 bg-slate-800/30 rounded-xl border border-amber-500/10'
        aria-labelledby='details-heading'
      >
        <h2
          id='details-heading'
          className='text-lg font-semibold text-white mb-4'
        >
          Album Details
        </h2>
        <dl className='grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-6 text-sm'>
          {album.release_date.Valid && (
            <div>
              <dt className='font-semibold text-amber-300/70 uppercase tracking-wide'>
                Release Date
              </dt>
              <dd className='text-white mt-1'>
                {formatDate(album.release_date.String)}
              </dd>
            </div>
          )}
          <div>
            <dt className='font-semibold text-amber-300/70 uppercase tracking-wide'>
              Total Tracks
            </dt>
            <dd className='text-white mt-1'>{tracks.length}</dd>
          </div>
          <div>
            <dt className='font-semibold text-amber-300/70 uppercase tracking-wide'>
              Total Duration
            </dt>
            <dd className='text-white mt-1'>
              {formatDuration(total_duration)}
            </dd>
          </div>
        </dl>
      </section>

      {/* Back link */}
      <div className='mt-8'>
        <Link
          to='/'
          className='inline-flex items-center gap-2 text-slate-400 hover:text-amber-400 transition-colors'
        >
          <i className='fa-solid fa-arrow-left' aria-hidden='true' />
          Back to Home
        </Link>
      </div>
    </article>
  );
}

function ArtistBadge({ artist }: { artist: ArtistType }) {
  return (
    <div className='flex items-center gap-2 px-3 py-1.5 bg-slate-800/60 rounded-full border border-slate-700/50 hover:border-amber-500/30 transition-colors'>
      {artist.thumb.Valid ? (
        <img
          src={artist.thumb.String}
          alt=''
          className='w-6 h-6 rounded-full object-cover'
        />
      ) : (
        <div className='w-6 h-6 rounded-full bg-slate-700 flex items-center justify-center'>
          <i
            className='fa-solid fa-user text-slate-500 text-xs'
            aria-hidden='true'
          />
        </div>
      )}
      <span className='text-sm text-white font-medium'>{artist.name}</span>
    </div>
  );
}

type TrackRowProps = {
  track: TrackType;
  genres: string[];
  isPlaying: boolean;
  isCurrentTrack: boolean;
  onToggle: () => void;
};

function TrackRow({
  track,
  genres,
  isPlaying,
  isCurrentTrack,
  onToggle,
}: TrackRowProps) {
  return (
    <li
      className={`group flex items-center gap-4 px-4 py-3 hover:bg-slate-800/50 transition-colors ${
        isCurrentTrack ? "bg-slate-800/40" : ""
      }`}
    >
      {/* Track number / Playing indicator */}
      <span className='w-8 text-center font-mono text-sm'>
        {isPlaying ? (
          <i
            className='fa-solid fa-volume-high text-amber-400 animate-pulse'
            aria-hidden='true'
          />
        ) : (
          <span
            className={`${isCurrentTrack ? "text-amber-400" : "text-slate-500"} group-hover:text-amber-400`}
          >
            {track.track_index}
          </span>
        )}
      </span>

      {/* Track info */}
      <div className='flex-1 min-w-0'>
        <p
          className={`font-medium truncate ${isCurrentTrack ? "text-amber-400" : "text-white"}`}
        >
          {track.title}
        </p>
        <div className='flex items-center gap-2 text-xs text-slate-500 mt-0.5'>
          {track.composer.Valid && (
            <span className='truncate'>
              <i className='fa-solid fa-pen-nib mr-1' aria-hidden='true' />
              {track.composer.String}
            </span>
          )}
          {genres.length > 0 && (
            <span className='truncate text-amber-400/60'>
              {genres.join(", ")}
            </span>
          )}
        </div>
      </div>

      {/* Track metadata */}
      <div className='flex items-center gap-4 text-xs text-slate-500'>
        <span
          className='hidden sm:block'
          title={`${track.codec} • ${formatBitRate(track.bit_rate)}`}
        >
          {track.codec.toUpperCase()}
        </span>
        <span className='w-12 text-right'>
          {formatTrackDuration(track.duration)}
        </span>
      </div>

      {/* Play/Pause button */}
      <button
        onClick={onToggle}
        className={`w-8 h-8 flex items-center justify-center rounded-full bg-amber-500 text-slate-900 hover:bg-amber-400 transition-all ${
          isCurrentTrack ? "opacity-100" : "opacity-0 group-hover:opacity-100"
        }`}
        title={isPlaying ? "Pause track" : "Play track"}
        aria-label={isPlaying ? `Pause ${track.title}` : `Play ${track.title}`}
      >
        {isPlaying ? (
          <i className='fa-solid fa-pause text-xs' aria-hidden='true' />
        ) : (
          <i className='fa-solid fa-play text-xs ml-0.5' aria-hidden='true' />
        )}
      </button>
    </li>
  );
}
