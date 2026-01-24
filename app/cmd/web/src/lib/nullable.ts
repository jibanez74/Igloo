/**
 * Helper functions for unwrapping nullable types from the Go backend.
 *
 * The backend uses Go's sql.NullString, sql.NullInt64, sql.NullFloat64 types
 * which serialize to { Valid: boolean, String/Int64/Float64: value }.
 * These helpers provide a cleaner API for accessing the values.
 */

import type { NullableString, NullableInt64, NullableFloat64 } from "@/types";

/**
 * Unwraps a NullableString, returning the string value or null if not valid.
 *
 * @example
 * const coverUrl = unwrapString(album.cover);
 * // Instead of: album.cover.Valid ? album.cover.String : null
 */
export function unwrapString(
  nullable: NullableString | null | undefined
): string | null {
  return nullable?.Valid ? nullable.String : null;
}

/**
 * Unwraps a NullableString, returning the string value or undefined if not valid.
 * Useful for optional props that expect string | undefined.
 *
 * @example
 * <Component title={unwrapStringOrUndefined(item.title)} />
 */
export function unwrapStringOrUndefined(
  nullable: NullableString | null | undefined
): string | undefined {
  return nullable?.Valid ? nullable.String : undefined;
}

/**
 * Unwraps a NullableInt64, returning the number value or null if not valid.
 *
 * @example
 * const albumId = unwrapInt(track.album_id);
 * // Instead of: track.album_id.Valid ? Number(track.album_id.Int64) : null
 */
export function unwrapInt(
  nullable: NullableInt64 | null | undefined
): number | null {
  return nullable?.Valid ? nullable.Int64 : null;
}

/**
 * Unwraps a NullableInt64, returning the number value or undefined if not valid.
 */
export function unwrapIntOrUndefined(
  nullable: NullableInt64 | null | undefined
): number | undefined {
  return nullable?.Valid ? nullable.Int64 : undefined;
}

/**
 * Unwraps a NullableFloat64, returning the number value or null if not valid.
 *
 * @example
 * const popularity = unwrapFloat(album.spotify_popularity);
 */
export function unwrapFloat(
  nullable: NullableFloat64 | null | undefined
): number | null {
  return nullable?.Valid ? nullable.Float64 : null;
}

/**
 * Unwraps a NullableFloat64, returning the number value or undefined if not valid.
 */
export function unwrapFloatOrUndefined(
  nullable: NullableFloat64 | null | undefined
): number | undefined {
  return nullable?.Valid ? nullable.Float64 : undefined;
}
