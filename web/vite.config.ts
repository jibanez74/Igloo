import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [
    react({
      babel: {
        plugins: [["babel-plugin-react-compiler"]],
      },
    }),
  ],

  // css: {
  //   preprocessorOptions: {
  //     scss: {},
  //   },
  // },

  server: {
    port: 3000,
    open: true,
    proxy: {
      "/api/v1": {
        target: "https://swifty.hare-crocodile.ts.net/api/v1",
        // target: "http://localhost:8080/api/v1",
        changeOrigin: true,
        rewrite: path => path.replace(/^\/api\/v1/, ""), // Remove '/api/v1' from the request path
      },
    },
  },
});
