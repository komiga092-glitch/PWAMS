import { openOfflineDatabase } from "./db.js";
import { subscribeConnectivity } from "./connectivity.js";
import { syncPendingMutations } from "./sync.js";
import { renderConflictStatus } from "./conflicts.js";
import { pullSync } from "./pull.js";
import { syncPendingMediaUploads } from "./media.js";
import {
  isOfflineSessionValid,
  revalidateOnlineSession,
  OFFLINE_SESSION_EXPIRED_MESSAGE,
} from "./session.js";

/* =========================================================
   OFFLINE SESSION WINDOW GUARD (spec §7)
   A user may work offline for max 48 hours. After that the
   PWA layout is blocked until a real server reconnect check
   succeeds.
   ========================================================= */

let sessionLockElement: HTMLElement | null = null;

function showOfflineSessionLock(): void {
  if (sessionLockElement) return;

  const lock = document.createElement("div");
  lock.className = "offline-session-lock";
  lock.setAttribute("role", "alertdialog");
  lock.setAttribute("aria-modal", "true");
  lock.innerHTML = `
    <div class="offline-session-lock-card">
      <h2>Session locked</h2>
      <p>${OFFLINE_SESSION_EXPIRED_MESSAGE}</p>
      <button type="button" data-session-reconnect>Reconnect</button>
      <p class="offline-session-lock-status" data-session-lock-status hidden></p>
    </div>
  `;
  document.body.appendChild(lock);
  document.body.style.overflow = "hidden";
  sessionLockElement = lock;

  const button = lock.querySelector<HTMLButtonElement>(
    "[data-session-reconnect]",
  );
  button?.addEventListener("click", () => {
    void (async () => {
      button.disabled = true;
      const status = lock.querySelector<HTMLElement>(
        "[data-session-lock-status]",
      );
      if (status) {
        status.hidden = false;
        status.textContent = "Checking connection...";
      }

      if (await revalidateOnlineSession()) {
        hideOfflineSessionLock();
      } else if (status) {
        status.textContent =
          "Still offline or session rejected. Try again once connectivity returns.";
        button.disabled = false;
      }
    })();
  });
}

function hideOfflineSessionLock(): void {
  sessionLockElement?.remove();
  sessionLockElement = null;
  document.body.style.overflow = "";
}

async function enforceOfflineSessionWindow(): Promise<void> {
  try {
    if (!navigator.onLine && !(await isOfflineSessionValid())) {
      showOfflineSessionLock();
      return;
    }
    if (navigator.onLine || (await isOfflineSessionValid())) {
      hideOfflineSessionLock();
    }
  } catch (error) {
    console.error("Offline session window check failed", error);
  }
}

/** Spec §17: warn when browser storage drops below 100 MB free. */
async function checkStorageQuota(): Promise<void> {
  if (!navigator.storage?.estimate) return;
  try {
    const estimate = await navigator.storage.estimate();
    const quota = estimate.quota ?? 0;
    const usage = estimate.usage ?? 0;
    if (quota > 0 && quota - usage < 100 * 1024 * 1024) {
      console.warn(
        "PWAMS storage warning: less than 100 MB of PWA storage remains.",
      );
      const indicator = document.querySelector<HTMLElement>(
        "[data-offline-status]",
      );
      if (indicator) {
        indicator.title = "Low device storage - sync may fail.";
      }
    }
  } catch {
    // Storage estimation is best-effort only.
  }
}

async function synchronizeOfflineChanges(): Promise<void> {
  if (!(await revalidateOnlineSession())) {
    const indicator = document.querySelector<HTMLElement>(
      "[data-offline-status]",
    );
    if (indicator) {
      indicator.textContent = "Authentication required";
      indicator.dataset.state = "offline";
    }
    return;
  }

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
    await syncPendingMediaUploads();
  } catch (error) {
    console.error("Offline media sync failed:", error);
  }
  try {
    await renderConflictStatus();
  } catch (error) {
    console.error("Offline conflict status refresh failed:", error);
  }
}

window.addEventListener("online", () => {
  void synchronizeOfflineChanges().then(() =>
    enforceOfflineSessionWindow(),
  );
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

  await enforceOfflineSessionWindow();

  if (navigator.onLine) {
    await synchronizeOfflineChanges();
  }

  await checkStorageQuota();

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
