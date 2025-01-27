import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/music/")({
  component: RouteComponent,
});

function RouteComponent() {
  return <div>Hello "/music/"!</div>;
}
