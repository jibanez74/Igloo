import {
  HeadContent,
  Outlet,
  createRootRouteWithContext,
} from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import { APP_DESCRIPTION, APP_NAME } from "@/lib/constants";

type RouterContextType = {
  queryClient: QueryClient;
};

// Defaults every route inherits. HeadContent keeps only the deepest title and
// one meta per name, so a child route's head overrides these, and a route that
// sets nothing (or is still loading or failed) shows them. The robots noindex
// is static in index.html, where crawlers that never run JS still see it.
const ROOT_HEAD = {
  meta: [
    { title: APP_NAME },
    { name: "description", content: APP_DESCRIPTION },
  ],
};

export const Route = createRootRouteWithContext<RouterContextType>()({
  head: () => ROOT_HEAD,
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="min-h-svh bg-card text-foreground antialiased">
      <HeadContent />
      <Outlet />
    </div>
  );
}
