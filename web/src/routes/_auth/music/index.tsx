import { createFileRoute } from "@tanstack/solid-router";

export const Route = createFileRoute("/_auth/music/")({
  component: RouteComponent,
});

function RouteComponent() {
  return <div>Hello "/music/"!</div>;
}
