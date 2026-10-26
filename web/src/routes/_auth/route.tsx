import { useQuery } from "@tanstack/react-query";
import { useMovieScanStatus, useMusicScanStatus, useShowScanStatus } from "@/hooks/useScanStatus";
import { redirect, Outlet, createFileRoute, useLocation } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";
import { ApiFailureError } from "@/lib/is-api-failure";
import { useSignOutOnSessionExpiry } from "@/hooks/useEndSession";
import AppShell from "@/components/app/AppShell";
import AppLoadingScreen, { AppLoadError } from "@/components/app/AppLoadingScreen";

export const Route = createFileRoute("/_auth")({
  beforeLoad: async ({ context, location }) => {
    const res = await context.queryClient.fetchQuery(
      authUserQueryOpts({ revalidate: true }),
    );

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

function AuthLayout() {
  useSignOutOnSessionExpiry();
  const { data } = useQuery(authUserQueryOpts());
  const librariesVisible = useLocation({ select: location => location.pathname.replace(/\/+$/, "") === "/settings/libraries" });
  const scanOptions = {
    enabled: data?.error === false && data.data.user.is_admin,
    background: !librariesVisible,
  };
  useMovieScanStatus(scanOptions);
  useMusicScanStatus(scanOptions);
  useShowScanStatus(scanOptions);
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
