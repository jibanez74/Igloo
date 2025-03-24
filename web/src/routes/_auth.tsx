import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";
import { authState } from "../stores/authStore";

export const Route = createFileRoute("/_auth")({
  beforeLoad: ({ location }) => {
    if (!authState.isAuthenticated) {
      // throw redirect({
      //   to: "/login",
      //   search: {
      //     redirect: location.href,
      //   },
      // });
    }
  },
  component: () => <Outlet />,
});
