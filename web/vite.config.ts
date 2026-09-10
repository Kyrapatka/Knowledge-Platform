import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: process.env.API_URL || "http://127.0.0.1:8080",
        changeOrigin: false,
      },
      "/health": "http://127.0.0.1:8080",
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          math: ["katex", "rehype-katex", "remark-math"],
          markdown: ["react-markdown", "remark-gfm"],
        },
      },
    },
  },
});
