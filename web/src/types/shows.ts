import type { components } from "./openapi.gen";

type Schema = components["schemas"];

// Show from our library (scanned) - used for Recently Added Shows on home
// (API returns poster_path; frontend builds the URL).
export type LatestShowType = Schema["LatestShow"];

// `data` payload of GET /api/shows/latest.
export type LatestShowsDataType = Schema["LatestShowsData"];
