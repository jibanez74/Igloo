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
          // E2E_MOCK_API_PORT lets the Playwright run put the mock API on a
          // free port; plain `bun run dev` keeps talking to the Go server.
          target: `http://127.0.0.1:${process.env.E2E_MOCK_API_PORT ?? "8080"}`,
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
      rolldownOptions: {
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
      // 5s vitest default under parallel-suite load and cold module import.
      // On a machine already busy with other work (load average 12-20) they
      // took over 15s, so the headroom is generous; it only lengthens failing
      // runs.
      testTimeout: 30000,
      // Undo spies and stubbed globals between tests so no suite can leak setup
      // into the next. Individual files therefore need no restore hooks.
      restoreMocks: true,
      unstubGlobals: true,
      // Node 25+ defines its own localStorage global. Without
      // --localstorage-file it evaluates to undefined (with an
      // ExperimentalWarning) and hides jsdom's, so turn Web Storage off in the
      // workers. Node 26 names the flag --webstorage (has() lists only the
      // positive form); older Node knows only --no-experimental-webstorage,
      // and Node before 22 has neither.
      execArgv: process.allowedNodeEnvironmentFlags.has("--webstorage")
        ? ["--no-webstorage"]
        : process.allowedNodeEnvironmentFlags.has(
              "--no-experimental-webstorage",
            )
          ? ["--no-experimental-webstorage"]
          : [],
      coverage: {
        provider: "v8",
        reporter: ["text", "json-summary", "html"],
        reportsDirectory: "./coverage",
        include: ["src/**/*.{ts,tsx}"],
        exclude: [
          "src/test/**",
          "src/routeTree.gen.ts",
          "src/types/openapi.gen.ts",
          "src/main.tsx",
          "src/**/*.d.ts",
        ],
      },
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
