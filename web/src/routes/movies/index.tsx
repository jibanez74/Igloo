import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/movies/")({
  component: () => <div>Hello /movies/!</div>,
});
