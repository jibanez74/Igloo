import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";
import { movieDetailsQueryOpts } from "@/lib/query-opts";

const trailerSearchSchema = z.object({
  mediaType: z.enum(["movie", "tv"]).optional(),
  mediaId: z.coerce.number().int().positive().optional(),
  videoKey: z.string().optional(),
  returnTo: z.string().optional(),
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
