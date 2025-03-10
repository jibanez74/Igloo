import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import Navbar from "../components/Navbar";
import type { AuthContextType } from "../types/Auth";

type RouteContext = {
  auth: AuthContextType;
};

export const Route = createRootRouteWithContext<RouteContext>()({
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
