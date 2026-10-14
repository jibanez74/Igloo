import type { PersonData, PersonTitle } from "@/types/people";

export function personTitle(id: number, overrides: Partial<PersonTitle> = {}): PersonTitle {
  return { tmdb_id: id, media_type: "movie", title: `Title ${id}`, date: "2024-01-01", poster_path: "", popularity: id, library_id: null, roles: [{ kind: "acting", role: "Hero", department: "Acting" }], ...overrides };
}

export function personProfile(overrides: Partial<PersonData> = {}): PersonData {
  return {
    person: { tmdb_id: 9001, name: "Alex Vega", biography: "First paragraph.\n\nSecond paragraph.", known_for_department: "Acting", birthday: "1980-05-12", deathday: "", place_of_birth: "San Juan", gender: 3, also_known_as: ["Alexandra Vega"], profile_path: "/alex.jpg", homepage: "https://example.com", external_ids: { imdb_id: "nm9001", facebook_id: "alex", instagram_id: "alex", twitter_id: "alex", tiktok_id: "alex", youtube_id: "UCalex", wikidata_id: "Q9001" } },
    filmography: [personTitle(10, { title: "Library Movie", library_id: 711 }), personTitle(20, { title: "Library Show", media_type: "tv", library_id: 401 }), personTitle(30, { title: "Remote Movie", roles: [{ kind: "acting", role: "Hero", department: "Acting" }, { kind: "crew", role: "Director", department: "Directing" }] })],
    photos: ["/extra.jpg"], enrichment_status: "available", ...overrides,
  };
}
