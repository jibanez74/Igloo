export const AUTH_USER_KEY = "auth-user";
export const RECENT_MOVIES_KEY = "latest-movies";
export const RECENT_SHOWS_KEY = "latest-shows";
export const RECENT_ALBUMS_KEY = "latest-albums";
export const MOVIES_KEY = "movies";
export const MOVIE_DETAILS_KEY = "movie-details";
export const ALBUMS_KEY = "albums";
export const ALBUM_DETAILS_KEY = "album-details";
export const MUSICIANS_KEY = "musicians";
export const TRACKS_KEY = "tracks";
export const SETTINGS_KEY = "settings";

export const LOGIN_API_ROUTE = "/api/auth/login";
export const AUTH_API_ROUTE = "/api/auth/user";

export const ERROR_NOTFOUND = {
  error: true,
  message: "404 - The resource you requested was not found.",
};
export const NETWORK_ERROR = {
  error: true,
  message: "500 - A network error occurred while processing your request.",
};
