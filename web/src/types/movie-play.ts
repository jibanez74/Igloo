import { z } from "zod";
import { STREAM_MODE_IDS } from "@/lib/constants";

export const playSearchSchema = z.object({
  mode: z.enum(STREAM_MODE_IDS).optional().catch(undefined),
  audio_track: z.coerce.number().int().min(0).catch(0).default(0),
  subtitle_track: z.coerce.number().int().min(0).optional().catch(undefined),
  start: z.coerce.number().min(0).catch(0).default(0),
});

export type PlaySearchParams = z.infer<typeof playSearchSchema>;
