import { fallback } from "@tanstack/router-zod-adapter";
import { z } from "zod";
import { STREAM_MODE_IDS } from "@/lib/constants";

export const playSearchSchema = z.object({
  mode: fallback(z.enum(STREAM_MODE_IDS), "direct").default("direct"),
  audio_track: fallback(z.coerce.number().int().min(0), 0).default(0),
  subtitle_track: fallback(z.coerce.number().int().min(0).optional(), undefined),
});

export type PlaySearchParams = z.infer<typeof playSearchSchema>;
