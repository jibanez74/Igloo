import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/_auth/music/album/$id")({
  component: RouteComponent,
});

function RouteComponent() {
  return <div>Hello "/_auth/music/album/$id"!</div>;
}
