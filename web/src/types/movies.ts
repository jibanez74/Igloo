import type { Float64Type, Int64Type, StringType } from "@/types/Sqlite";
import type {
  NullableInt64,
  NullableString,
  PlaylistCollaboratorType,
} from "@/types/music";

// Simple movie type for basic listings
export type SimpleMovieType = {
  id: number;
  title: string;
  thumb: string;
};

// Movie from our library (scanned) - used for Latest Movies on home (API returns poster_path; frontend builds URL)
export type LatestMovieType = {
  id: number;
  title: string;
  poster_path: StringType;
  year: Int64Type;
};

/** Rows from GET /api/movies/library, /liked, and playlist movie pages (includes certification). Compatible with MovieCard. */
export type MoviesLibraryListItemType = LatestMovieType & {
  certification: StringType;
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

// Library movie details (from GET /api/movies/details/{id}) - full movie object (nullable fields use Sqlite types)
export type LibraryMovieDetailsMovieType = {
  id: number;
  title: string;
  file_path: string;
  file_name: string;
  size: number;
  container: string;
  mime_type: string;
  adult: boolean;
  tmdb_id: Int64Type;
  imdb_id: StringType;
  poster_path: StringType;
  backdrop_path: StringType;
  language: StringType;
  year: Int64Type;
  release_date: StringType;
  overview: StringType;
  tag_line: StringType;
  certification: StringType;
  critic_rating: Float64Type;
  audience_rating: Float64Type;
  revenue: Float64Type;
  budget: Float64Type;
  run_time: Int64Type;
  /** Exact container duration in seconds (ffprobe); HLS/session use. */
  duration: Float64Type;
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
  artist_profile: StringType;
};

// Library crew row (GET /api/movies/details/:id crew array)
export type LibraryMovieCrewType = {
  id: number;
  movie_id: number;
  artist_id: number;
  job: string;
  department: string;
  artist_name: string;
  artist_profile: StringType;
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
  logo: StringType;
  country: StringType;
};

// Library extra video (GET /api/movies/details/:id extra_videos array)
export type LibraryMovieExtraVideoType = {
  id: number;
  title: string;
  external_id: StringType;
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
    run_time: Int64Type;
    /** Exact duration in seconds (ffprobe), when scanned. */
    duration: Float64Type;
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
  codec_profile: StringType;
  codec_level: Int64Type;
  bit_rate: number;
  width: number;
  height: number;
  coded_width: Int64Type;
  coded_height: Int64Type;
  aspect_ratio: StringType;
  frame_rate: number;
  avg_frame_rate: StringType;
  bit_depth: Int64Type;
  color_range: StringType;
  color_space: StringType;
  color_primaries: StringType;
  color_transfer: StringType;
  language: StringType;
  title: StringType;
};

export type AudioStreamType = {
  id: number;
  movie_id: number;
  stream_index: number;
  codec: string;
  codec_profile: StringType;
  bit_rate: number;
  sample_rate: Int64Type;
  channels: number;
  channel_layout: StringType;
  language: StringType;
  title: StringType;
};

export type SubtitleType = {
  id: number;
  movie_id: number;
  stream_index: number;
  codec: string;
  language: StringType;
  title: StringType;
  is_forced: boolean;
  is_default: boolean;
};

export type ChapterType = {
  id: number;
  title: string;
  start_time: number;
  thumb: StringType;
  movie_id: Int64Type;
};

// TMDB search result (from POST /api/movies/:id/tmdb-search)
export type TmdbSearchResultType = {
  tmdb_id: number;
  title: string;
  release_date: string;
  overview: string;
  poster_path: string;
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
