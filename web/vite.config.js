import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    port: 3000,
    proxy: {
      "/api/v1": {
        target: "http://localhost:8080/api/v1",
      },
    },
  },
  plugins: [react()],
});
