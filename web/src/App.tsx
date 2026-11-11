import { RouterProvider, createRouter } from "@tanstack/react-router";
import { QueryClient } from "@tanstack/react-query";
import { routeTree } from "./routeTree.gen";
import RoutePending from "./components/app/RoutePending";

const router = createRouter({
  routeTree,
  context: {
    queryClient: undefined!,
  },
  defaultPreload: "intent",
  scrollRestoration: true,
  defaultStructuralSharing: true,
  defaultPreloadStaleTime: 0,
  defaultPendingComponent: RoutePending,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }

  interface StaticDataRouteOption {
    /**
     * The route fills the window by itself: the shell draws no sidebar, no
     * header and no page padding around it. For the players, which bring
     * their own title and way back.
     */
    immersive?: boolean;
  }
}

type AppProps = {
  queryClient: QueryClient;
};

export default function App({ queryClient }: AppProps) {
  return <RouterProvider router={router} context={{ queryClient }} />;
}
