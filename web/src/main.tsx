    import "./assets/styles.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createQueryClient } from "@/lib/query-client";
import AppBoot from "./AppBoot";

const queryClient = createQueryClient();

const rootElement = document.getElementById("app");
if (rootElement && !rootElement.innerHTML) {
  const root = createRoot(rootElement);
  root.render(
    <StrictMode>
      <AppBoot queryClient={queryClient} />
    </StrictMode>
  );
}
