import { useEffect } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/sonner";
import { dismissBootSplash } from "@/lib/boot-splash";
import { applyTheme, getStoredTheme } from "@/lib/theme";
import App from "./App";
import { AudioPlayerProvider } from "./context/AudioPlayerContext";

type AppBootProps = {
  queryClient: QueryClient;
};

export default function AppBoot({ queryClient }: AppBootProps) {
  // No cleanup on purpose: the splash must go once React has painted, and a
  // StrictMode or HMR remount in the fade window must not cancel that.
  useEffect(() => {
    applyTheme(getStoredTheme());
    dismissBootSplash();
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <AudioPlayerProvider>
        <App queryClient={queryClient} />
        <Toaster />
      </AudioPlayerProvider>
    </QueryClientProvider>
  );
}
