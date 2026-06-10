import { createFileRoute } from "@tanstack/react-router";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import { trailerSearchSchema } from "@/types/route-search";

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
