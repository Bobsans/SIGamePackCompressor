import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import replacementPlugin from "./plugins/vite-replace-plugin";
import path from "path";

export default defineConfig({
  plugins: [
    vue(),
    replacementPlugin({
      API_SERVER_URL: process.env.API_SERVER_URL ?? "http://localhost:8000"
    })
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@theme": path.resolve(__dirname, "./src/@assets/style")
    }
  }
});
