import type { QueryClient } from "@tanstack/react-query";
import { isApiFailure } from "@/lib/is-api-failure";
import { showActionFailed, showSuccess } from "@/lib/toast-helpers";

/** Capitalised, singular: it opens the toast copy and is lowercased mid-sentence. */
export type LibraryRefreshNoun = "Music" | "Movie" | "Show";

/**
 * Runs a library cache refresh with the shared success/failure toasts. Never
 * throws, so component handlers can reset their UI state with plain sequential
 * code instead of a `finally` block (which bails React Compiler out of the
 * whole component).
 */
export async function refreshLibraryWithToasts(
  queryClient: QueryClient,
  refresh: (queryClient: QueryClient) => Promise<void>,
  libraryNoun: LibraryRefreshNoun,
): Promise<void> {
  try {
    await refresh(queryClient);
    showSuccess(
      "Library refreshed",
      `${libraryNoun} library data is up to date.`,
    );
  } catch (error) {
    console.error(
      `Failed to refresh ${libraryNoun.toLowerCase()} library:`,
      error,
    );
    showActionFailed(
      "refresh library",
      `Unable to refresh the ${libraryNoun.toLowerCase()} library. Please try again.`,
    );
  }
}

/** Marks every query under each key stale; active ones refetch on their own. */
export function invalidateLibraryQueryKeys(
  queryClient: QueryClient,
  keys: readonly string[],
) {
  for (const key of keys) {
    void queryClient.invalidateQueries({ queryKey: [key] });
  }
}

/**
 * Drops inactive queries under each key and refetches the active ones,
 * rejecting if any refetch fails or lands on an API error envelope, so the
 * caller's toast reflects what the user will actually see.
 */
export async function refreshLibraryQueryKeys(
  queryClient: QueryClient,
  keys: readonly string[],
) {
  await Promise.all(
    keys.map(async key => {
      queryClient.removeQueries({ queryKey: [key], type: "inactive" });
      await queryClient.refetchQueries(
        { queryKey: [key], type: "active" },
        { throwOnError: true },
      );

      const refreshedQueries = queryClient.getQueriesData({
        queryKey: [key],
        type: "active",
      });

      for (const [, data] of refreshedQueries) {
        if (isApiFailure(data)) {
          throw new Error(data.message);
        }
      }
    }),
  );
}
