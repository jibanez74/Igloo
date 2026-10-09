// Contract-typed music builders shared by the unit tests and the e2e specs
// (e2e/fixtures/music.ts re-exports them). Each takes the fields a test cares
// about and fills the rest with neutral defaults.

import type { components } from "@/types/openapi.gen";
import { nullableInt64, nullableString } from "./fixtures";

type Schema = components["schemas"];

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

/** The bare playlist row that details and create/update answer with. */
export function playlist(
  fields: Pick<Schema["Playlist"], "id" | "name"> & Partial<Schema["Playlist"]>,
): Schema["Playlist"] {
  return {
    user_id: 1,
    description: nullableString(),
    cover_image: nullableString(),
    is_public: false,
    movie_id: nullableInt64(),
    content_type: "track",
    created_at: TIMESTAMP,
    updated_at: TIMESTAMP,
    ...fields,
  };
}

export function playlistSummary(
  fields: Pick<Schema["PlaylistSummary"], "id" | "name"> &
    Partial<Schema["PlaylistSummary"]>,
): Schema["PlaylistSummary"] {
  return {
    ...playlist({ id: fields.id, name: fields.name }),
    track_count: 0,
    total_duration: 0,
    is_owner: true,
    can_edit: true,
    ...fields,
  };
}

export function playlistTrack(
  fields: Pick<Schema["PlaylistTrack"], "id" | "title" | "position"> &
    Partial<Schema["PlaylistTrack"]>,
): Schema["PlaylistTrack"] {
  return {
    playlist_track_id: 100 + fields.id,
    added_at: TIMESTAMP,
    added_by: nullableInt64(),
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    album_id: nullableInt64(),
    musician_id: nullableInt64(),
    album_title: nullableString(),
    album_cover: nullableString(),
    musician_name: nullableString(),
    ...fields,
  };
}

export function playlistDetail(
  fields: Pick<Schema["MusicPlaylistDetailData"], "playlist"> &
    Partial<Schema["MusicPlaylistDetailData"]>,
): Schema["MusicPlaylistDetailData"] {
  return {
    track_count: 0,
    duration: 0,
    is_owner: true,
    can_edit: true,
    collaborators: null,
    ...fields,
  };
}
