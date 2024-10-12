import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";

const searchSchema = z.object({
  directPlay: z.boolean().optional(),
  videoCodec: z.string().optional(),
  videoBitRate: z.string().optional(),
  audioCodec: z.string().optional(),
  audioBitRate: z.string().optional(),
  audioChannels: z.string().optional(),
  container: z.string().optional(),
});

export const Route = createFileRoute("/movies/play")({
  component: () => <div>Hello /movies/play!</div>,
  validateSearch: searchSchema.parse,
});
