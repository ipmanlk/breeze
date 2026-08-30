import { defineConfig } from "vite";
import { fileURLToPath } from "node:url";

export default defineConfig({
  build: {
    target: "es2022",
    sourcemap: true,
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true, ws: true },
      "/ws": { target: "http://localhost:8080", changeOrigin: true, ws: true },
    },
  },
});
