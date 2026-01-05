import { RouterProvider, createRouter } from "@tanstack/react-router";
import { QueryClient } from "@tanstack/react-query";
import { routeTree } from "./routeTree.gen";
import RouterPending from "./components/RouterPending";
import { useAudioPlayer } from "./context/AudioPlayerContext";

const router = createRouter({
  routeTree,
  context: {
    queryClient: undefined!,
    audioPlayer: undefined!,
  },
  defaultPreload: "intent",
  scrollRestoration: true,
  defaultStructuralSharing: true,
  defaultPreloadStaleTime: 0,
  defaultPendingComponent: RouterPending,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

type AppProps = {
  queryClient: QueryClient;
};

export default function App({ queryClient }: AppProps) {
  const audioPlayer = useAudioPlayer();

  return (
    <RouterProvider
      router={router}
      context={{ queryClient, audioPlayer }}
    />
  );
}

