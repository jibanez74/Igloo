import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";
import { authState } from "../stores/authStore";

export const Route = createFileRoute("/_auth")({
  beforeLoad: ({ location }) => {
    if (!authState.isAuthenticated) {
      console.log("user is not authenticated");
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
