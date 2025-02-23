import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
import path from "path";
import { fileURLToPath } from "url";
var __dirname = path.dirname(fileURLToPath(import.meta.url));
export default defineConfig({
    plugins: [react(), tailwindcss(), TanStackRouterVite()],
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
    server: {
        port: 3000,
        open: true,
        proxy: {
            "/api/v1": {
                target: "https://swifty.hare-crocodile.ts.net/api/v1",
                changeOrigin: true,
                rewrite: function (path) {
                    return path.replace(/^\/api\/v1/, "");
                }, // Remove '/api/v1' from the request path
            },
        },
    },
    build: {
        outDir: "../backend/cmd/client",
        emptyOutDir: true,
    },
});
