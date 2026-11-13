import { defineConfig } from "vite";
export default defineConfig({
  server: { port: 5179, proxy: { "/api": "http://127.0.0.1:4310" } },
  build: { outDir: "dist", emptyOutDir: true },
});
