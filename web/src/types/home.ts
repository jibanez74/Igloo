import type { components } from "./openapi.gen";

type Schema = components["schemas"];

// `data` payload of GET /api/continue-watching. The row mixes movies and
// episodes, so it belongs to neither domain file. Aliased from the named *Data
// schema, never from the envelope: an envelope inherits JsonSuccess.data's open
// index signature, which silently disables property checking.
export type ContinueWatchingDataType = Schema["ContinueWatchingData"];

// One card in the row, discriminated by `kind`.
export type ContinueWatchingItemType = Schema["ContinueWatchingItem"];
export type ContinueWatchingMovieItemType = Schema["ContinueWatchingMovieItem"];
export type ContinueWatchingEpisodeItemType =
  Schema["ContinueWatchingEpisodeItem"];
