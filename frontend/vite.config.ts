import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const localHostName = os.hostname().toLowerCase();
const localAllowedHosts = [localHostName, localHostName + ".local"];

function androidApkDownload(): Plugin {
  const apkPath = path.resolve(__dirname, "public/downloads/chakuchuri-android.apk");
  return {
    name: "android-apk-download",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = req.url?.split("?")[0] || "";
        if (url !== "/downloads/chakuchuri-android.apk") return next();
        if (!fs.existsSync(apkPath)) {
          res.statusCode = 404;
          res.setHeader("Content-Type", "text/plain; charset=utf-8");
          res.end("Android APK is not ready yet.");
          return;
        }
        const stat = fs.statSync(apkPath);
        res.statusCode = 200;
        res.setHeader("Content-Type", "application/vnd.android.package-archive");
        res.setHeader("Content-Length", String(stat.size));
        res.setHeader("Content-Disposition", 'attachment; filename="chakuchuri-android.apk"');
        res.setHeader("Cache-Control", "no-store");
        fs.createReadStream(apkPath).pipe(res);
      });
    }
  };
}

export default defineConfig({
  plugins: [react(), androidApkDownload()],
  server: {
    allowedHosts: localAllowedHosts,
    host: "0.0.0.0",
    port: 5170,
    strictPort: true,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8002",
        changeOrigin: true,
        ws: true,
        xfwd: true,
        timeout: 0,
        proxyTimeout: 0,
        configure: (proxy) => {
          proxy.on("error", (err) => {
            console.warn("[vite] api proxy error:", err.message);
          });
          proxy.on("proxyReqWs", (_proxyReq, _req, socket) => {
            socket.on("error", (err) => {
              console.warn("[vite] ws proxy socket error:", err.message);
            });
          });
        }
      },
      "/health": {
        target: "http://127.0.0.1:8002",
        changeOrigin: true,
        xfwd: true,
        timeout: 0,
        proxyTimeout: 0
      }
    }
  },
  preview: {
    allowedHosts: localAllowedHosts,
    host: "0.0.0.0",
    port: 5170,
    strictPort: true
  }
});
