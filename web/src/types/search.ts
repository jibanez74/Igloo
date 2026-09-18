import type { components } from "./openapi.gen";

type Schema = components["schemas"];

// `data` payloads of the /api/search/* endpoints. Aliased from the named *Data
// schemas, never from the envelopes: an envelope inherits JsonSuccess.data's
// open index signature, which silently disables property checking.
export type SearchAllResponseType = Schema["SearchAllData"];
export type SearchMoviesResponseType = Schema["MovieSearchData"];
export type SearchShowsResponseType = Schema["ShowSearchData"];
export type SearchAlbumsResponseType = Schema["AlbumSearchData"];
export type SearchMusiciansResponseType = Schema["MusicianSearchData"];
export type SearchTracksResponseType = Schema["TrackSearchData"];

// The shape the five paginated payloads share, for components that render any
// category generically (see routes/_auth/search/index.tsx).
export type PaginatedSearchResponse<T> = {
  query: string;
  results: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
};

export type SearchTab =
  | "all"
  | "movies"
  | "shows"
  | "albums"
  | "musicians"
  | "tracks";
