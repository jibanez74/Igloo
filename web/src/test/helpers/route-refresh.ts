// A route's head reads its title through the loader from the query cache, so
// a dialog that renames what the title shows must let the details refetch land
// before it reloads the route. These record and check that order for dialog
// suites that mock `useRouter` (the mock itself stays in each suite, since
// `vi.mock` is hoisted per file).
import { expect, vi, type Mock } from "vitest";
import type { QueryClient } from "@tanstack/react-query";

/**
 * Records, in the order they finish, each `invalidateQueries` refetch on
 * `queryClient` and the next call to the mocked `router.invalidate`.
 */
export function recordRefreshOrder(
  queryClient: QueryClient,
  routerInvalidate: Mock<() => Promise<void>>,
) {
  const events: string[] = [];
  const invalidateQueries = queryClient.invalidateQueries.bind(queryClient);

  vi.spyOn(queryClient, "invalidateQueries").mockImplementation(
    async (filters, options) => {
      await invalidateQueries(filters, options);
      events.push(`refetched ${JSON.stringify(filters?.queryKey)}`);
    },
  );
  routerInvalidate.mockImplementationOnce(() => {
    events.push("router reload");
    return Promise.resolve();
  });

  return events;
}

/** Asserts `queryKey` was refetched before the route reload started. */
export function expectRefetchedBeforeReload(
  events: string[],
  queryKey: readonly unknown[],
) {
  const refetched = events.indexOf(`refetched ${JSON.stringify(queryKey)}`);

  expect(refetched).toBeGreaterThanOrEqual(0);
  expect(events.indexOf("router reload")).toBeGreaterThan(refetched);
}
