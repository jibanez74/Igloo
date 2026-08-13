import type { QueryClient } from "@tanstack/react-query";
import { showActionFailed, showSuccess } from "@/lib/toast-helpers";

/**
 * Runs a library cache refresh with the shared success/failure toasts. Never
 * throws, so component handlers can reset their UI state with plain sequential
 * code instead of a `finally` block (which bails React Compiler out of the
 * whole component).
 */
export async function refreshLibraryWithToasts(
  queryClient: QueryClient,
  refresh: (queryClient: QueryClient) => Promise<void>,
  libraryNoun: "Music" | "Movie",
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
