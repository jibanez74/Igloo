import { useEffect } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import { SPLASH_REMOVE_DELAY_MS } from "@/lib/app-boot";
import App from "./App";
import { AudioPlayerProvider } from "./context/AudioPlayerContext";

type AppBootProps = {
  queryClient: QueryClient;
};

export default function AppBoot({ queryClient }: AppBootProps) {
  useEffect(() => {
    const root = document.documentElement;
    const splash = document.getElementById("initial-splash");

    root.classList.add("dark");
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
              toast: "bg-slate-800 border-slate-700 text-slate-100 shadow-xl",
              title: "text-slate-100 font-medium",
              description: "text-slate-400",
              closeButton:
                "bg-slate-700 border-slate-600 text-slate-400 hover:bg-slate-600 hover:text-slate-100",
              success:
                "bg-emerald-900/90 border-emerald-700/50 text-emerald-100",
              error: "bg-red-900/90 border-red-700/50 text-red-100",
              info: "bg-slate-800 border-amber-500/30 text-slate-100",
            },
          }}
        />
      </AudioPlayerProvider>
    </QueryClientProvider>
  );
}
