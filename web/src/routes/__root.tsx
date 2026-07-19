import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import BootstrapHeadMetadataSync from "@/components/app/BootstrapHeadMetadataSync";

type RouterContextType = {
  queryClient: QueryClient;
};

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
