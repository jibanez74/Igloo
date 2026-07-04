import { defineConfig } from "vitest/config";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import { fileURLToPath, URL } from "node:url";
import { visualizer } from "rollup-plugin-visualizer";

export default defineConfig(({ mode }) => {
  const shouldAnalyze = mode === "analyze";

  return {
    server: {
      port: 3000,
      open: true,

      proxy: {
        "/api": {
          target: "http://127.0.0.1:8080",
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
        plugins: shouldAnalyze
          ? [
              visualizer({
                filename: "dist/bundle-analysis.html",
                template: "treemap",
                gzipSize: true,
                brotliSize: true,
                open: false,
              }),
            ]
          : undefined,
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
      // Full-router-load route tests (movies/movie-details/home) can exceed the
      // 5s vitest default under parallel-suite load and cold module import,
      // producing flaky timeouts in CI. Give them headroom.
      testTimeout: 15000,
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
  };
    });
