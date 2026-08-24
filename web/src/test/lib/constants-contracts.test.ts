import { describe, expect, it } from "vitest";
import {
  ALBUMS_PAGINATED_KEY,
  ALBUMS_PER_PAGE,
  AUDIO_VOLUME_STEP,
  FOCUS_VISIBLE_RING_CLASS,
  HOME_ALBUM_GRID_CLASS,
  HOME_POSTER_GRID_CLASS,
  LIKED_TRACKS_KEY,
  LIKED_TRACKS_PER_PAGE,
  MOVIES_BY_GENRE_KEY,
  MOVIES_LIBRARY_KEY,
  MOVIES_LIKED_KEY,
  MOVIE_VOLUME_STEP,
  MOVIES_PER_PAGE,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MUSICIANS_PAGINATED_KEY,
  MUSICIANS_PER_PAGE,
  NOTIFICATION_TITLES,
  PLAYER_ICON_BUTTON_CLASS,
  PLAYER_PRIMARY_BUTTON_CLASS,
  PLAYER_TRANSPORT_INERT_CLASS,
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

  it("keeps player chrome buttons on the single ring recipe and shared motion (design-system §1.7, §3.5)", () => {
    for (const constant of [
      PLAYER_ICON_BUTTON_CLASS,
      PLAYER_PRIMARY_BUTTON_CLASS,
    ]) {
      expect(constant).toContain(FOCUS_VISIBLE_RING_CLASS);
      expect(constant).toContain(MOTION_PLAYER_CHROME_BUTTON_CLASS);
      // No plain `focus:` variants — rings are keyboard-only (focus-visible).
      expect(constant).not.toMatch(/(^|[^-])focus:/);
    }
    // Loading/inert transport controls stay focusable: aria-disabled, never disabled.
    expect(PLAYER_PRIMARY_BUTTON_CLASS).toContain("aria-disabled:opacity-50");
    expect(PLAYER_PRIMARY_BUTTON_CLASS).not.toMatch(/(^|\s)disabled:/);
    expect(PLAYER_TRANSPORT_INERT_CLASS).toBe(
      "aria-disabled:cursor-not-allowed aria-disabled:opacity-30",
    );
    expect(AUDIO_VOLUME_STEP).toBe(0.1);
    expect(AUDIO_VOLUME_STEP).toBe(MOVIE_VOLUME_STEP);
  });

  it("keeps home grids on auto-fill so sparse sections don't stretch (design-system §3.2)", () => {
    expect(HOME_POSTER_GRID_CLASS).toContain(
      "repeat(auto-fill,minmax(min(7.5rem,100%),1fr))",
    );
    expect(HOME_ALBUM_GRID_CLASS).toContain(
      "repeat(auto-fill,minmax(min(8rem,100%),1fr))",
    );
    expect(HOME_POSTER_GRID_CLASS).not.toContain("auto-fit");
    expect(HOME_ALBUM_GRID_CLASS).not.toContain("auto-fit");
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
