import {
  redirect,
  createRootRouteWithContext,
  Outlet,
} from "@tanstack/react-router";
import Navbar from "../components/Navbar";
import type { AuthContextType } from "../types/Auth";

type RouteContext = {
  auth: AuthContextType;
};

export const Route = createRootRouteWithContext<RouteContext>()({
  beforeLoad: ({ context, location }) => {
    if (location.href !== "/login" && !context.auth.isAuthenticated) {
      throw redirect({
        to: "/login",
        search: {
          redirect: location.href,
        },
      });
    }
  },
  component: RootComponent,
});

function RootComponent() {
  return (
    <>
      <header>
        <Navbar />
      </header>
      <Outlet />
    </>
  );
}
