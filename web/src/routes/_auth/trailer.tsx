import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod/mini";
import { movieDetailsQueryOpts } from "@/lib/query-opts";

const trailerSearchSchema = z.object({
  mediaType: z.optional(z.enum(["movie", "tv"])),
  mediaId: z.optional(z.coerce.number().check(z.int(), z.positive())),
  videoKey: z.optional(z.string()),
  returnTo: z.optional(z.string()),
});

export const Route = createFileRoute("/_auth/trailer")({
  validateSearch: trailerSearchSchema,
  loaderDeps: ({ search }) => ({
    mediaType: search.mediaType,
    mediaId: search.mediaId,
    videoKey: search.videoKey,
  }),
  loader: async ({ context, deps }) => {
    if (deps.mediaId && deps.mediaId > 0 && !deps.videoKey) {
      await context.queryClient.ensureQueryData(
        movieDetailsQueryOpts(deps.mediaId),
      );
    }
  },
});
