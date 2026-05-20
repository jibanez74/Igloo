import { createFileRoute } from "@tanstack/react-router";
import { watchRoomQueryOpts } from "@/lib/query-opts";

export const Route = createFileRoute("/_auth/watch-rooms/$id")({
  params: {
    parse: params => ({
      id: parseWatchRoomId(params.id),
    }),
    stringify: params => ({
      id: String(params.id),
    }),
  },
  loader: async ({ context, params: { id } }) => {
    if (id === null) return;
    await context.queryClient.ensureQueryData(watchRoomQueryOpts(id));
  },
});

function parseWatchRoomId(value: string) {
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < 1) {
    return null;
  }

  return parsed;
}
