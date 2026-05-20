import { createLazyFileRoute } from "@tanstack/react-router";
import {
  WatchRoomPage,
  WatchRoomUnavailable,
} from "@/components/watch-room/WatchRoomPage";

export const Route = createLazyFileRoute("/_auth/watch-rooms/$id")({
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
