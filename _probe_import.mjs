// _probe_import.mjs — reproduce the mutations.js dynamic import failure
import { chromium } from "playwright";
import { startServer } from "./_e2e_server.mjs";
import { base, personsPage } from "./_e2e_parts.mjs";

const server = await startServer();
const browser = await chromium.launch({ channel: "chrome", headless: true });
const context = await browser.newContext();
const page = await context.newPage();
page.on("console", (m) => console.log("[console]", m.type(), m.text()));
page.on("pageerror", (e) => console.log("[pageerror]", e.message));
await page.route("**/persons", (route) => route.fulfill({ contentType: "text/html", body: personsPage }));
await page.goto(base + "/persons", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(500);
const result = await page.evaluate(async () => {
  try {
    const m = await import("/static/js/offline/mutations.js");
    return "OK exports=" + Object.keys(m).join(",");
  } catch (e) {
    return "IMPORT_ERROR: " + String(e) + " | cause=" + String(e?.cause ?? "");
  }
});
console.log("RESULT:", result);
const direct = await page.evaluate(async () => {
  try {
    const m = await import("/static/js/offline/db.js");
    return "DB_OK exports=" + Object.keys(m).join(",");
  } catch (e) {
    return "DB_IMPORT_ERROR: " + String(e);
  }
});
console.log("DIRECT:", direct);
await browser.close();
server.close();
