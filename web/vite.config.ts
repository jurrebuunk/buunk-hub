import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go server embeds dist/ and serves it. For frontend work, sign in once
// at http://localhost:8080 (the cookie is for localhost, any port), then use
// `npm run dev`: it proxies the server's paths to :8080.
export default defineConfig({
  plugins: [react()],
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    proxy: Object.fromEntries(
      ["/api", "/auth", "/healthz"].map((path) => [
        path,
        {
          target: "http://localhost:8080",
          // The server only takes changes from its own origin; in
          // development the page is on :5173.
          headers: { Origin: "http://localhost:8080" },
        },
      ]),
    ),
  },
});
