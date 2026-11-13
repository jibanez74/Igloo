import type { ReactNode } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import { useForgetEndedSession } from "@/hooks/useEndSession";
import { createTestQueryClient } from "./render";

function LoginPage() {
  useForgetEndedSession();
  return <p>Signed out</p>;
}

/**
 * Renders `page` behind a two-route router (the page at `/`, a login page at
 * `/login`) so a test can cross a session boundary the way the app does:
 * `endSession()` arrives at the login page logged out, which ends the session
 * for mutations in flight and clears the caches; `signIn()` returns to the page
 * as the next reader. Avoids the generated route tree, so suites that mock
 * `@/lib/api` can use it.
 */
export async function renderAcrossSessions(page: () => ReactNode) {
  const queryClient = createTestQueryClient();
  const rootRoute = createRootRoute({ component: Outlet });
  const routeTree = rootRoute.addChildren([
    createRoute({ getParentRoute: () => rootRoute, path: "/", component: page }),
    createRoute({ getParentRoute: () => rootRoute, path: "/login", component: LoginPage }),
  ]);
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });

  await act(async () => {
    await router.load();
  });

  const view = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );

  const endSession = async () => {
    await act(async () => {
      await router.navigate({
        to: "/login",
        replace: true,
        state: (previous) => ({ ...previous, loggedOut: true }),
      });
    });
    await screen.findByText("Signed out");
  };

  const signIn = async () => {
    await act(async () => {
      await router.navigate({ to: "/" });
    });
  };

  return { ...view, queryClient, router, endSession, signIn };
}

/**
 * Resolves once a mutation's callbacks have all run. TanStack dispatches a
 * mutation's success or error only after awaiting its onSuccess / onError /
 * onSettled, and still notifies the mutation cache for a mutation a clear
 * removed, so this is the moment a negative assertion about those callbacks
 * becomes meaningful.
 */
export function mutationSettled(queryClient: ReturnType<typeof createTestQueryClient>) {
  return new Promise<void>((resolve) => {
    const unsubscribe = queryClient.getMutationCache().subscribe((event) => {
      if (
        event.type === "updated" &&
        (event.action.type === "success" || event.action.type === "error")
      ) {
        unsubscribe();
        resolve();
      }
    });
  });
}
