import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go server embeds dist/ and serves it; in development `npm run dev`
// proxies everything that isn't the frontend to the server on :8080.
export default defineConfig({
  plugins: [react()],
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    proxy: { "/healthz": "http://localhost:8080" },
  },
});
