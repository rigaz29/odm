import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";

// One page per style, and an index linking them.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: "./",
  build: {
    rollupOptions: {
      input: {
        index: resolve(__dirname, "index.html"),
        a: resolve(__dirname, "a.html"),
        b: resolve(__dirname, "b.html"),
        d: resolve(__dirname, "d.html"),
      },
    },
    chunkSizeWarningLimit: 3000,
  },
});
