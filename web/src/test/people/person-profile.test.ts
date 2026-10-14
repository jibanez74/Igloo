import { describe, expect, it } from "vitest";
import { personSearchSchema } from "@/lib/route-search";
import { filterPersonTitles, knownForTitles, personAge, personDateLabel, personExternalLinks, personReturnTo } from "@/lib/person-profile";
import { personProfile, personTitle } from "../helpers/people";

describe("person profile", () => {
  it("calculates calendar ages and omits invalid or future birthdays", () => {
    expect(personAge("1980-05-12", "", new Date("2026-05-11T12:00:00Z"))).toBe(45);
    expect(personAge("1980-05-12", "", new Date("2026-05-12T00:00:00Z"))).toBe(46);
    expect(personAge("1980-05-12", "2020-05-11")).toBe(39);
    expect(personAge("2000-02-29", "2024-02-29")).toBe(24);
    expect(personAge("2020-02-30", "")).toBeNull();
    expect(personAge("", "")).toBeNull();
    expect(personAge("1980-05-12", "not-a-date")).toBeNull();
    expect(personAge("2099-01-01", "", new Date("2026-01-01"))).toBeNull();
    expect(personDateLabel("1980-05-12")).toBe("May 12, 1980");
    expect(personDateLabel("2024-99-99")).toBeNull();
  });
  it("sorts dated titles newest first, uses title ties, and puts undated titles last", () => {
    const titles = [personTitle(1, { title: "Undated", date: "" }), personTitle(2, { title: "Z", date: "2020" }), personTitle(3, { title: "A", date: "2020" }), personTitle(4, { date: "2024" })];
    expect(filterPersonTitles(titles, "all", "all", "all").map(title => title.tmdb_id)).toEqual([4, 3, 2, 1]);
  });
  it("filters grouped work without discarding roles and ranks known-for work", () => {
    const { filmography } = personProfile();
    expect(filterPersonTitles(filmography, "movie", "crew", "unavailable")[0].roles).toHaveLength(2);
    expect(filterPersonTitles(filmography, "tv", "all", "available").map(title => title.tmdb_id)).toEqual([20]);
    const titles = Array.from({ length: 12 }, (_, i) => personTitle(i, { roles: [{ kind: "crew", role: "Director", department: "Directing" }] }));
    expect(knownForTitles([...titles, ...filmography], "Acting").map(title => title.tmdb_id).slice(0, 3)).toEqual([30, 20, 10]);
    expect(knownForTitles(titles, "")).toHaveLength(8);
  });
  it("accepts only originating detail destinations and validates filter search", () => {
    expect(personReturnTo("/tv-shows/401?season=2")).toBe("/tv-shows/401?season=2");
    expect(personReturnTo("/movies/in-theaters/711")).toBe("/movies/in-theaters/711");
    for (const value of ["https://evil.test", "//evil.test", "/\\evil.test", "/people/1", "/settings", "/movies/1/../2", "/movies/1/play", 5]) expect(personReturnTo(value)).toBe("/");
    expect(personSearchSchema.parse({ media: "bad", work: "bad", library: "bad", page: -1, returnTo: "https://evil.test" })).toEqual({ media: "all", work: "all", library: "all", page: 1, returnTo: "/" });
  });
  it("constructs external profiles and rejects unsafe official websites", () => {
    const { person } = personProfile();
    expect(personExternalLinks(person)).toHaveLength(9);
    expect(personExternalLinks({ ...person, homepage: "javascript:alert(1)" }).some(link => link.label === "Official website")).toBe(false);
  });
});
