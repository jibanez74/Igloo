import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/movies/$movieID/play")({
  component: RouteComponent,
});

function RouteComponent() {
  return <div>Hello "/movies/$movieID/play"!</div>;
}
