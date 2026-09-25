import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");
  const apiTarget = env.VITE_API_TARGET || "http://localhost:8000";

  return {
    base: "/dashboard",
    plugins: [react()],
    server: {
      port: 3000,
      host: "0.0.0.0",
      proxy: {
        "/api": apiTarget,
        "/sub": apiTarget,
      },
    },
    build: {
      outDir: "../server/web/dist/dashboard",
      emptyOutDir: true,
      rollupOptions: {
        input: {
          index: "index.html",
          access: "access/index.html",
        },
      },
    },
  };
});
