// MOVIE TYPES
// Types for movie listings and details from TMDB API

// Simple movie type for basic listings
export type SimpleMovieType = {
  id: number;
  title: string;
  thumb: string;
};

// Movie from our library (scanned) - used for Latest Movies on home
export type LatestMovieType = {
  id: number;
  title: string;
  poster: string | null;
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

// Library movie details (from GET /api/movies/details/{id}) - minimal shape for play page
export type LibraryMovieDetailsMovieType = {
  id: number;
  title: string;
  poster: string | null;
  year: number | null;
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
