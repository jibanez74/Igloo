import "./assets/css/styles.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import queryClient from "./lib/queryClient";
import { RouterProvider } from "react-router-dom";
import routes from "./routing/routes";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={routes} />
    </QueryClientProvider>
  </StrictMode>
);
