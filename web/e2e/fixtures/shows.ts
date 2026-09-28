import type { ShowDetailsDataType, ShowLibraryItemType } from "../../src/types";
import { seasonEpisodes, showDetails } from "../../src/test/helpers/show-details";
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

function libraryShowDetails(show: ShowLibraryItemType): ShowDetailsDataType {
  const details = showDetails();

  return {
    ...details,
    show: {
      ...details.show,
      id: show.id,
      name: show.name,
      poster_path: show.poster_path,
      premiere_year: show.premiere_year,
      certification: show.certification,
    },
  };
}

/**
 * The body for whatever pointing at a list show's card can request: the
 * card's details prefetch, and the router's intent preload of the show route,
 * which also loads the first season's episodes. Returns undefined for any
 * other path or an id not in `shows`.
 */
export function showCardPreload(
  pathname: string,
  shows: Map<number, ShowLibraryItemType>,
): unknown {
  const match = pathname.match(
    /^\/api\/shows\/(?:details\/(\d+)|(\d+)\/seasons\/(\d+)\/episodes)$/,
  );
  const show = match && shows.get(Number(match[1] ?? match[2]));
  if (!show) {
    return undefined;
  }

  return match[3] === undefined
    ? libraryShowDetails(show)
    : seasonEpisodes(Number(match[3]));
}
