// MOVIE TYPES
// Types for movie listings and details from TMDB API

// Simple movie type for basic listings
export type SimpleMovieType = {
  id: number;
  title: string;
  thumb: string;
};

// Movie from our library (scanned) - used for Latest Movies on home; poster_path is TMDB path, frontend builds URL
export type LatestMovieType = {
  id: number;
  title: string;
  poster_path: string | null;
  year: number | null;
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

// Library movie details (from GET /api/movies/details/{id}) - minimal shape for play page; poster_path is TMDB path
export type LibraryMovieDetailsMovieType = {
  id: number;
  title: string;
  poster_path: string | null;
  year: number | null;
};

// Library API cast member (GET /api/movies/details/{id})
export type LibraryCastMemberType = {
  id: number;
  movie_id: number;
  artist_id: number;
  character: string;
  cast_order: number;
  artist_name: string;
  artist_profile: string | null;
};

// Library API crew member
export type LibraryCrewMemberType = {
  id: number;
  movie_id: number;
  artist_id: number;
  job: string;
  department: string;
  artist_name: string;
  artist_profile: string | null;
};

// Library API genre (tag from DB)
export type LibraryGenreType = { id: number; tag: string };

// Library API extra video (trailer, featurette, etc.)
export type LibraryExtraVideoType = {
  id: number;
  title: string;
  external_id: string | null;
  key: string;
  type: string;
  site: string;
  official: boolean;
};

// Library API production company (logo is TMDB path; frontend builds URL)
export type LibraryProductionCompanyType = {
  id: number;
  name: string;
  tmdb_id: number;
  logo: string | null;
  country: string | null;
};

// Full library movie details API response (GET /api/movies/details/{id}); image fields are paths only
export type LibraryMovieDetailsResponse = {
  movie: {
    id: number;
    title: string;
    poster_path: string | null;
    backdrop_path?: string | null;
    year: number | null;
    release_date?: string | null;
    overview?: string | null;
    tag_line?: string | null;
    run_time?: number | null;
    audience_rating?: number | null;
    critic_rating?: number | null;
    budget?: number | null;
    revenue?: number | null;
    language?: string | null;
    imdb_id?: string | null;
    tmdb_id?: number | null;
    [key: string]: unknown;
  };
  cast: LibraryCastMemberType[];
  crew: LibraryCrewMemberType[];
  genres: LibraryGenreType[];
  production_companies: LibraryProductionCompanyType[];
  extra_videos: LibraryExtraVideoType[];
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
