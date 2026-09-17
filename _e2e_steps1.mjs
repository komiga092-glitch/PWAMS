// _e2e_steps1.mjs — steps 0..3: seed/auth timestamp, online page, offline render, offline create+outbox
import { base } from "./_e2e_parts.mjs";

const readMetaTs = `new Promise((resolve) => {
  const r = indexedDB.open("pwams-offline", 4);
  r.onsuccess = () => {
    const g = r.result.transaction("metadata","readonly").objectStore("metadata").get("offline_session");
    g.onsuccess = () => resolve(g.result?.lastAuthenticatedAt);
  };
})`;
const readOutbox = `new Promise((resolve) => {
  const r = indexedDB.open("pwams-offline", 4);
  r.onsuccess = () => {
    const c = r.result.transaction("outbox","readwrite").objectStore("outbox").getAll();
    c.onsuccess = () => resolve(c.result);
  };
})`;

export async function step0(page, results) {
  await page.goto(base + "/seed", { waitUntil: "domcontentloaded" });
  await page.waitForFunction(() => document.body.dataset.ready === "1");
  const tsBefore = await page.evaluate(readMetaTs);
  results.push((typeof tsBefore === "number" && tsBefore > 0 ? "PASS" : "FAIL")
    + " | seeded last_authenticated_at present in IndexedDB (type=" + typeof tsBefore + ")");
  await page.evaluate(async () => {
    const m = await import("/static/js/offline/session.js");
    await m.recordSuccessfulAuthentication("admin");
  });
  const tsAfter = await page.evaluate(readMetaTs);
  results.push((tsAfter > 0 && tsAfter >= tsBefore ? "PASS" : "FAIL")
    + " | recordSuccessfulAuthentication() stored last_authenticated_at (before=" + tsBefore + ", after=" + tsAfter + ")");
  const owner = await page.evaluate(async () => {
    const m = await import("/static/js/offline/db.js");
    return m.getOfflineSessionOwner();
  });
  results.push((owner === "admin" ? "PASS" : "FAIL")
    + " | offline session owner bound to authenticated account (owner=" + owner + ")");
  await page.goto(base + "/persons", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);
  const onlineRows = await page.locator("table.data-table tbody tr").count();
  results.push((onlineRows === 0 ? "PASS" : "FAIL")
    + " | ONLINE /persons: server DOM untouched by offline renderer (tbody rows=" + onlineRows + ")");
}

export async function step1(page, context, results) {
  // Ensure the service worker is active BEFORE going offline so the
  // precache mirrors production asset availability (mutations.js etc.).
  await page.evaluate(() => Promise.race([
    (navigator.serviceWorker && navigator.serviceWorker.ready) || Promise.resolve(null),
    new Promise((resolve) => setTimeout(resolve, 5000)),
  ]));
  await context.setOffline(true);
  await page.evaluate(() => window.dispatchEvent(new Event("offline")));
  await page.evaluate(() => window.dispatchEvent(new CustomEvent("pwams:offline-mutation")));
  await page.waitForTimeout(900);
  const rows = await page.locator("table.data-table tbody tr").count();
  const txt = (await page.locator("table.data-table tbody").innerText()) ?? "";
  const expired = await page.locator("[data-offline-session-expired]").count();
  results.push((rows === 1 && txt.includes("Alice Seeker") ? "PASS" : "FAIL")
    + " | OFFLINE: supported /persons renders local IndexedDB row (rows=" + rows + ", has Alice=" + txt.includes("Alice Seeker") + ")");
  results.push((expired === 0 ? "PASS" : "FAIL") + " | OFFLINE: no session-expired banner inside valid 48h window (count=" + expired + ")");
}

export async function step2(page, results) {
  await page.evaluate(async () => {
    const m = await import("/static/js/offline/mutations.js");
    await m.savePendingMutation("person", "CREATE", {
      id: "p2-offline",
      full_name: "Bob Offline",
      nic_passport: "990000000V",
      phone: "0711111111",
      status: "active",
    });
  });
  await page.waitForTimeout(900);
  const rows = await page.locator("table.data-table tbody tr").count();
  const outbox = await page.evaluate(readOutbox);
  const pending = (outbox ?? []).filter((o) => o.status === "PENDING");
  results.push((rows === 2 ? "PASS" : "FAIL") + " | OFFLINE CREATE: local record rendered (rows=" + rows + ")");
  results.push((pending.length >= 1 && pending.some((o) => o.entityType === "person") ? "PASS" : "FAIL")
    + " | OUTBOX: PENDING person mutation created (count=" + pending.length + ")");
}