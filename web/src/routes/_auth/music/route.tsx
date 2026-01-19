import { createFileRoute } from "@tanstack/react-router";
import { fallback, zodSearchValidator } from "@tanstack/router-zod-adapter";
import { z } from "zod";

const musicSearchSchema = z.object({
  tab: fallback(z.enum(["musicians", "albums", "tracks"]), "albums").default(
    "albums"
  ),
  albumsPage: fallback(z.number().int().positive(), 1).default(1),
  musiciansPage: fallback(z.number().int().positive(), 1).default(1),
});

export type MusicSearchParams = z.infer<typeof musicSearchSchema>;

export const Route = createFileRoute("/_auth/music")({
  validateSearch: zodSearchValidator(musicSearchSchema),
});
