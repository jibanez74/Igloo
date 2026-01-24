import "./assets/styles.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import { AudioPlayerProvider } from "./context/AudioPlayerContext";
import App from "./App";

const queryClient = new QueryClient();

const rootElement = document.getElementById("app");
if (rootElement && !rootElement.innerHTML) {
  const root = createRoot(rootElement);
  root.render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <AudioPlayerProvider>
          <App queryClient={queryClient} />
          <Toaster
            // Accessibility: Close button provides keyboard/screen reader access
            closeButton
            // Position toasts at top-right for visibility
            position="top-right"
            // Use rich colors for success/error differentiation
            richColors
            // Custom styling to match the app's dark theme
            toastOptions={{
              // Custom class names for dark theme styling
              classNames: {
                toast:
                  "bg-slate-800 border-slate-700 text-slate-100 shadow-xl",
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
    </StrictMode>
  );
}
