import type { ShowLibraryItemType } from "../../src/types";
import { nullableInt64, nullableString } from "../e2e-api";

// The show and episodes mock-api-server.ts serves to the mocked player specs.
export const MOCK_SHOW_ID = 401;
export const MOCK_EPISODE_ID = 70103;
export const MOCK_NEXT_EPISODE_ID = 70104;

/** A show as the library and genre lists return it. */
export function libraryShow(
  id: number,
  name: string,
  premiereYear: number,
  posterPath = "",
): ShowLibraryItemType {
  return {
    id,
    name,
    poster_path: nullableString(posterPath),
    premiere_year: nullableInt64(premiereYear),
    certification: nullableString("TV-14"),
  };
}
