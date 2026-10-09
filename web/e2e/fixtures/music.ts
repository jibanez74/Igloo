import type { components } from "../../src/types/openapi.gen";
import { nullableFloat64, nullableInt64, nullableString } from "../e2e-api";

// Contract-typed music fixtures shared by the music specs: the builders come
// from the unit-test helpers, and the Glacier Sessions / Aurora Pines scenario
// below is built from them.

export {
  playlist,
  playlistDetail,
  playlistSummary,
  playlistTrack,
  simpleAlbum,
  simpleMusician,
  trackListItem,
} from "../../src/test/helpers/music";

type Schema = components["schemas"];

type AlbumDetails = Schema["AlbumDetailsData"];

export type MusicianDetails = Schema["MusicianDetailsData"];

const TIMESTAMP = "2026-01-01T00:00:00Z";

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
