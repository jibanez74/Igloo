import type {
  MoviesLibraryListItemType,
} from "./movies";
import type {
  SimpleAlbumType,
  SimpleMusicianType,
  TrackListItemType,
} from "./music";

export type SearchSection<T> = {
  results: T[];
  total: number;
};

export type SearchAllResponseType = {
  query: string;
  movies: SearchSection<MoviesLibraryListItemType>;
  albums: SearchSection<SimpleAlbumType>;
  musicians: SearchSection<SimpleMusicianType>;
  tracks: SearchSection<TrackListItemType>;
};

type PaginatedSearchResponse<T> = {
  query: string;
  results: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
};

export type SearchMoviesResponseType = PaginatedSearchResponse<MoviesLibraryListItemType>;
export type SearchAlbumsResponseType = PaginatedSearchResponse<SimpleAlbumType>;
export type SearchMusiciansResponseType = PaginatedSearchResponse<SimpleMusicianType>;
export type SearchTracksResponseType = PaginatedSearchResponse<TrackListItemType>;

export type SearchTab = "all" | "movies" | "albums" | "musicians" | "tracks";
