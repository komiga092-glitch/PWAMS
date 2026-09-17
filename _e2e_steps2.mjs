// _e2e_steps2.mjs — steps 4..7: expiry, restore, unsupported page, reconnect
import { base } from "./_e2e_parts.mjs";

const setTs = (ms) => `new Promise((resolve) => {
  const r = indexedDB.open("pwams-offline", 4);
  r.onsuccess = () => {
    const tx = r.result.transaction("metadata","readwrite");
    tx.objectStore("metadata").put({ id: "offline_session", lastAuthenticatedAt: ${ms} });
    tx.oncomplete = () => resolve();
  };
})`;
const readOutbox = `new Promise((resolve) => {
  const r = indexedDB.open("pwams-offline", 4);
  r.onsuccess = () => {
    const c = r.result.transaction("outbox","readwrite").objectStore("outbox").getAll();
    c.onsuccess = () => resolve(c.result);
  };
})`;

export async function step3(page, results) {
  await page.evaluate(setTs("Date.now() - (49 * 60 * 60 * 1000)"));
  await page.evaluate(() => window.dispatchEvent(new CustomEvent("pwams:offline-mutation")));
  await page.waitForTimeout(800);
  const expired = await page.locator("[data-offline-session-expired]").count();
  results.push((expired === 1 ? "PASS" : "FAIL") + " | EXPIRY: last_authenticated_at=now-49h shows session-expired state (count=" + expired + ")");
  await page.evaluate(setTs("Date.now()"));
  await page.evaluate(() => window.dispatchEvent(new CustomEvent("pwams:offline-mutation")));
  await page.waitForTimeout(800);
  const rows = await page.locator("table.data-table tbody tr").count();
  const exp2 = await page.locator("[data-offline-session-expired]").count();
  results.push((rows === 2 && exp2 === 0 ? "PASS" : "FAIL")
    + " | RESTORE: timestamp restored, local rendering resumes (rows=" + rows + ", expiredBanners=" + exp2 + ")");

  // KEY ORIGINAL-ISSUE SCENARIO (STEP 12.5 item 17):
  // LOGIN -> OFFLINE -> REFRESH must re-render the supported page from
  // local IndexedDB data (SW navigation fallback -> offline shell ->
  // pages.js auto-render on DOMContentLoaded).
  await page.reload({ waitUntil: "domcontentloaded" }).catch(() => {});
  await page.waitForTimeout(1500);
  const reloadRows = await page.locator("table.data-table tbody tr").count();
  const reloadTxt = (await page.locator("table.data-table tbody").innerText().catch(() => "")) ?? "";
  const reloadExpired = await page.locator("[data-offline-session-expired]").count();
  const swState = await page.evaluate(async () => {
    const indicator = document.querySelector("[data-offline-status]");
    let probeError = null;
    let probeRows = null;
    try {
      const store = window.pwamsOfflineReadStore;
      probeRows = store ? (await store("persons")).length : "no-readstore";
    } catch (error) {
      probeError = String(error).slice(0, 120);
    }
    return {
      controller: !!(navigator.serviceWorker && navigator.serviceWorker.controller),
      url: location.href,
      hasTable: !!document.querySelector("table.data-table"),
      readStore: typeof window.pwamsOfflineReadStore,
      probeRows,
      probeError,
      indicator: indicator ? indicator.textContent : "none",
      bodyHead: (document.body ? document.body.innerText : "").slice(0, 100),
    };
  }).catch((error) => ({ error: String(error).slice(0, 160) }));
  results.push("OBS | offline-refresh diagnostics: " + JSON.stringify(swState));
  results.push((reloadRows === 2 && reloadTxt.includes("Alice Seeker") && reloadExpired === 0 ? "PASS" : "FAIL")
    + " | OFFLINE REFRESH: reload while offline re-renders local data (rows=" + reloadRows
    + ", has Alice=" + reloadTxt.includes("Alice Seeker") + ", expiredBanners=" + reloadExpired + ")");
}

export async function step4(page, context, results) {
  await context.setOffline(false);
  await page.goto(base + "/reports", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(600);
  await context.setOffline(true);
  await page.evaluate(() => window.dispatchEvent(new Event("offline")));
  await page.evaluate(() => window.dispatchEvent(new CustomEvent("pwams:offline-mutation")));
  await page.waitForTimeout(800);
  const repBody = (await page.locator("main").innerText().catch(() => "")) ?? "";
  const repExpired = await page.locator("[data-offline-session-expired]").count();
  results.push((repExpired === 0 ? "PASS" : "FAIL")
    + " | UNSUPPORTED /reports offline: no session-expired banner (count=" + repExpired + ")");
  results.push("OBS | unsupported page offline body snippet: " + JSON.stringify((repBody || "EMPTY").slice(0, 160)));
}

export async function step5(page, context, results, server) {
  await context.setOffline(false);
  await page.evaluate(() => window.dispatchEvent(new Event("online")));
  // Wait for the real compiled sync.js to push the PENDING outbox entry.
  let outbox2 = null;
  for (let i = 0; i < 20; i++) {
    await page.waitForTimeout(400);
    outbox2 = await page.evaluate(readOutbox);
    if ((outbox2 ?? []).some((o) => o.status === "SYNCED")) break;
  }
  const synced = (outbox2 ?? []).filter((o) => o.status === "SYNCED");
  const stillPending = (outbox2 ?? []).filter((o) => o.status === "PENDING").length;
  results.push((synced.length >= 1 ? "PASS" : "FAIL")
    + " | RECONNECT: compiled sync.js pushed outbox entry -> status SYNCED (synced=" + synced.length + ", pending=" + stillPending + ")");

  const pushLog = server?.syncPushLog ?? [];
  const personPush = pushLog.find((e) => e.entity_type === "person" && e.operation === "CREATE");
  results.push((personPush && personPush.idempotency_key === personPush.id ? "PASS" : "FAIL")
    + " | SERVER RECEIVED push with matching Idempotency-Key (entry=" + JSON.stringify(personPush ?? null) + ")");

  // Replay the same operation id directly: server must ignore the duplicate.
  const replay = await page.evaluate(async (op) => {
    const response = await fetch("/api/v1/sync/push", {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": op.operationId,
        "X-CSRF-Token": (document.cookie.match(/csrf_token=([^;]+)/) ?? [])[1] ?? "",
      },
      body: JSON.stringify({
        operations: [{
          id: op.operationId,
          entity_type: "person",
          operation: "CREATE",
          record_id: op.recordId,
          client_version: 1,
          payload: { full_name: "Bob Offline" },
          created_at: op.createdAt,
        }],
      }),
    });
    return response.json();
  }, synced[0] ?? { operationId: "missing", recordId: "missing", createdAt: "" });
  const replayStatus = replay?.results?.[0]?.status;
  results.push((replayStatus === "duplicate_ignored" ? "PASS" : "FAIL")
    + " | IDEMPOTENCY: replayed operation id rejected as duplicate (status=" + replayStatus + ")");
  results.push("OBS | PostgreSQL verification NOT EXECUTED (E2E stub server; Go/Postgres layer covered by go test suite)");
  results.push("OBS | conflict handling on real 409 + PostgreSQL round-trip NOT EXECUTED in this stub environment");
}