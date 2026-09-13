import type {
  ShowDetailsDataType,
  ShowEpisodeType,
  ShowSeasonEpisodesDataType,
  ShowSeasonSummaryType,
} from "@/types";
import {
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "../helpers/fixtures";

export const SHOW_ID = 401;

export function seasonSummary(
  seasonNumber: number,
  available: number,
  tmdbEpisodes: number | null = null,
): ShowSeasonSummaryType {
  return {
    id: 5000 + seasonNumber,
    season_number: seasonNumber,
    name: seasonNumber === 0 ? "Specials" : `Season ${seasonNumber}`,
    overview: nullableString("A season of television."),
    air_date: nullableString("2024-03-01"),
    poster_path: nullableString("/season.jpg"),
    tmdb_episode_count: nullableInt64(tmdbEpisodes),
    available_episode_count: available,
  };
}

export function episode(
  seasonNumber: number,
  episodeNumber: number,
): ShowEpisodeType {
  return {
    id: 70000 + seasonNumber * 100 + episodeNumber,
    episode_number: episodeNumber,
    name: `S${seasonNumber} Episode ${episodeNumber}`,
    overview: nullableString("An episode happens."),
    air_date: nullableString("2024-03-08"),
    still_path: nullableString("/still.jpg"),
    tmdb_runtime: nullableInt64(47),
    vote_average: nullableFloat64(8.1),
    vote_count: nullableInt64(220),
  };
}

// Specials sort last, exactly as GetShowSeasonSummaries orders them.
export const SEASONS = [
  seasonSummary(1, 2, 2),
  seasonSummary(2, 1, 8),
  seasonSummary(0, 1, 1),
];

export function showDetails(
  overrides: Partial<ShowDetailsDataType> = {},
): ShowDetailsDataType {
  return {
    show: {
      id: SHOW_ID,
      name: "Frost Harbor",
      original_name: nullableString("Frost Harbor"),
      premiere_year: nullableInt64(2024),
      tmdb_id: nullableInt64(90210),
      overview: nullableString("A harbor freezes and a town changes with it."),
      tagline: nullableString("The ice remembers."),
      language: nullableString("en"),
      origin_countries: nullableString("US"),
      first_air_date: nullableString("2024-03-01"),
      last_air_date: nullableString("2026-05-20"),
      status: nullableString("Returning Series"),
      type: nullableString("Scripted"),
      poster_path: nullableString("/frost-harbor.jpg"),
      backdrop_path: nullableString("/frost-harbor-backdrop.jpg"),
      vote_average: nullableFloat64(8.4),
      vote_count: nullableInt64(1200),
      certification: nullableString("TV-14"),
      tmdb_season_count: nullableInt64(2),
      tmdb_episode_count: nullableInt64(10),
    },
    seasons: SEASONS,
    cast: [
      {
        credit_id: "credit-lead-1",
        artist_id: 900,
        character: "Captain",
        cast_order: 0,
        episode_count: 10,
        artist_name: "Ada Contract",
        artist_profile: nullableString("/ada.jpg"),
      },
      {
        // Same artist, second role: this is why credit_id is the React key.
        credit_id: "credit-lead-2",
        artist_id: 900,
        character: "Captain's Double",
        cast_order: 1,
        episode_count: 2,
        artist_name: "Ada Contract",
        artist_profile: nullableString("/ada.jpg"),
      },
    ],
    crew: [
      {
        credit_id: "crew-1",
        artist_id: 901,
        department: "Directing",
        job: "Director",
        episode_count: 6,
        artist_name: "Bo Contract",
        artist_profile: nullableString(""),
      },
    ],
    creators: [{ id: 900, name: "Ada Contract", profile: nullableString("/ada.jpg") }],
    genres: [{ id: 1, tag: "Drama" }],
    networks: [
      { id: 77, name: "Contract Network", logo: nullableString("/network.png"), country: nullableString("US") },
    ],
    production_companies: [{ id: 88, name: "Contract Pictures" }],
    extra_videos: [
      {
        id: 1,
        title: "Frost Harbor Trailer",
        key: "abc123",
        type: "trailer",
        site: "youtube",
      },
    ],
    ...overrides,
  };
}

export function seasonEpisodes(
  seasonNumber: number,
): ShowSeasonEpisodesDataType {
  const season = SEASONS.find(s => s.season_number === seasonNumber);
  const count = season?.available_episode_count ?? 0;

  return {
    season: season ?? seasonSummary(seasonNumber, 0),
    episodes: Array.from({ length: count }, (_, index) =>
      episode(seasonNumber, index + 1),
    ),
  };
}
