/** Why the router is running a loader: see TanStack Router's `cause`. */
type LoaderCause = "enter" | "stay" | "preload";

/**
 * Awaits a loader's queries when the route is entered or preloaded, so a page
 * arrives complete. When the route stays matched and only its loader deps
 * changed (a tab, page, sort or season switch), it starts them without
 * waiting: the URL change commits at once and the page's own skeleton covers
 * the load, instead of the old view lingering until the router's pending view
 * replaces the page (design-system §3.4).
 */
export async function loadOnEntry(
  cause: LoaderCause,
  loads: Promise<unknown>[],
) {
  if (cause !== "stay") {
    await Promise.all(loads);
    return;
  }

  for (const load of loads) {
    void settle(load);
  }
}

// Nothing waits for an in-page load, so its failure is left to the page's own
// query, which renders the error state.
async function settle(load: Promise<unknown>) {
  try {
    await load;
  } catch {
    // Reported by the page's query.
  }
}
