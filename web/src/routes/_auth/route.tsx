import { useQuery } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";
import { useScanStatus } from "@/hooks/useScanStatus";
import { redirect, Outlet, createFileRoute, useLocation } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";
import { STALE_1M } from "@/lib/query-client";
import type { ApiResponseType, AuthUser } from "@/types";
import { ApiFailureError } from "@/lib/is-api-failure";
import { useSignOutOnSessionExpiry } from "@/hooks/useEndSession";
import { isSessionEnding } from "@/lib/session";
import AppShell from "@/components/app/AppShell";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import AppLoadingScreen, { AppLoadError } from "@/components/app/AppLoadingScreen";

export const Route = createFileRoute("/_auth")({
  beforeLoad: async ({ context, location }) => {
    const res = await signedInUser(context.queryClient);

    if (!res.error) return;

    if (res.status === 401) {
      // The session is gone: like a logout, the login page skips asking again
      // and forgets what this session cached (useForgetEndedSession).
      throw redirect({
        to: "/login",
        search: { redirect: location.href },
        state: (previous) => ({ ...previous, loggedOut: true }),
      });
    }

    // No answer about the session at all (offline, the server down, a proxy
    // error): a signed-in reader must not be sent to log in for it.
    throw new ApiFailureError(res);
  },
  // Boot waits here, before the shell exists, so this one route keeps the
  // full-screen splash; the routes inside the shell pend in its content area.
  pendingComponent: AppLoadingScreen,
  errorComponent: AppLoadError,
  // Only the layout is split off. The error card must render when the server
  // cannot be reached, so it cannot wait on a chunk from that server: a
  // failed chunk makes the router reload the page onto the browser's offline
  // screen.
  codeSplitGroupings: [["component"]],
  component: AuthLayout,
});

/**
 * The user this navigation runs as. A signed-in user checked within the last
 * minute (STALE_1M) is answered from the cache, so a navigation or hover
 * preload does not wait on the session before its own loaders start. Past
 * that, or with no signed-in entry (boot, a failure, an invalidated entry),
 * the guard waits for the server: with the server down that check is what
 * fails first and shows AppLoadError, before a route chunk that cannot load
 * makes the router reload onto the browser's offline page. Within the minute
 * that can still happen: a server lost then, and a section whose code has not
 * loaded yet, reload onto the offline page. A session that ends within the
 * minute is caught by the first request that hears its 401
 * (onSessionExpired). A session that is ending always asks: if the way to
 * /login is superseded, the guard must not keep the shell on the cached user
 * while the expiry handler ignores 401s.
 */
async function signedInUser(queryClient: QueryClient) {
  const options = authUserQueryOpts();
  const query = queryClient
    .getQueryCache()
    .find<ApiResponseType<{ user: AuthUser }>>({ queryKey: options.queryKey, exact: true });
  const cached = query?.state.data;
  if (cached?.error === false && !query?.isStaleByTime(STALE_1M) && !isSessionEnding()) {
    return cached;
  }
  return queryClient.fetchQuery(authUserQueryOpts({ revalidate: true }));
}

// The music player lives with the shell, not above the router: the login
// page never loads it, and signing out (which leaves the shell) takes it down
// with the session. It wraps everything here so ending a session from the
// shell (useSignOutOnSessionExpiry) can stop it first.
function AuthLayout() {
  return (
    <AudioPlayerProvider>
      <AuthShell />
    </AudioPlayerProvider>
  );
}

function AuthShell() {
  useSignOutOnSessionExpiry();
  const { data } = useQuery(authUserQueryOpts());
  const librariesVisible = useLocation({ select: location => location.pathname.replace(/\/+$/, "") === "/settings/libraries" });
  useScanStatus({
    enabled: data?.error === false && data.data.user.is_admin,
    background: !librariesVisible,
  });
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
