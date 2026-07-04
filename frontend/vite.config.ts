import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Proxy API calls to the Go backend during local development so the
    // frontend can use relative URLs (see src/api/client.ts) in every
    // environment without a CORS round trip.
    proxy: {
      "/api/v1": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
      "/readyz": "http://localhost:8080",
    },
  },
});
