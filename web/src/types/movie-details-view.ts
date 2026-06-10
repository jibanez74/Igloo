export type MovieCastMemberView = {
  id: number;
  name: string;
  character: string;
  imageUrl: string | null;
};

export type MovieCrewMemberView = {
  id: number;
  job: string;
  name: string;
};

export type MovieGenreView = {
  id: number;
  name: string;
};

export type MovieDetailsGridItemView = {
  label: string;
  value: string;
  ariaLabel?: string;
};

export type MovieExtraVideoView = {
  id: number;
  title: string;
  key: string;
  type: string;
  site: string;
  official: boolean;
};

export type MovieProductionCompanyView = {
  id: number;
  name: string;
  logoUrl: string | null;
};

export type MovieDetailsView = {
  id: number;
  title: string;
  posterUrl: string | null;
  backdropUrl: string | null;
  tagline: string | null;
  overview: string | null;
  releaseYear: number | null;
  releaseDate: string | null;
  runtimeMinutes: number | null;
  runtimeFormatted: string | null;
  rating: number | null;
  genres: MovieGenreView[];
  cast: MovieCastMemberView[];
  director: { name: string } | null;
  writers: MovieCrewMemberView[];
  productionCompanies: MovieProductionCompanyView[];
  trailerKey: string | null;
  extraVideos: MovieExtraVideoView[];
  detailsGridItems: MovieDetailsGridItemView[];
};
