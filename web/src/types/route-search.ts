import { z } from "zod/mini";
import { STREAM_MODE_IDS } from "@/lib/constants";
import { getSafeRedirect } from "@/lib/redirect-utils";

export const loginSearchSchema = z.object({
  redirect: z.pipe(
    z._default(z.catch(z.string(), "/"), "/"),
    z.transform((url: string) => getSafeRedirect(url, "/")),
  ),
});

export type LoginSearchParams = z.infer<typeof loginSearchSchema>;

export const trailerSearchSchema = z.object({
  mediaType: z.optional(z.enum(["movie", "tv"])),
  mediaId: z.optional(z.coerce.number().check(z.int(), z.positive())),
  videoKey: z.optional(z.string()),
  returnTo: z.optional(z.string()),
});

export type TrailerSearchParams = z.infer<typeof trailerSearchSchema>;

export const searchSearchSchema = z.object({
  q: z._default(z.catch(z.string(), ""), ""),
  tab: z._default(
    z.catch(z.enum(["all", "movies", "albums", "musicians", "tracks"]), "all"),
    "all",
  ),
  page: z._default(
    z.catch(z.number().check(z.int(), z.positive()), 1),
    1,
  ),
});

export type SearchParams = z.infer<typeof searchSearchSchema>;

export const moviesSearchSchema = z.object({
  tab: z._default(
    z.catch(z.enum(["all", "genres", "playlists"]), "all"),
    "all",
  ),
  allPage: z._default(
    z.catch(z.number().check(z.int(), z.positive()), 1),
    1,
  ),
  sort: z._default(z.catch(z.enum(["asc", "desc"]), "asc"), "asc"),
  genresPage: z._default(
    z.catch(z.number().check(z.int(), z.positive()), 1),
    1,
  ),
  genreId: z.catch(
    z.optional(z.number().check(z.int(), z.positive())),
    undefined,
  ),
  playlistsPage: z._default(
    z.catch(z.number().check(z.int(), z.positive()), 1),
    1,
  ),
  view: z.catch(z.optional(z.enum(["liked"])), undefined),
});

export type MoviesSearchParams = z.infer<typeof moviesSearchSchema>;

export const musicSearchSchema = z.object({
  tab: z._default(
    z.catch(z.enum(["musicians", "albums", "tracks", "playlists"]), "albums"),
    "albums",
  ),
  albumsPage: z._default(
    z.catch(z.number().check(z.int(), z.positive()), 1),
    1,
  ),
  musiciansPage: z._default(
    z.catch(z.number().check(z.int(), z.positive()), 1),
    1,
  ),
  playlistsView: z._default(
    z.catch(z.enum(["playlists", "liked"]), "playlists"),
    "playlists",
  ),
  likedTracksPage: z._default(
    z.catch(z.number().check(z.int(), z.positive()), 1),
    1,
  ),
});

export type MusicSearchParams = z.infer<typeof musicSearchSchema>;

export const playSearchSchema = z.object({
  mode: z.catch(z.optional(z.enum(STREAM_MODE_IDS)), undefined),
  audio_track: z._default(
    z.catch(z.coerce.number().check(z.int(), z.minimum(0)), 0),
    0,
  ),
  subtitle_track: z.catch(
    z.optional(z.coerce.number().check(z.int(), z.minimum(0))),
    undefined,
  ),
  start: z._default(
    z.catch(z.coerce.number().check(z.minimum(0)), 0),
    0,
  ),
});

export type PlaySearchParams = z.infer<typeof playSearchSchema>;
