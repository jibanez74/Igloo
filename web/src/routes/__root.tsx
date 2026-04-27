import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import type { RouterContextType } from "@/types";

export const Route = createRootRouteWithContext<RouterContextType>()({
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="min-h-svh bg-slate-900 text-slate-100 antialiased">
      <Outlet />
    </div>
  );
}
