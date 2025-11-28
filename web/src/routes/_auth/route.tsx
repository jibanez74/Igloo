import { redirect, Outlet, createFileRoute } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";

export const Route = createFileRoute("/_auth")({
  beforeLoad: async ({ context, location }) => {
    const res = await context.queryClient.ensureQueryData(authUserQueryOpts());

    if (res.error) {
      throw redirect({
        to: "/login",
        from: location.href,
      });
    }
  },
  component: () => <Outlet />,
});
