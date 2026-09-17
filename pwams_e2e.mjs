// pwams_e2e.mjs — orchestrator: start server, launch headless Chrome, run steps, write report
import { chromium } from "playwright";
import fs from "node:fs";
import path from "node:path";
import { ROOT, base } from "./_e2e_parts.mjs";
import { startServer } from "./_e2e_server.mjs";
import { step0, step1, step2 } from "./_e2e_steps1.mjs";
import { step3, step4, step5 } from "./_e2e_steps2.mjs";

const results = [];
const consoleLogs = [];

let server = null;
let browser = null;
let context = null;
let page = null;

try {
  server = await startServer();
  results.push("INFO | static server up at " + base);
} catch (e) {
  results.push("ENVFAIL | server start: " + String(e).slice(0, 400));
}

try {
  browser = await chromium.launch({ channel: "chrome", headless: true });
  context = await browser.newContext();
  page = await context.newPage();
  page.on("console", (m) => consoleLogs.push(m.type() + ": " + m.text()));
  results.push("INFO | chrome launched (channel=chrome, headless)");
} catch (e) {
  results.push("ENVFAIL | chrome launch: " + String(e).slice(0, 400));
}

if (page) {
  const steps = [
    ["step0_auth_and_timestamp", () => step0(page, results)],
    ["step1_offline_render", () => step1(page, context, results)],
    ["step2_offline_create_outbox", () => step2(page, results)],
    ["step3_expiry_restore", () => step3(page, results)],
    ["step4_unsupported", () => step4(page, context, results)],
    ["step5_reconnect_sync", () => step5(page, context, results, server)],
  ];
  for (const [name, fn] of steps) {
    try {
      await fn();
    } catch (e) {
      results.push("ENVFAIL | " + name + ": " + String(e).slice(0, 400));
    }
  }
} else {
  results.push("ENVFAIL | no browser page available; steps skipped");
}

try { if (context) await context.close(); } catch (e) { /* ignore */ }
try { if (browser) await browser.close(); } catch (e) { /* ignore */ }
try { if (server) server.close(); } catch (e) { /* ignore */ }

const out = [
  "PWAMS STEP 12.5 — REAL-BROWSER OFFLINE E2E (headless Chrome)",
  "============================================================",
  "node " + process.version,
  "playwright 1.63.0 (installed --no-save)",
  "chrome: C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe (channel=chrome)",
  base,
  "",
  ...results,
  "",
  "console messages (first 30):",
  ...consoleLogs.slice(0, 30),
];
fs.writeFileSync(path.join(ROOT, "_step12_logs", "browser_e2e.txt"), out.join("\r\n"), "utf8");
console.log("E2E DONE -> _step12_logs/browser_e2e.txt");