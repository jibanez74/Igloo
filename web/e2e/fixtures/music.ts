import type { components } from "../../src/types/openapi.gen";
import { nullableFloat64, nullableInt64, nullableString } from "../e2e-api";

// Contract-typed music fixtures shared by the music specs. Builders take the
// fields a spec cares about and fill the rest with neutral defaults.

type Schema = components["schemas"];

type AlbumDetails = Schema["AlbumDetailsData"];

export type MusicianDetails = Schema["MusicianDetailsData"];

const TIMESTAMP = "2026-01-01T00:00:00Z";

export function trackListItem(
  fields: Pick<Schema["TrackListItem"], "id" | "title"> &
    Partial<Schema["TrackListItem"]>,
): Schema["TrackListItem"] {
  return {
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    album_id: nullableInt64(),
    album_title: nullableString(),
    album_cover: nullableString(),
    musician_id: nullableInt64(),
    musician_name: nullableString(),
    ...fields,
  };
}

export function simpleAlbum(
  fields: Pick<Schema["SimpleAlbum"], "id" | "title"> &
    Partial<Schema["SimpleAlbum"]>,
): Schema["SimpleAlbum"] {
  return {
    cover: nullableString(),
    musician: nullableString(),
    year: nullableInt64(2026),
    ...fields,
  };
}

export function simpleMusician(
  fields: Pick<Schema["SimpleMusician"], "id" | "name"> &
    Partial<Schema["SimpleMusician"]>,
): Schema["SimpleMusician"] {
  return {
    thumb: nullableString(),
    album_count: 1,
    track_count: 1,
    ...fields,
  };
}

export function playlistSummary(
  fields: Pick<Schema["PlaylistSummary"], "id" | "name"> &
    Partial<Schema["PlaylistSummary"]>,
): Schema["PlaylistSummary"] {
  return {
    user_id: 1,
    description: nullableString(),
    cover_image: nullableString(),
    is_public: false,
    movie_id: nullableInt64(),
    content_type: "track",
    created_at: TIMESTAMP,
    updated_at: TIMESTAMP,
    track_count: 0,
    total_duration: 0,
    is_owner: true,
    can_edit: true,
    ...fields,
  };
}

// "Glacier Sessions" by "Aurora Pines": the album and musician pages link to
// each other, so both specs serve both.
export const GLACIER_SESSIONS_ID = 42;
export const AURORA_PINES_ID = 7;

function albumTrack(
  fields: Pick<Schema["AlbumTrack"], "id" | "title" | "track_index" | "disc" | "duration">,
): Schema["AlbumTrack"] {
  return {
    codec: "flac",
    channel_layout: "stereo",
    bit_rate: 900_000,
    album_id: nullableInt64(GLACIER_SESSIONS_ID),
    musician_id: nullableInt64(AURORA_PINES_ID),
    mime_type: "audio/flac",
    ...fields,
  };
}

function musicianTrack(
  fields: Pick<Schema["MusicianTrack"], "id" | "title" | "duration">,
): Schema["MusicianTrack"] {
  return {
    codec: "flac",
    bit_rate: 900_000,
    album_id: nullableInt64(GLACIER_SESSIONS_ID),
    album_title: nullableString("Glacier Sessions"),
    album_cover: nullableString(),
    ...fields,
  };
}

export const glacierSessions: AlbumDetails = {
  album: {
    id: GLACIER_SESSIONS_ID,
    title: "Glacier Sessions",
    sort_title: "Glacier Sessions",
    musician: nullableString("Aurora Pines"),
    spotify_id: nullableString("spotify-glacier"),
    spotify_popularity: nullableFloat64(73),
    release_date: nullableString("2026-02-14"),
    year: nullableInt64(2026),
    total_tracks: nullableInt64(3),
    cover: nullableString(),
    created_at: TIMESTAMP,
    updated_at: TIMESTAMP,
  },
  tracks: [
    albumTrack({ id: 101, title: "Northern Drift", track_index: 1, disc: 1, duration: 214_000 }),
    albumTrack({ id: 102, title: "Cold Current", track_index: 2, disc: 1, duration: 198_000 }),
    albumTrack({ id: 103, title: "Second Disc Opener", track_index: 1, disc: 2, duration: 245_000 }),
  ],
  artists: [{ id: AURORA_PINES_ID, name: "Aurora Pines", thumb: nullableString() }],
  track_genres: [
    { track_id: 101, tag: "Ambient" },
    { track_id: 102, tag: "Electronic" },
  ],
  album_genres: ["Ambient", "Electronic"],
  total_duration: 657_000,
};

export const auroraPines: MusicianDetails = {
  musician: {
    id: AURORA_PINES_ID,
    name: "Aurora Pines",
    sort_name: "Aurora Pines",
    summary: nullableString("Aurora Pines makes ambient music."),
    spotify_popularity: nullableFloat64(82),
    spotify_followers: nullableInt64(1_234_567),
    spotify_id: nullableString("sp-aurora"),
    thumb: nullableString(),
    created_at: TIMESTAMP,
    updated_at: TIMESTAMP,
  },
  albums: [
    {
      id: GLACIER_SESSIONS_ID,
      title: "Glacier Sessions",
      cover: nullableString(),
      year: nullableInt64(2026),
      release_date: nullableString("2026-02-14"),
      track_count: 2,
    },
  ],
  tracks: [
    musicianTrack({ id: 101, title: "Northern Drift", duration: 214_000 }),
    musicianTrack({ id: 102, title: "Cold Current", duration: 198_000 }),
  ],
  genres: ["Ambient", "Electronic"],
  total_duration: 412_000,
};
