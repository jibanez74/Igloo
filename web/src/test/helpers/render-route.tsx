// Kept separate from helpers/render.tsx on purpose: importing the generated
// route tree evaluates every route module, which breaks suites that mock
// @tanstack/react-router. Only full-router tests should pull this in.

import type { ReactElement, ReactNode } from "react";
import type { QueryClient } from "@tanstack/react-query";
import { QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { act, render } from "@testing-library/react";
import { routeTree } from "@/routeTree.gen";
import { createTestQueryClient } from "./render";

/**
 * Boots the real route tree at `initialEntry` and renders it. Loaders run
 * before render (via `router.load()`), so callers must stub `fetch` first.
 *
 * `wrapper` nests extra providers between the QueryClientProvider and the
 * router, for routes that read from a context the app shell normally supplies.
 */
export async function renderRoute(
  initialEntry: string,
  options?: {
    queryClient?: QueryClient;
    wrapper?: (children: ReactNode) => ReactElement;
  },
) {
  const queryClient = options?.queryClient ?? createTestQueryClient();
  const history = createMemoryHistory({
    initialEntries: [initialEntry],
  });
  const router = createRouter({
    routeTree,
    context: {
      queryClient,
    },
    history,
  });

  await act(async () => {
    await router.load();
  });

  const routerElement = (
    <RouterProvider router={router} context={{ queryClient }} />
  );

  const view = render(
    <QueryClientProvider client={queryClient}>
      {options?.wrapper ? options.wrapper(routerElement) : routerElement}
    </QueryClientProvider>,
  );

  return {
    queryClient,
    router,
    ...view,
  };
}
