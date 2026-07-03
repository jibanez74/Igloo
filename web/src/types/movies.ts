import type {
  NullableFloat64,
  NullableInt64,
  NullableString,
} from "./nullable";
import type { PlaylistCollaboratorType } from "./music";

// Movie from our library (scanned) - used for Latest Movies on home (API returns poster_path; frontend builds URL)
export type LatestMovieType = {
  id: number;
  title: string;
  poster_path: NullableString;
  year: NullableInt64;
};

/** Rows from GET /api/movies/library, /liked, and playlist movie pages (includes certification). Compatible with MovieCard. */
export type MoviesLibraryListItemType = LatestMovieType & {
  certification: NullableString;
};

/** Paginated movie list payload (library, liked, playlist items). */
export type MoviesLibraryPaginatedDataType = {
  movies: MoviesLibraryListItemType[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
  sort: "asc" | "desc";
};

/** GET /api/movies/stats */
export type MoviesStatsDataType = {
  total_movies: number;
};

/** Row from GET /api/movies/genres (genres that have at least one movie). */
export type MovieGenreWithCountType = {
  genre_id: number;
  genre_tag: string;
  movie_count: number;
};

/** Full playlist row for content_type = movie (API `playlist` object). */
export type MoviePlaylistRowType = {
  id: number;
  user_id: number;
  name: string;
  description: NullableString;
  cover_image: NullableString;
  is_public: boolean;
  folder_id: NullableInt64;
  movie_id: NullableInt64;
  content_type: string;
  created_at: string;
  updated_at: string;
};

/** GET /api/movies/playlists */
export type MoviePlaylistSummaryType = MoviePlaylistRowType & {
  movie_count: number;
  is_owner: boolean;
  can_edit: boolean;
};

export type MoviePlaylistsListResponseType = {
  playlists: MoviePlaylistSummaryType[];
};

/** GET /api/movies/playlists/:id */
export type MoviePlaylistDetailResponseType = {
  playlist: MoviePlaylistRowType;
  movie_count: number;
  is_owner: boolean;
  can_edit: boolean;
  collaborators: PlaylistCollaboratorType[] | null;
};

export type CreateMoviePlaylistRequest = {
  name: string;
  description?: string;
  is_public?: boolean;
  movie_id?: number;
};

// Cast member from TMDB credits
export type CastMemberType = {
  id: number;
  name: string;
  character: string;
  profile_path: string;
  order: number;
};

// Crew member from TMDB credits
export type CrewMemberType = {
  id: number;
  name: string;
  job: string;
  department: string;
  profile_path: string;
};

// Movie currently playing in theaters (from TMDB API)
export type TheaterMovieType = {
  id: number;
  title: string;
  original_title: string;
  overview: string;
  release_date: string;
  poster_path: string;
  backdrop_path: string;
  popularity: number;
  vote_average: number;
  vote_count: number;
  adult: boolean;
  original_language: string;
  genre_ids: number[];
  video: boolean;
};

// Library movie details (from GET /api/movies/details/{id}) - full movie object (nullable fields use Go nullable wrappers)
export type LibraryMovieDetailsMovieType = {
  id: number;
  title: string;
  file_path: string;
  file_name: string;
  size: number;
  container: string;
  mime_type: string;
  adult: boolean;
  tmdb_id: NullableInt64;
  imdb_id: NullableString;
  poster_path: NullableString;
  backdrop_path: NullableString;
  language: NullableString;
  year: NullableInt64;
  release_date: NullableString;
  overview: NullableString;
  tag_line: NullableString;
  certification: NullableString;
  critic_rating: NullableFloat64;
  audience_rating: NullableFloat64;
  revenue: NullableFloat64;
  budget: NullableFloat64;
  run_time: NullableInt64;
  /** Exact container duration in seconds (ffprobe); HLS/session use. */
  duration: NullableFloat64;
  created_at: string;
  updated_at: string;
};

// Library cast row (GET /api/movies/details/:id cast array)
export type LibraryMovieCastType = {
  id: number;
  movie_id: number;
  artist_id: number;
  character: string;
  cast_order: number;
  artist_name: string;
  artist_profile: NullableString;
};

// Library crew row (GET /api/movies/details/:id crew array)
export type LibraryMovieCrewType = {
  id: number;
  movie_id: number;
  artist_id: number;
  job: string;
  department: string;
  artist_name: string;
  artist_profile: NullableString;
};

// Library genre row (GET /api/movies/details/:id genres array)
export type LibraryMovieGenreType = {
  id: number;
  tag: string;
};

// Library production company row (GET /api/movies/details/:id production_companies array)
export type LibraryMovieProductionCompanyType = {
  id: number;
  name: string;
  tmdb_id: number;
  logo: NullableString;
  country: NullableString;
};

// Library extra video (GET /api/movies/details/:id extra_videos array)
export type LibraryMovieExtraVideoType = {
  id: number;
  title: string;
  external_id: NullableString;
  key: string;
  type: string;
  site: string;
  official: boolean;
  created_at: string;
  updated_at: string;
};

// Full response from GET /api/movies/details/:id
export type LibraryMovieDetailsResponse = {
  movie: LibraryMovieDetailsMovieType;
  cast: LibraryMovieCastType[];
  crew: LibraryMovieCrewType[];
  genres: LibraryMovieGenreType[];
  production_companies: LibraryMovieProductionCompanyType[];
  extra_videos: LibraryMovieExtraVideoType[];
};

// Technical details response (GET /api/movies/:id/technical-details)
export type MovieTechnicalDetailsResponse = {
  movie: {
    file_name: string;
    file_path: string;
    size: number;
    container: string;
    mime_type: string;
    run_time: NullableInt64;
    /** Exact duration in seconds (ffprobe), when scanned. */
    duration: NullableFloat64;
  };
  video_streams: VideoStreamType[];
  audio_streams: AudioStreamType[];
  subtitles: SubtitleType[];
  chapters: ChapterType[];
};

export type MovieWatchProgressType = {
  progress_sec: number | null;
  duration_sec: number | null;
  watched: boolean;
  updated_at: string | null;
};

export type VideoStreamType = {
  id: number;
  movie_id: number;
  stream_index: number;
  codec: string;
  codec_profile: NullableString;
  codec_level: NullableInt64;
  bit_rate: number;
  width: number;
  height: number;
  coded_width: NullableInt64;
  coded_height: NullableInt64;
  aspect_ratio: NullableString;
  frame_rate: number;
  avg_frame_rate: NullableString;
  bit_depth: NullableInt64;
  color_range: NullableString;
  color_space: NullableString;
  color_primaries: NullableString;
  color_transfer: NullableString;
  language: NullableString;
  title: NullableString;
};

export type AudioStreamType = {
  id: number;
  movie_id: number;
  stream_index: number;
  codec: string;
  codec_profile: NullableString;
  bit_rate: number;
  sample_rate: NullableInt64;
  channels: number;
  channel_layout: NullableString;
  language: NullableString;
  title: NullableString;
};

export type SubtitleType = {
  id: number;
  movie_id: number;
  stream_index: number;
  codec: string;
  language: NullableString;
  title: NullableString;
  is_forced: boolean;
  is_default: boolean;
};

export type ChapterType = {
  id: number;
  title: string;
  start_time: number;
  thumb: NullableString;
  movie_id: NullableInt64;
};

// TMDB search result (from POST /api/movies/:id/tmdb-search)
export type TmdbSearchResultType = {
  tmdb_id: number;
  title: string;
  release_date: string;
  overview: string;
  poster_path: string;
  already_in_library: boolean;
  library_movie_id?: number;
};

export type TmdbSearchMoviesRequest = {
  title: string;
  year?: number;
  tmdb_id?: number;
};

export type TmdbStatusType = {
  available: boolean;
};

export type UpdateMovieMetadataRequest = {
  title?: string;
  year?: number;
  release_date?: string;
  overview?: string;
  tag_line?: string;
  certification?: string;
  poster_path?: string;
  backdrop_path?: string;
  language?: string;
};

// Full movie details including credits and videos (from TMDB API)
export type MovieDetailsType = {
  id: number;
  title: string;
  original_title: string;
  overview: string;
  release_date: string;
  poster_path: string;
  backdrop_path: string;
  popularity: number;
  vote_average: number;
  vote_count: number;
  adult: boolean;
  original_language: string;
  genre_ids: number[];
  video: boolean;
  runtime: number;
  status: string;
  tagline: string;
  budget: number;
  revenue: number;
  homepage: string;
  imdb_id: string;
  production_companies: {
    id: number;
    logo_path: string;
    name: string;
    origin_country: string;
  }[];
  genres: {
    id: number;
    name: string;
  }[];
  credits: {
    cast: CastMemberType[];
    crew: CrewMemberType[];
  };
  videos: {
    results: {
      id: string;
      key: string;
      name: string;
      site: string;
      type: string;
      official: boolean;
    }[];
  };
};
