/**
 * Nullable helpers: unwrap backend nullable types (Valid + value) into plain values.
 * Works with nullable database wrapper types from the Go backend.
 * — same shape from the backend. Also accepts already-plain values for API responses that send strings/numbers.
 */

/** Shape of nullable string from backend. */
type NullableStringLike = { Valid: boolean; String: string };

/** Shape of nullable int from backend. */
type NullableInt64Like = { Valid: boolean; Int64: number };

/** Shape of nullable float from backend (NullableFloat64, etc.) */
type NullableFloat64Like = { Valid: boolean; Float64: number };

/**
 * Returns a string or null. Accepts nullable object (Valid + String) or plain string.
 * Use for poster_path, backdrop_path, and any DB/API string that may be nullable.
 *
 * @example
 * import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
 * const url = buildTmdbImageUrl(unwrapString(movie.poster_path), TMDB_POSTER_SIZE);
 */
export function unwrapString(
  value: NullableStringLike | string | null | undefined,
): string | null {
  if (value == null) return null;
  if (typeof value === "string") return value;
  return value.Valid ? value.String : null;
}

/**
 * Same as unwrapString but returns undefined when not valid.
 * Useful for optional props that expect string | undefined.
 */
export function unwrapStringOrUndefined(
  value: NullableStringLike | string | null | undefined,
): string | undefined {
  if (value == null) return undefined;
  if (typeof value === "string") return value;
  return value.Valid ? value.String : undefined;
}

/**
 * Returns a number or null. Accepts nullable object (Valid + Int64) or plain number.
 *
 * @example
 * const year = unwrapInt(movie.year);
 */
export function unwrapInt(
  value: NullableInt64Like | number | null | undefined,
): number | null {
  if (value == null) return null;
  if (typeof value === "number") return value;
  return value.Valid ? value.Int64 : null;
}

/**
 * Returns a number or null. Accepts nullable object (Valid + Float64) or plain number.
 *
 * @example
 * const rating = unwrapFloat(movie.audience_rating);
 */
export function unwrapFloat(
  value: NullableFloat64Like | number | null | undefined,
): number | null {
  if (value == null) return null;
  if (typeof value === "number") return value;
  return value.Valid ? value.Float64 : null;
}

/**
 * Same as unwrapFloat but returns undefined when not valid.
 */
export function unwrapFloatOrUndefined(
  value: NullableFloat64Like | number | null | undefined,
): number | undefined {
  if (value == null) return undefined;
  if (typeof value === "number") return value;
  return value.Valid ? value.Float64 : undefined;
}
