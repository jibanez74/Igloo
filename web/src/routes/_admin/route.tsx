import { redirect, Outlet, createFileRoute } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";
import AppShell from "@/components/AppShell";

export const Route = createFileRoute("/_admin")({
  beforeLoad: async ({ context, location }) => {
    const res = await context.queryClient.ensureQueryData(authUserQueryOpts());

    if (res.error) {
      throw redirect({
        to: "/login",
        search: { redirect: location.href },
      });
    }

    if (!res.data.user.is_admin) {
      throw redirect({ to: "/" });
    }
  },
  component: AdminLayout,
});

function AdminLayout() {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
