// Contract-typed movie playlist builders for the unit tests. Each takes the
// fields a test cares about and fills the rest with neutral defaults, the way
// helpers/music.ts does for track playlists.

import type { components } from "@/types/openapi.gen";
import { nullableInt64, nullableString, userSummary } from "./fixtures";

type Schema = components["schemas"];

const TIMESTAMP = "2026-01-01T00:00:00Z";

/** The bare movie playlist row that details and create/update answer with. */
export function moviePlaylist(
  fields: Pick<Schema["MoviePlaylist"], "id" | "name"> &
    Partial<Schema["MoviePlaylist"]>,
): Schema["MoviePlaylist"] {
  return {
    user_id: 1,
    description: nullableString(),
    cover_image: nullableString(),
    is_public: false,
    movie_id: nullableInt64(),
    content_type: "movie",
    created_at: TIMESTAMP,
    updated_at: TIMESTAMP,
    ...fields,
  };
}

export function moviePlaylistSummary(
  fields: Pick<Schema["MoviePlaylistSummary"], "id" | "name"> &
    Partial<Schema["MoviePlaylistSummary"]>,
): Schema["MoviePlaylistSummary"] {
  return {
    ...moviePlaylist({ id: fields.id, name: fields.name }),
    movie_count: 0,
    is_owner: true,
    can_edit: true,
    ...fields,
  };
}

/** The GET /api/movies/playlists/:id payload around a playlist row. */
export function moviePlaylistDetail(
  fields: Pick<Schema["MoviePlaylistDetailData"], "playlist"> &
    Partial<Schema["MoviePlaylistDetailData"]>,
): Schema["MoviePlaylistDetailData"] {
  return {
    movie_count: 0,
    is_owner: true,
    can_edit: true,
    owner: userSummary(),
    collaborators: null,
    ...fields,
  };
}
