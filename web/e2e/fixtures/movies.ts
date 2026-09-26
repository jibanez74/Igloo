import type { MoviesLibraryListItemType } from "../../src/types";
import { nullableInt64, nullableString } from "../e2e-api";

/** The movie mock-api-server.ts serves to the mocked player specs. */
export const MOCK_MOVIE_ID = 101;

/** A movie as the library, genre and liked lists return it. */
export function libraryMovie(
  id: number,
  title: string,
  year: number,
  posterPath = "",
): MoviesLibraryListItemType {
  return {
    id,
    title,
    poster_path: nullableString(posterPath),
    year: nullableInt64(year),
    certification: nullableString("PG-13"),
  };
}

/**
 * `featured` followed by generated items up to a full page of `perPage`, every
 * other one with a poster, so a grid renders both card variants.
 */
export function fillLibraryPage<T>(
  featured: T[],
  { prefix, startId, perPage }: { prefix: string; startId: number; perPage: number },
  build: (id: number, title: string, year: number, posterPath: string) => T,
) {
  const slug = prefix.toLowerCase().replaceAll(" ", "-");

  return [
    ...featured,
    ...Array.from({ length: perPage - featured.length }, (_, index) =>
      build(
        startId + index,
        `${prefix} ${index + 1}`,
        2000 + ((index + 1) % 20),
        index % 2 === 0 ? `/${slug}-${index + 1}.jpg` : "",
      ),
    ),
  ];
}
