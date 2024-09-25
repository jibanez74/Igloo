import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  server: {
    port: 3000,
    open: true,
    proxy: {
      "/api/v1": {
        target: "https://swifty.hare-crocodile.ts.net/api/v1",
        changeOrigin: true,
        rewrite: path => path.replace(/^\/api\/v1/, ""), // Remove '/api/v1' from the request path
      },
    },
  },
});
