import type {
  LibraryMovieDetailsResponse,
  MoviePlaylistSummaryType,
  MovieTechnicalDetailsResponse,
  MoviesLibraryListItemType,
  PlaybackSettingsType,
  WatchProgressType,
  PrerollTrailer,
} from "../../src/types";
import { nullableFloat64, nullableInt64, nullableString } from "../e2e-api";

/** The movie mock-api-server.ts serves to the mocked player specs. */
export const MOCK_MOVIE_ID = 101;

/** A movie as the library, genre and liked lists return it. */
export function libraryMovie(
  id: number,
  title: string,
  year: number,
  posterPath = "",
): MoviesLibraryListItemType {
  return {
    id,
    title,
    poster_path: nullableString(posterPath),
    year: nullableInt64(year),
    certification: nullableString("PG-13"),
  };
}

/** A movie playlist as GET /api/movies/playlists lists it; a shared one belongs to user 2. */
export function moviePlaylist(
  id: number,
  name: string,
  movieCount: number,
  isOwner: boolean,
  description: string,
  coverImage = "",
): MoviePlaylistSummaryType {
  return {
    id,
    user_id: isOwner ? 1 : 2,
    name,
    description: nullableString(description),
    cover_image: nullableString(coverImage),
    is_public: false,
    movie_id: nullableInt64(),
    content_type: "movie",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    movie_count: movieCount,
    is_owner: isOwner,
    can_edit: isOwner,
  };
}

/** A list movie's details, as its card prefetches them. */
export function libraryMovieDetails(
  movie: MoviesLibraryListItemType,
): LibraryMovieDetailsResponse {
  return {
    movie: {
      id: movie.id,
      title: movie.title,
      adult: false,
      tmdb_id: nullableInt64(movie.id),
      imdb_id: nullableString(`tt${movie.id.toString().padStart(7, "0")}`),
      poster_path: movie.poster_path,
      backdrop_path: nullableString(),
      language: nullableString("en"),
      year: movie.year,
      release_date: nullableString(`${movie.year.Int64}-01-01`),
      overview: nullableString(`${movie.title} overview`),
      tag_line: nullableString(),
      certification: movie.certification,
      critic_rating: nullableFloat64(),
      audience_rating: nullableFloat64(),
      revenue: nullableFloat64(),
      budget: nullableFloat64(),
      run_time: nullableInt64(120),
      duration: nullableFloat64(7200),
    },
    cast: [],
    crew: [],
    genres: [],
    production_companies: [],
    extra_videos: [],
  };
}

function libraryMovieTechnicalDetails(
  movie: MoviesLibraryListItemType,
): MovieTechnicalDetailsResponse {
  return {
    movie: {
      file_name: `${movie.title}.mp4`,
      size: 0,
      container: "mp4",
      mime_type: "video/mp4",
      run_time: nullableInt64(120),
      duration: nullableFloat64(7200),
    },
    video_streams: [],
    audio_streams: [],
    subtitles: [],
    chapters: [],
  };
}

const noWatchProgress: WatchProgressType = {
  progress_sec: null,
  duration_sec: null,
  watched: false,
  updated_at: null,
};

const defaultPlaybackSettings: { settings: PlaybackSettingsType } = {
  settings: {
    profiles: [],
    server_upload_mbps: null,
    hardware_acceleration_device: "cpu",
    effective_hardware_acceleration_device: "cpu",
    hardware_fallback_reason: "",
    max_transcode_height: 1080,
  },
};

/**
 * The body for whatever pointing at a list movie's card can request: the
 * card's details prefetch, and the router's intent preload of its details and
 * play links. A pointer can land on a card whenever a click re-renders the
 * grid under it, so a spec that stubs every request answers these too.
 * Returns undefined for any other path or an id not in `movies`.
 */
export function movieCardPreload(
  pathname: string,
  movies: Map<number, MoviesLibraryListItemType>,
): unknown {
  if (pathname === "/api/settings/playback") {
    return defaultPlaybackSettings;
  }

  const match = pathname.match(
    /^\/api\/movies\/(?:details\/(\d+)|(\d+)\/(technical-details|like-status|watch-progress))$/,
  );
  const movie = match && movies.get(Number(match[1] ?? match[2]));
  if (!movie) {
    return undefined;
  }

  switch (match[3]) {
    case "technical-details":
      return libraryMovieTechnicalDetails(movie);
    case "like-status":
      return { is_liked: false };
    case "watch-progress":
      return noWatchProgress;
    default:
      return libraryMovieDetails(movie);
  }
}

/**
 * `featured` followed by generated items up to a full page of `perPage`, every
 * other one with a poster, so a grid renders both card variants.
 */
export function fillLibraryPage<T>(
  featured: T[],
  { prefix, startId, perPage }: { prefix: string; startId: number; perPage: number },
  build: (id: number, title: string, year: number, posterPath: string) => T,
) {
  const slug = prefix.toLowerCase().replaceAll(" ", "-");

  return [
    ...featured,
    ...Array.from({ length: perPage - featured.length }, (_, index) =>
      build(
        startId + index,
        `${prefix} ${index + 1}`,
        2000 + ((index + 1) % 20),
        index % 2 === 0 ? `/${slug}-${index + 1}.jpg` : "",
      ),
    ),
  ];
}

/** The trailer queue the mock server plays before MOCK_MOVIE_ID when the pre-roll is on. */
export function prerollTrailers(): PrerollTrailer[] {
  return [
    {
      title: "Glacier Run",
      youtube_key: "glacier-run-trailer",
      source: "theaters",
      movie_id: null,
      tmdb_id: 9001,
    },
    {
      title: "North Light",
      youtube_key: "north-light-trailer",
      source: "library",
      movie_id: 102,
      tmdb_id: 102,
    },
  ];
}
