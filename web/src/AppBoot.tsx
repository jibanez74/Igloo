import { useEffect } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { dismissBootSplash } from "@/lib/boot-splash";
import { applyTheme, getStoredTheme } from "@/lib/theme";
import App from "./App";

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
      <TooltipProvider>
        <App queryClient={queryClient} />
      </TooltipProvider>
      <Toaster />
    </QueryClientProvider>
  );
}
