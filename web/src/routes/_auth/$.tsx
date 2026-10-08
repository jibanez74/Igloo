import { createFileRoute } from "@tanstack/react-router";
import MediaNotFound from "@/components/shared/MediaNotFound";
import { routeHead } from "@/lib/route-head";

const NOT_FOUND_HEAD = routeHead("Page not found");

// Catches every path no other authenticated route claims, so an unknown URL
// renders inside the shell with a way back instead of the router's bare text.
export const Route = createFileRoute("/_auth/$")({
  head: () => NOT_FOUND_HEAD,
  component: NotFoundPage,
});

function NotFoundPage() {
  return (
    <div className="mx-auto max-w-2xl">
      <h1 className="mb-6 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
        Page not found
      </h1>
      <MediaNotFound
        title="Not found"
        message="There is no page at this address."
        back="home"
      />
    </div>
  );
}
