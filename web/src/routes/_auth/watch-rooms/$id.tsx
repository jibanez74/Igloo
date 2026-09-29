import { createFileRoute } from "@tanstack/react-router";
import { watchRoomQueryOpts } from "@/lib/query-opts";
import { parseRouteId } from "@/lib/route-id";
import { routeHead } from "@/lib/route-head";
import {
  WatchRoomPage,
  WatchRoomUnavailable,
} from "@/components/watch-room/WatchRoomPage";

const WATCH_ROOM_FALLBACK_HEAD = routeHead("Watch Room");

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
    if (id === null) return { movieTitle: null };

    const res = await context.queryClient.ensureQueryData(
      watchRoomQueryOpts(id),
    );

    return {
      movieTitle: res.error ? null : res.data.room?.movie_title || null,
    };
  },
  head: ({ loaderData }) => {
    const movieTitle = loaderData?.movieTitle;

    return movieTitle
      ? routeHead(
          `${movieTitle} Watch Room`,
          `Watch ${movieTitle} together in a shared synchronized room.`,
        )
      : WATCH_ROOM_FALLBACK_HEAD;
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
