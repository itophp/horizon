import { defineConfig, mergeConfig } from "vite";
import base from "./vite.config";
export default mergeConfig(
  base,
  defineConfig({
    build: {
      outDir: "dist/showcase",
      rollupOptions: { input: "runtime-showcase.html" },
    },
  }),
);
