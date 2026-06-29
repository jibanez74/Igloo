import { useEffect } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
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
        <Toaster
          closeButton
          position="top-right"
          richColors
          toastOptions={{
            classNames: {
              toast: "bg-muted border-border text-foreground shadow-xl",
              title: "text-foreground font-medium",
              description: "text-muted-foreground",
              closeButton:
                "bg-accent border-border text-muted-foreground hover:bg-muted hover:text-foreground",
              success:
                "bg-emerald-900/90 border-emerald-700/50 text-emerald-100",
              error: "bg-red-900/90 border-red-700/50 text-red-100",
              info: "bg-muted border-primary/30 text-foreground",
            },
          }}
        />
      </AudioPlayerProvider>
    </QueryClientProvider>
  );
}
