import type { PropsWithChildren, ReactElement } from "react";
import type { QueryClient } from "@tanstack/react-query";
import { QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { createQueryClient } from "@/lib/query-client";

/**
 * The app's QueryClient, failing fast so tests never wait on retry backoff.
 */
export function createTestQueryClient() {
  return createQueryClient({
    mutations: {
      retry: false,
    },
    queries: {
      retry: false,
    },
  });
}

/** Provides `queryClient` to a tree: a render or renderHook `wrapper`. */
export function queryClientWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  };
}

/**
 * Renders `ui` under a QueryClientProvider. Pass `queryClient` to seed or spy
 * on the cache before rendering; otherwise a fresh test client is made.
 */
export function renderWithQueryClient(
  ui: ReactElement,
  { queryClient = createTestQueryClient() }: { queryClient?: QueryClient } = {},
) {
  return {
    queryClient,
    ...render(ui, { wrapper: queryClientWrapper(queryClient) }),
  };
}
