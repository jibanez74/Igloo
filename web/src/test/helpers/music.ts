// Contract-typed music builders shared by the unit tests and the e2e specs
// (e2e/fixtures/music.ts re-exports them). Each takes the fields a test cares
// about and fills the rest with neutral defaults.

import type { components } from "@/types/openapi.gen";
import { nullableInt64, nullableString, userSummary } from "./fixtures";

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
    artwork: [],
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
    owner: userSummary(),
    collaborators: null,
    ...fields,
  };
}

/** An album page track; its musician is albumId + 100, as albumDetailsResponse names it. */
export function albumTrack(
  id: number,
  albumId: number,
  title: string,
  trackIndex: number,
): Schema["AlbumTrack"] {
  return {
    id,
    title,
    codec: "flac",
    mime_type: "audio/flac",
    track_index: trackIndex,
    duration: 180_000,
    disc: 1,
    channel_layout: "stereo",
    bit_rate: 900000,
    explicit: false,
    album_id: nullableInt64(albumId),
    musician_id: nullableInt64(albumId + 100),
  };
}

/** An album page answer with one track, one artist and one genre. */
export function albumDetailsResponse(
  id: number,
  title: string,
  artistName: string,
  trackTitle: string,
): Schema["AlbumDetailsData"] {
  return {
    album: {
      id,
      title,
      sort_title: title,
      musician: nullableString(artistName),
      spotify_id: nullableString(`spotify-${id}`),
      apple_id: nullableInt64(),
      apple_url: null,
      spotify_url: `https://open.spotify.com/album/spotify-${id}`,
      album_type: nullableString("album"),
      release_date: nullableString("2026-01-01"),
      year: nullableInt64(2026),
      total_tracks: nullableInt64(1),
      cover: nullableString(`/covers/${id}.jpg`),
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    tracks: [albumTrack(id * 10, id, trackTitle, 1)],
    artists: [
      {
        id: id + 100,
        name: artistName,
        thumb: nullableString(""),
      },
    ],
    track_genres: [
      {
        track_id: id * 10,
        tag: "Alternative",
      },
    ],
    album_genres: ["Alternative"],
    total_duration: 180_000,
  };
}
