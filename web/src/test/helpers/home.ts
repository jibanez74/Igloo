import { jsonResponse, requestURL } from "./api";
import { authUser } from "./fixtures";

/** An authenticated Home page with empty libraries and no notifications. */
export function homeShellResponse(input: RequestInfo | URL) {
  const url = requestURL(input);
  switch (url) {
    case "/api/auth/user":
      return jsonResponse(authUser());
    case "/api/watch-rooms":
      return jsonResponse({ error: false, data: { rooms: [] } });
    case "/api/continue-watching":
      return jsonResponse({ error: false, data: { items: [] } });
    case "/api/movies/latest":
    case "/api/tmdb/movies/in-theaters":
      return jsonResponse({ error: false, data: { movies: [] } });
    case "/api/shows/latest":
      return jsonResponse({ error: false, data: { shows: [] } });
    case "/api/music/albums/latest":
      return jsonResponse({ error: false, data: { albums: [] } });
    case "/api/notifications/unread-count":
      return jsonResponse({ error: false, data: { unread_count: 0 } });
    default:
      throw new Error(`Unexpected request: ${url}`);
  }
}
