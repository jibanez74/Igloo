import { createFileRoute } from "@tanstack/react-router";
import { watchRoomQueryOpts } from "@/lib/query-opts";
import { parseRouteId } from "@/lib/route-id";
import {
  WatchRoomPage,
  WatchRoomUnavailable,
} from "@/components/watch-room/WatchRoomPage";

export const Route = createFileRoute("/_auth/watch-rooms/$id")({
  params: {
    parse: params => ({
      id: parseRouteId(params.id),
    }),
    stringify: params => ({
      id: String(params.id),
    }),
  },
  loader: async ({ context, params: { id } }) => {
    if (id === null) return;
    await context.queryClient.ensureQueryData(watchRoomQueryOpts(id));
  },
  component: WatchRoomRoute,
});

function WatchRoomRoute() {
  const { id } = Route.useParams();
  const navigate = Route.useNavigate();

  if (id === null) {
    return (
      <WatchRoomUnavailable
        message="This watch room link is invalid."
        onBackHome={() => navigate({ to: "/" })}
      />
    );
  }

  return <WatchRoomPage roomId={id} />;
}
