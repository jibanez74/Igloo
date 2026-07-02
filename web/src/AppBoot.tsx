import { useEffect } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/sonner";
import { SPLASH_REMOVE_DELAY_MS } from "@/lib/app-boot";
import { applyTheme, getStoredTheme } from "@/lib/theme";
import App from "./App";
import { AudioPlayerProvider } from "./context/AudioPlayerContext";

type AppBootProps = {
  queryClient: QueryClient;
};

export default function AppBoot({ queryClient }: AppBootProps) {
  useEffect(() => {
    const root = document.documentElement;
    const splash = document.getElementById("initial-splash");

    applyTheme(getStoredTheme());
    root.setAttribute("data-app-ready", "true");

    if (!splash) {
      return;
    }

    const removeSplash = window.setTimeout(() => {
      splash.remove();
    }, SPLASH_REMOVE_DELAY_MS);

    return () => {
      window.clearTimeout(removeSplash);
    };
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
