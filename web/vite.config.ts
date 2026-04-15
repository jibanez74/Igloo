import { defineConfig } from "vitest/config";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  server: {
    port: 3000,
    open: true,

    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },

  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },

  build: {
    target: "esnext",
    modulePreload: {
      polyfill: false,
    },
    reportCompressedSize: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!/[\\/]node_modules[\\/]/.test(id)) {
            return;
          }

          if (/[\\/]node_modules[\\/](react|react-dom)[\\/]/.test(id)) {
            return "vendor-react";
          }
          if (/[\\/]node_modules[\\/]@tanstack[\\/](react-router|router)[\\/]/.test(id)) {
            return "vendor-router";
          }
          if (/[\\/]node_modules[\\/]@tanstack[\\/](react-query|query)[\\/]/.test(id)) {
            return "vendor-query";
          }
          if (/[\\/]node_modules[\\/](radix-ui|@radix-ui)[\\/]/.test(id)) {
            return "vendor-radix";
          }
          if (/[\\/]node_modules[\\/]lucide-react[\\/]/.test(id)) {
            return "vendor-lucide";
          }
        },
      },
    },
  },

  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    include: ["src/test/**/*.test.{ts,tsx}"],
  },

  plugins: [
    tailwindcss(),
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
      routesDirectory: "./src/routes",
      generatedRouteTree: "./src/routeTree.gen.ts",
    }),

    react({
      babel: {
        plugins: [["babel-plugin-react-compiler"]],
      },
    }),
  ],
});
