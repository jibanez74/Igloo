import type { components } from "./openapi.gen";

type Schema = components["schemas"];

// Show from our library (scanned) - used for Recently Added Shows on home
// (API returns poster_path; frontend builds the URL).
export type LatestShowType = Schema["LatestShow"];

// `data` payload of GET /api/shows/latest.
export type LatestShowsDataType = Schema["LatestShowsData"];

// `data` payload of GET /api/shows/details/{id}. Aliased from the named *Data
// schema, never from the envelope: an envelope inherits JsonSuccess.data's open
// index signature, which silently disables property checking.
export type ShowDetailsDataType = Schema["ShowDetailsData"];

// `data` payload of GET /api/shows/{id}/seasons/{seasonNumber}/episodes.
export type ShowSeasonEpisodesDataType = Schema["ShowSeasonEpisodesData"];

export type ShowType = Schema["Show"];
export type ShowSeasonSummaryType = Schema["ShowSeasonSummary"];
export type ShowEpisodeType = Schema["ShowEpisode"];
export type ShowCastCreditType = Schema["ShowCastCredit"];
export type ShowCrewCreditType = Schema["ShowCrewCredit"];
export type ShowPersonType = Schema["ShowPerson"];
export type ShowGenreType = Schema["ShowGenre"];
export type ShowNetworkType = Schema["ShowNetwork"];
export type ShowProductionCompanyType = Schema["ShowProductionCompany"];
export type ExtraVideoType = Schema["ExtraVideo"];
