import "./assets/styles.css";
import { render } from "solid-js/web";
import { RouterProvider, createRouter } from "@tanstack/solid-router";
import { QueryClientProvider, QueryClient } from "@tanstack/solid-query";
import { routeTree } from "./routeTree.gen";
import NotFound from "./components/NotFound";

const queryClient = new QueryClient();

const router = createRouter({
  routeTree,
  scrollRestoration: true,
  defaultPreload: "intent",
  // defaultPreloadStaleTime: 0,
  defaultNotFoundComponent: NotFound,
  context: { queryClient },
});

declare module "@tanstack/solid-router" {
  interface Register {
    router: typeof router;
  }
}

const rootElement = document.getElementById("root")!;

if (!rootElement.innerHTML) {
  render(
    () => (
      <QueryClientProvider client={queryClient}>
        {" "}
        <RouterProvider router={router} />{" "}
      </QueryClientProvider>
    ),
    rootElement
  );
}
