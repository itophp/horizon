import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath, URL } from "node:url";
import type { Plugin } from "vite";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));
const loopbackHosts = ["127.0.0.1", "localhost", "[::1]"];
function hostname(value = "") {
  return value.match(/^(\[[^\]]+\]|[^:]+)/)?.[1] ?? "";
}
// Development convenience only: hand the local service credentials to the page
// served by this dev server, so the WebMCP form can be prefilled. The endpoint
// never ships in a build, only answers loopback requests from this same origin,
// sets no CORS headers and disables caching. The pairing key remains required by
// the local service; this just removes the manual copy from the terminal.
function webmcpConnection(): Plugin {
  return {
    name: "baseline-webmcp-connection",
    apply: "serve",
    configureServer(server) {
      server.middlewares.use("/__webmcp/connection", async (req, res) => {
        res.setHeader("Cache-Control", "no-store");
        res.setHeader("Content-Type", "application/json; charset=utf-8");
        const host = hostname(req.headers.host);
        const origin = req.headers.origin;
        if (
          !loopbackHosts.includes(host) ||
          (origin && origin !== `http://${req.headers.host}`)
        )
          return res
            .writeHead(403)
            .end(JSON.stringify({ code: "invalid_origin" }));
        try {
          const saved: { port?: unknown; token?: unknown } = JSON.parse(
            await readFile(
              resolve(
                repositoryRoot,
                process.env.WEBMCP_STATE_DIR ?? ".local/webmcp",
                "connection.json",
              ),
              "utf8",
            ),
          );
          if (
            typeof saved.port !== "number" ||
            !Number.isInteger(saved.port) ||
            saved.port < 1024 ||
            saved.port > 65535 ||
            typeof saved.token !== "string" ||
            !/^[a-f0-9]{64}$/.test(saved.token)
          )
            throw new Error("invalid_connection");
          res.end(JSON.stringify({ port: saved.port, token: saved.token }));
        } catch {
          res.writeHead(404).end(JSON.stringify({ code: "not_running" }));
        }
      });
    },
  };
}
export default defineConfig({
  plugins: [react(), tailwindcss(), webmcpConnection()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "^/mcp$": {
        target: process.env.API_PROXY_TARGET ?? "http://127.0.0.1:8080",
      },
      "/api": {
        target: process.env.API_PROXY_TARGET ?? "http://127.0.0.1:8080",
      },
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    restoreMocks: true,
  },
});
