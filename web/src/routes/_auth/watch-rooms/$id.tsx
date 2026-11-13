import type { ReactNode } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { watchRoomQueryOpts } from "@/lib/query-opts";
import { parseRouteId } from "@/lib/route-id";
import { routeHead } from "@/lib/route-head";
import { WatchRoomPage } from "@/components/watch-room/WatchRoomPage";
import {
  WatchRoomLoading,
  WatchRoomUnavailable,
} from "@/components/watch-room/WatchRoomStatus";

const WATCH_ROOM_FALLBACK_HEAD = routeHead("Watch Room");

export const Route = createFileRoute("/_auth/watch-rooms/$id")({
  // The room has the window to itself: no sidebar and no header. It is a
  // page of cards, though, not one player surface, so RoomFrame below gives
  // it back the padding the shell would have.
  staticData: { immersive: true },
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
  pendingComponent: () => (
    <RoomFrame>
      <WatchRoomLoading />
    </RoomFrame>
  ),
  component: WatchRoomRoute,
});

function RoomFrame({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-1 flex-col px-4 py-6 sm:px-6 lg:px-8">
      {children}
    </div>
  );
}

function WatchRoomRoute() {
  const { id } = Route.useParams();
  const navigate = Route.useNavigate();

  if (id === null) {
    return (
      <RoomFrame>
        <WatchRoomUnavailable
          message="This watch room link is invalid."
          onBackHome={() => navigate({ to: "/" })}
        />
      </RoomFrame>
    );
  }

  return (
    <RoomFrame>
      <WatchRoomPage roomId={id} />
    </RoomFrame>
  );
}
