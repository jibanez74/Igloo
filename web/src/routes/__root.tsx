import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import BootstrapHeadMetadataSync from "@/components/BootstrapHeadMetadataSync";
import type { RouterContextType } from "@/types";

export const Route = createRootRouteWithContext<RouterContextType>()({
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="min-h-svh bg-card text-foreground antialiased">
      <BootstrapHeadMetadataSync />
      <Outlet />
    </div>
  );
}
