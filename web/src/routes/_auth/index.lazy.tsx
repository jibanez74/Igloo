import { createLazyFileRoute } from "@tanstack/react-router";

export const Route = createLazyFileRoute("/_auth/")({
  component: HomePage,
});

function HomePage() {
  return (
    <div>
      <h1>Hello Home Page</h1>
    </div>
  );
}
