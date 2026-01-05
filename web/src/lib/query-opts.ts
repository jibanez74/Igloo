import { queryOptions } from "@tanstack/react-query";
import {
  getAlbumDetails,
  getAuthUser,
  getLatestAlbums,
  getMovieInTheaterDetails,
  getMoviesInTheaters,
} from "@/lib/api";
import {
  ALBUM_DETAILS_KEY,
  AUTH_USER_KEY,
  LATEST_ALBUMS_KEY,
  MOVIE_DETAILS_KEY,
  MOVIES_IN_THEATERS_KEY,
} from "@/lib/constants";

export function authUserQueryOpts() {
  return queryOptions({
    queryKey: [AUTH_USER_KEY],
    queryFn: getAuthUser,
  });
}

export function latestAlbumsQueryOpts() {
  return queryOptions({
    queryKey: [LATEST_ALBUMS_KEY],
    queryFn: getLatestAlbums,
  });
}

export function inTheatersQueryOpts() {
  return queryOptions({
    queryKey: [MOVIES_IN_THEATERS_KEY],
    queryFn: getMoviesInTheaters,
  });
}

export function movieDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MOVIE_DETAILS_KEY, id],
    queryFn: () => getMovieInTheaterDetails(id),
    enabled: id > 0,
  });
}

export function albumDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [ALBUM_DETAILS_KEY, id],
    queryFn: () => getAlbumDetails(id),
  });
}
