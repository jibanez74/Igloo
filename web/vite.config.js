import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  ServiceWorker: {
    port: 3000,
  },
  plugins: [react()],
});
