import { openOfflineDatabase } from "./db.js";
import { subscribeConnectivity } from "./connectivity.js";
import { syncPendingMutations } from "./sync.js";
import { renderConflictStatus } from "./conflicts.js";
import { pullSync } from "./pull.js";

async function synchronizeOfflineChanges(): Promise<void> {
  try {
    await syncPendingMutations();
  } catch (error) {
    console.error("Offline sync failed:", error);
  }
  try {
    await pullSync();
  } catch (error) {
    console.error("Offline pull sync failed:", error);
  }
  try {
    await renderConflictStatus();
  } catch (error) {
    console.error("Offline conflict status refresh failed:", error);
  }
}

window.addEventListener("online", () => {
  void synchronizeOfflineChanges();
});

window.addEventListener("pwams:sync-conflict", () => {
  void renderConflictStatus();
});

function updateStatusIndicator(online: boolean): void {
  const indicator = document.querySelector<HTMLElement>(
    "[data-offline-status]",
  );
  if (!indicator) return;
  indicator.textContent = online ? "Online" : "Offline";
  indicator.dataset.state = online ? "online" : "offline";
  indicator.setAttribute("aria-label", online ? "Online" : "Offline");
}

window.addEventListener("pwams:offline-mutation", () => {
  const indicator = document.querySelector<HTMLElement>(
    "[data-offline-status]",
  );
  if (!indicator || navigator.onLine) return;
  indicator.textContent = "Offline - Pending changes";
  indicator.dataset.state = "offline";
});

async function initializeOfflineFoundation(): Promise<void> {
  try {
    await openOfflineDatabase();
  } catch (error) {
    console.error("PWAMS offline database initialization failed", error);
  }

  subscribeConnectivity(updateStatusIndicator);
  try {
    await renderConflictStatus();
  } catch (error) {
    console.error("Offline conflict status initialization failed:", error);
  }

  if (navigator.onLine) {
    await synchronizeOfflineChanges();
  }

  if ("serviceWorker" in navigator) {
    try {
      await navigator.serviceWorker.register(
        "/static/js/offline/service-worker.js",
        { scope: "/" },
      );
    } catch (error) {
      console.error("PWAMS service worker registration failed", error);
    }
  }
}

void initializeOfflineFoundation();
