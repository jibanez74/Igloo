import { useQuery } from "@tanstack/react-query";
import { useMovieScanStatus, useMusicScanStatus, useShowScanStatus } from "@/hooks/useScanStatus";
import { redirect, Outlet, createFileRoute, useLocation } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";
import AppShell from "@/components/app/AppShell";

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
