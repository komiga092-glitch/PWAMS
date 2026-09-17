// _e2e_server.mjs — tiny static server for the E2E (serves real /static modules)
import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { MIME, STATIC, base, port, seedPage, personsPage, reportsPage } from "./_e2e_parts.mjs";

export async function startServer() {
  const syncPushLog = [];
  const seenOperationIds = new Set();
  const server = http.createServer((req, res) => {
    const u = new URL(req.url, base);
    let file = null;
    let body = null;
    if (u.pathname === "/seed") body = seedPage;
    else if (u.pathname === "/persons") body = personsPage;
    else if (u.pathname === "/reports") body = reportsPage;
    else if (u.pathname === "/auth/me") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({ success: true, username: "admin", role: "Super Admin" }));
      return;
    } else if (u.pathname === "/api/v1/sync/push" && req.method === "POST") {
      let raw = "";
      req.on("data", (c) => (raw += c));
      req.on("end", () => {
        let operations = [];
        try { operations = JSON.parse(raw)?.operations ?? []; } catch { /* ignore */ }
        const results = operations.map((op) => {
          const duplicate = seenOperationIds.has(op.id);
          seenOperationIds.add(op.id);
          syncPushLog.push({
            id: op.id,
            entity_type: op.entity_type,
            operation: op.operation,
            record_id: op.record_id,
            idempotency_key: req.headers["idempotency-key"] ?? null,
            duplicate,
          });
          return { id: op.id, status: duplicate ? "duplicate_ignored" : "applied" };
        });
        res.writeHead(200, { "content-type": "application/json" });
        res.end(JSON.stringify({ results }));
      });
      return;
    } else if (u.pathname === "/api/v1/sync/pull") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({ records: [], cursor: "cursor-e2e-0" }));
      return;
    } else if (u.pathname.startsWith("/static/")) {
      file = path.join(STATIC, u.pathname.slice("/static/".length).replace(/\//g, path.sep));
    } else if (u.pathname === "/offline.html") {
      file = path.join(STATIC, "offline.html");
    } else {
      res.writeHead(404, { "content-type": "text/plain" });
      res.end("not found");
      return;
    }
    if (body !== null) {
      res.writeHead(200, { "content-type": "text/html; charset=utf-8" });
      res.end(body);
      return;
    }
    try {
      const data = fs.readFileSync(file);
      const ext = path.extname(file).toLowerCase();
      const headers = { "content-type": MIME[ext] ?? "application/octet-stream" };
      // Mirror production (cmd/server/main.go): allow root scope so the SW
      // can be registered from pages served at "/".
      if (u.pathname === "/static/js/offline/service-worker.js") {
        headers["service-worker-allowed"] = "/";
      }
      res.writeHead(200, headers);
      res.end(data);
    } catch {
      res.writeHead(404, { "content-type": "text/plain" });
      res.end("missing: " + u.pathname);
    }
  });
  await new Promise((resolve, reject) => server.listen(port, "127.0.0.1", (e) => (e ? reject(e) : resolve())));
  server.syncPushLog = syncPushLog;
  return server;
}