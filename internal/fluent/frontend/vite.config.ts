/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The page is embedded in ODM (internal/fluent/app.go) and served by Wails
// from the root, so assets use relative paths.
export default defineConfig({
  plugins: [react()],
  base: "./",
  build: { outDir: "dist", emptyOutDir: true, chunkSizeWarningLimit: 2000 },
  test: { environment: "node" },
});
