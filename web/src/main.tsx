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
          <Toaster />
        </AudioPlayerProvider>
      </QueryClientProvider>
    </StrictMode>
  );
}
