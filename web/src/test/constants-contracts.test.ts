import { describe, expect, it } from "vitest";
import {
  ALBUMS_PAGINATED_KEY,
  ALBUMS_PER_PAGE,
  FOCUS_VISIBLE_RING_CLASS,
  LIKED_TRACKS_KEY,
  LIKED_TRACKS_PER_PAGE,
  MOVIES_BY_GENRE_KEY,
  MOVIES_LIBRARY_KEY,
  MOVIES_LIKED_KEY,
  MOVIES_PER_PAGE,
  MUSICIANS_PAGINATED_KEY,
  MUSICIANS_PER_PAGE,
  NOTIFICATION_TITLES,
  PLAYLIST_TRACKS_KEY,
  PLAYLIST_TRACKS_PAGE_SIZE,
  SEARCH_MOVIES_KEY,
  SEARCH_PER_PAGE,
  STREAM_MODE_IDS,
  STREAM_MODES,
  TRACKS_INFINITE_KEY,
  TRACKS_INFINITE_PAGE_SIZE,
  WATCH_ROOM_CLIENT_EVENT_TYPES,
  WATCH_ROOM_EVENT_TYPES,
} from "@/lib/constants";
import {
  albumsPaginatedQueryOpts,
  likedMoviesQueryOpts,
  likedTracksQueryOpts,
  moviesByGenreQueryOpts,
  moviesLibraryQueryOpts,
  musiciansPaginatedQueryOpts,
  playlistTracksInfiniteQueryOpts,
  searchMoviesQueryOpts,
  tracksInfiniteQueryOpts,
} from "@/lib/query-opts";

describe("constants contracts", () => {
  it("keeps the single shadcn focus-ring recipe (design-system §1.7)", () => {
    expect(FOCUS_VISIBLE_RING_CLASS).toBe(
      "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-hidden",
    );
  });

  it("derives stream mode ids from stream modes", () => {
    expect(STREAM_MODE_IDS).toEqual(STREAM_MODES.map((mode) => mode.id));
  });

  it("keeps notification and watch-room protocol values stable", () => {
    expect(Object.values(NOTIFICATION_TITLES)).toEqual([
      "movie_request",
      "album_request",
      "track_request",
      "other",
    ]);
    expect(Object.values(WATCH_ROOM_EVENT_TYPES)).toEqual([
      "room_snapshot",
      "playback_changed",
      "member_joined",
      "member_left",
      "room_deleted",
      "pong",
    ]);
    expect(Object.values(WATCH_ROOM_CLIENT_EVENT_TYPES)).toEqual([
      "play",
      "pause",
      "seek",
      "ping",
    ]);
  });

  it("uses shared default page sizes in query option keys", () => {
    expect(moviesLibraryQueryOpts(1).queryKey).toEqual([
      MOVIES_LIBRARY_KEY,
      1,
      MOVIES_PER_PAGE,
      "asc",
    ]);
    expect(moviesByGenreQueryOpts(7, 1).queryKey).toEqual([
      MOVIES_BY_GENRE_KEY,
      7,
      1,
      MOVIES_PER_PAGE,
      "asc",
    ]);
    expect(likedMoviesQueryOpts(1).queryKey).toEqual([
      MOVIES_LIKED_KEY,
      1,
      MOVIES_PER_PAGE,
      "asc",
    ]);
    expect(albumsPaginatedQueryOpts(1).queryKey).toEqual([
      ALBUMS_PAGINATED_KEY,
      1,
      ALBUMS_PER_PAGE,
    ]);
    expect(musiciansPaginatedQueryOpts(1).queryKey).toEqual([
      MUSICIANS_PAGINATED_KEY,
      1,
      MUSICIANS_PER_PAGE,
    ]);
    expect(searchMoviesQueryOpts(" Casino ", 2).queryKey).toEqual([
      SEARCH_MOVIES_KEY,
      "Casino",
      2,
      SEARCH_PER_PAGE,
    ]);
    expect(tracksInfiniteQueryOpts().queryKey).toEqual([
      TRACKS_INFINITE_KEY,
      TRACKS_INFINITE_PAGE_SIZE,
    ]);
    expect(playlistTracksInfiniteQueryOpts(3).queryKey).toEqual([
      PLAYLIST_TRACKS_KEY,
      3,
      PLAYLIST_TRACKS_PAGE_SIZE,
    ]);
    expect(likedTracksQueryOpts(1).queryKey).toEqual([
      LIKED_TRACKS_KEY,
      1,
      LIKED_TRACKS_PER_PAGE,
    ]);
  });
});
