/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The page is embedded in ODM (internal/gui/app.go) and served by Wails from
// the root, so assets use relative paths. The translations come from
// internal/i18n, outside this folder.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: "./",
  build: { outDir: "dist", emptyOutDir: true, chunkSizeWarningLimit: 2000 },
  server: { fs: { allow: ["..", "../../i18n"] } },
  // The first Intl.NumberFormat loads its locale data, which once took a
  // Windows runner 7 s: more than vitest's 5 s for a test.
  test: { environment: "node", testTimeout: 30000 },
});
