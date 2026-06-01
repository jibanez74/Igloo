import { z } from "zod/mini";
import { STREAM_MODE_IDS } from "@/lib/constants";

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
