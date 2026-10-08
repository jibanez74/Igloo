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

export type SearchTab =
  | "all"
  | "movies"
  | "shows"
  | "albums"
  | "musicians"
  | "tracks";

/** A category tab: each one pages through `/api/search/{tab}`. */
export type PagedSearchTab = Exclude<SearchTab, "all">;

/** Each category's `data` payload, keyed by its tab (and path segment). */
export type SearchCategoryData = {
  movies: SearchMoviesResponseType;
  shows: SearchShowsResponseType;
  albums: SearchAlbumsResponseType;
  musicians: SearchMusiciansResponseType;
  tracks: SearchTracksResponseType;
};
