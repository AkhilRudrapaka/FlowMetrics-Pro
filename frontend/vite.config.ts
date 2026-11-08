import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// In development the API is proxied so the browser sees ONE origin; that is what lets the
// SameSite=Strict refresh cookie work without any CORS setup. Production does the same in nginx.
const api = "http://localhost:8080"; // the Go backend; change if you run it on another port

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { port: 5173, proxy: { "/api": api, "/healthz": api } },
  test: { environment: "node", include: ["src/**/*.test.ts"] },
});
