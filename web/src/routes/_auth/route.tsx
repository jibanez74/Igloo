import { useQuery } from "@tanstack/react-query";
import { useMovieScanStatus, useMusicScanStatus, useShowScanStatus } from "@/hooks/useScanStatus";
import { redirect, Outlet, createFileRoute, useLocation } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";
import AppShell from "@/components/app/AppShell";
import AppLoadingScreen from "@/components/app/AppLoadingScreen";

export const Route = createFileRoute("/_auth")({
  beforeLoad: async ({ context, location }) => {
    const res = await context.queryClient.fetchQuery(
      authUserQueryOpts({ revalidate: true }),
    );

    if (res.error) {
      throw redirect({
        to: "/login",
        search: { redirect: location.href },
      });
    }
  },
  // Boot waits here, before the shell exists, so this one route keeps the
  // full-screen splash; the routes inside the shell pend in its content area.
  pendingComponent: AppLoadingScreen,
  component: AuthLayout,
});

function AuthLayout() {
  const { data } = useQuery(authUserQueryOpts());
  const librariesVisible = useLocation({ select: location => location.pathname.replace(/\/+$/, "") === "/settings/libraries" });
  const scanOptions = {
    enabled: data?.error === false && data.data.user.is_admin,
    watchIdle: librariesVisible,
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
