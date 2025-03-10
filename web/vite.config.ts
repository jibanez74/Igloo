import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";

export default defineConfig({
    plugins: [react(), tailwindcss(), TanStackRouterVite()],
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
