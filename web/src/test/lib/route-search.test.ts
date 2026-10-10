import { describe, expect, it } from "vitest";
import { z } from "zod/mini";
import {
  MOVIES_INDEX_DEFAULT_SEARCH,
  MUSIC_INDEX_DEFAULT_SEARCH,
  SEARCH_INDEX_DEFAULT_SEARCH,
  SHOWS_INDEX_DEFAULT_SEARCH,
} from "@/lib/constants";
import {
  moviesSearchSchema,
  musicSearchSchema,
  searchSearchSchema,
  showsSearchSchema,
} from "@/lib/route-search";

// Each index route strips its defaults on navigation with the same object its
// schema fills in, so the two must agree or a default would stay in the URL
// (or a non-default be stripped).
describe("route search defaults", () => {
  it.each([
    ["movies", moviesSearchSchema, MOVIES_INDEX_DEFAULT_SEARCH],
    ["TV shows", showsSearchSchema, SHOWS_INDEX_DEFAULT_SEARCH],
    ["music", musicSearchSchema, MUSIC_INDEX_DEFAULT_SEARCH],
    ["search", searchSearchSchema, SEARCH_INDEX_DEFAULT_SEARCH],
  ])("%s: the schema fills in exactly the stripped defaults", (_name, schema, defaults) => {
    expect(z.parse(schema, {})).toEqual(defaults);
  });
});
