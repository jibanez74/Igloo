import { redirect, Outlet, createFileRoute } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";
import AppShell from "@/components/AppShell";

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
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
