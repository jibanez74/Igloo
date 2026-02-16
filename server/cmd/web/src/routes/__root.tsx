import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import type { RouterContextType } from "@/types";

export const Route = createRootRouteWithContext<RouterContextType>()({
  component: () => <Outlet />,
});
