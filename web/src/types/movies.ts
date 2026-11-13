import type { components } from "./openapi.gen";

// Movie from our library (scanned) - used for Latest Movies on home (API returns poster_path; frontend builds URL)
export type LatestMovieType = components["schemas"]["LatestMovie"];

/** Rows from GET /api/movies/library, /liked, and playlist movie pages (includes certification). Compatible with MovieCard. */
export type MoviesLibraryListItemType = components["schemas"]["MovieLibraryItem"];

/** Paginated movie list payload (library, liked, playlist items). */
export type MoviesLibraryPaginatedDataType =
  components["schemas"]["MoviesLibraryData"];

/** GET /api/movies/stats */
export type MoviesStatsDataType =
  components["schemas"]["MoviesStatsEnvelope"]["data"];

/** Row from GET /api/movies/genres (genres that have at least one movie). */
export type MovieGenreWithCountType =
  components["schemas"]["MovieGenreWithCount"];

/** GET /api/movies/genres */
export type MovieGenresDataType = components["schemas"]["MovieGenresData"];

/** Full playlist row for content_type = movie (API `playlist` object). */
export type MoviePlaylistRowType = components["schemas"]["MoviePlaylist"];

/** GET /api/movies/playlists */
export type MoviePlaylistSummaryType =
  components["schemas"]["MoviePlaylistSummary"];

export type MoviePlaylistsListResponseType =
  components["schemas"]["MoviePlaylistsData"];

/** GET /api/movies/playlists/:id */
export type MoviePlaylistDetailResponseType =
  components["schemas"]["MoviePlaylistDetailData"];

export type CreateMoviePlaylistRequest =
  components["schemas"]["CreateMoviePlaylistRequest"];

/** PUT /api/movies/playlists/:id replaces every field it names. */
export type UpdateMoviePlaylistRequest =
  components["schemas"]["UpdateMoviePlaylistRequest"];

/** POST /api/movies/playlists/:id/movies appends; duplicates count as skipped. */
export type AddMoviesToPlaylistRequest =
  components["schemas"]["AddMoviesRequest"];

export type MovieDetailsType = components["schemas"]["TmdbMovie"];
export type CrewMemberType = NonNullable<
  MovieDetailsType["credits"]["crew"]
>[number];
export type TheaterMovieType = components["schemas"]["TheaterMovie"];

// Library movie details (from GET /api/movies/details/{id}) - full movie object (nullable fields use Go nullable wrappers)
export type LibraryMovieDetailsMovieType = components["schemas"]["Movie"];

// Library cast row (GET /api/movies/details/:id cast array)
export type LibraryMovieCastType = components["schemas"]["MovieCastCredit"];

// Library crew row (GET /api/movies/details/:id crew array)
export type LibraryMovieCrewType = components["schemas"]["MovieCrewCredit"];

// Library genre row (GET /api/movies/details/:id genres array)
export type LibraryMovieGenreType = components["schemas"]["MovieGenre"];

// Library production company row (GET /api/movies/details/:id production_companies array)
export type LibraryMovieProductionCompanyType =
  components["schemas"]["MovieProductionCompany"];

// Library extra video (GET /api/movies/details/:id extra_videos array)
export type LibraryMovieExtraVideoType = components["schemas"]["ExtraVideo"];

// Full response from GET /api/movies/details/:id
export type LibraryMovieDetailsResponse =
  components["schemas"]["MovieDetailsEnvelope"]["data"];

// Technical details response (GET /api/movies/:id/technical-details)
export type MovieTechnicalDetailsResponse =
  components["schemas"]["MovieTechnicalDetailsEnvelope"]["data"];

export type VideoStreamType = components["schemas"]["VideoStream"];

export type AudioStreamType = components["schemas"]["AudioStream"];

export type SubtitleType = components["schemas"]["Subtitle"];

/** Source-capability chip (4K / HDR / 7.1 / CC) shown in the details hero. */
export type MediaCapabilityBadge = {
  /** Short chip text, e.g. "4K". */
  label: string;
  /** Accessible description, e.g. "4K Ultra HD video". */
  description: string;
};

export type ChapterType = components["schemas"]["Chapter"];

// TMDB search result (from POST /api/tmdb/movies/search)
export type TmdbSearchResultType = components["schemas"]["TmdbSearchResult"];

export type TmdbSearchMoviesRequest =
  components["schemas"]["TmdbSearchMoviesRequest"];

export type TmdbStatusType = components["schemas"]["TmdbStatusEnvelope"]["data"];

/** PUT /api/movies/:id/metadata: a field left out keeps its value. */
export type UpdateMovieMetadataRequest =
  components["schemas"]["UpdateMovieMetadataRequest"];
