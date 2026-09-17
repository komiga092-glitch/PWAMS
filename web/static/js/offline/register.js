import { openOfflineDatabase } from "./db.js";
import { subscribeConnectivity } from "./connectivity.js";
import { syncPendingMutations } from "./sync.js";
import { renderConflictStatus } from "./conflicts.js";
import { pullSync } from "./pull.js";
import { syncPendingMediaUploads } from "./media.js";
import { clearOfflineSession, isOfflineSessionValid, revalidateOnlineSession, offlineSessionExpiredMessage, } from "./session.js";
import { localize } from "./i18n.js";
/* =========================================================
   OFFLINE SESSION WINDOW GUARD (spec §7)
   A user may work offline for max 48 hours. After that the
   PWA layout is blocked until a real server reconnect check
   succeeds.
   ========================================================= */
let sessionLockElement = null;
function showOfflineSessionLock() {
    if (sessionLockElement)
        return;
    const lock = document.createElement("div");
    lock.className = "offline-session-lock";
    lock.setAttribute("role", "alertdialog");
    lock.setAttribute("aria-modal", "true");
    lock.innerHTML = `
    <div class="offline-session-lock-card">
      <h2>${localize("offline.session_locked", "Session locked")}</h2>
      <p>${offlineSessionExpiredMessage()}</p>
      <button type="button" data-session-reconnect>${localize("offline.reconnect", "Reconnect")}</button>
      <p class="offline-session-lock-status" data-session-lock-status hidden></p>
    </div>
  `;
    document.body.appendChild(lock);
    document.body.style.overflow = "hidden";
    sessionLockElement = lock;
    const button = lock.querySelector("[data-session-reconnect]");
    button?.addEventListener("click", () => {
        void (async () => {
            button.disabled = true;
            const status = lock.querySelector("[data-session-lock-status]");
            if (status) {
                status.hidden = false;
                status.textContent = localize("offline.checking_connection", "Checking connection...");
            }
            if (await revalidateOnlineSession()) {
                hideOfflineSessionLock();
            }
            else if (status) {
                status.textContent = localize("offline.reconnect_failed", "Still offline or session rejected. Try again once connectivity returns.");
                button.disabled = false;
            }
        })();
    });
}
function hideOfflineSessionLock() {
    sessionLockElement?.remove();
    sessionLockElement = null;
    document.body.style.overflow = "";
}
async function enforceOfflineSessionWindow() {
    try {
        if (!navigator.onLine && !(await isOfflineSessionValid())) {
            showOfflineSessionLock();
            return;
        }
        if (navigator.onLine || (await isOfflineSessionValid())) {
            hideOfflineSessionLock();
        }
    }
    catch (error) {
        console.error("Offline session window check failed", error);
    }
}
/** Spec §17: warn when browser storage drops below 100 MB free. */
async function checkStorageQuota() {
    if (!navigator.storage?.estimate)
        return;
    try {
        const estimate = await navigator.storage.estimate();
        const quota = estimate.quota ?? 0;
        const usage = estimate.usage ?? 0;
        if (quota > 0 && quota - usage < 100 * 1024 * 1024) {
            console.warn("PWAMS storage warning: less than 100 MB of PWA storage remains.");
            const indicator = document.querySelector("[data-offline-status]");
            if (indicator) {
                indicator.title = localize("offline.storage_warning", "Low device storage - sync may fail.");
            }
        }
    }
    catch {
        // Storage estimation is best-effort only.
    }
}
async function synchronizeOfflineChanges() {
    if (!(await revalidateOnlineSession())) {
        const indicator = document.querySelector("[data-offline-status]");
        if (indicator) {
            indicator.textContent = localize("offline.auth_required", "Authentication required");
            indicator.dataset.state = "offline";
        }
        return;
    }
    try {
        await syncPendingMutations();
    }
    catch (error) {
        console.error("Offline sync failed:", error);
    }
    try {
        await pullSync();
    }
    catch (error) {
        console.error("Offline pull sync failed:", error);
    }
    try {
        await syncPendingMediaUploads();
    }
    catch (error) {
        console.error("Offline media sync failed:", error);
    }
    try {
        await renderConflictStatus();
    }
    catch (error) {
        console.error("Offline conflict status refresh failed:", error);
    }
}
window.addEventListener("online", () => {
    void synchronizeOfflineChanges().then(() => enforceOfflineSessionWindow());
});
window.addEventListener("pwams:sync-conflict", () => {
    void renderConflictStatus();
});
function updateStatusIndicator(online) {
    const indicator = document.querySelector("[data-offline-status]");
    if (!indicator)
        return;
    const label = online
        ? localize("status.online", "Online")
        : localize("status.offline", "Offline");
    indicator.textContent = label;
    indicator.dataset.state = online ? "online" : "offline";
    indicator.setAttribute("aria-label", label);
}
window.addEventListener("pwams:offline-mutation", () => {
    const indicator = document.querySelector("[data-offline-status]");
    if (!indicator || navigator.onLine)
        return;
    indicator.textContent = localize("status.offline_pending", "Offline - Pending changes");
    indicator.dataset.state = "offline";
});
/* =========================================================
   ACCOUNT BOUNDARY (logout / account switch)
   The templates log out via <form method="post" action="/logout">.
   Before the browser leaves the page, every account-bound offline
   artefact (timestamp, owner, cached entities, outbox) is wiped so
   another account on the same device can never reach them.
   ========================================================= */
function isLogoutTarget(rawUrl) {
    try {
        return new URL(rawUrl, window.location.origin).pathname === "/logout";
    }
    catch {
        return false;
    }
}
document.addEventListener("submit", (event) => {
    const form = event.target;
    if (!form || !isLogoutTarget(form.action))
        return;
    // Programmatic form.submit() bypasses the submit event, so this runs once.
    event.preventDefault();
    void (async () => {
        try {
            await clearOfflineSession();
        }
        catch (error) {
            console.error("Offline identity cleanup on logout failed", error);
        }
        form.submit();
    })();
}, true);
document.addEventListener("click", (event) => {
    const anchor = event.target?.closest?.("a[href]");
    if (!anchor || !isLogoutTarget(anchor.getAttribute("href") ?? ""))
        return;
    event.preventDefault();
    void (async () => {
        try {
            await clearOfflineSession();
        }
        catch (error) {
            console.error("Offline identity cleanup on logout failed", error);
        }
        window.location.href = anchor.href;
    })();
}, true);
window.addEventListener("pwams:logout", () => {
    void clearOfflineSession().catch((error) => console.error("Offline identity cleanup failed", error));
});
async function initializeOfflineFoundation() {
    try {
        await openOfflineDatabase();
    }
    catch (error) {
        console.error("PWAMS offline database initialization failed", error);
    }
    subscribeConnectivity(updateStatusIndicator);
    try {
        await renderConflictStatus();
    }
    catch (error) {
        console.error("Offline conflict status initialization failed:", error);
    }
    await enforceOfflineSessionWindow();
    if (navigator.onLine) {
        await synchronizeOfflineChanges();
    }
    await checkStorageQuota();
    if ("serviceWorker" in navigator) {
        try {
            await navigator.serviceWorker.register("/static/js/offline/service-worker.js", { scope: "/" });
        }
        catch (error) {
            console.error("PWAMS service worker registration failed", error);
        }
    }
}
void initializeOfflineFoundation();
//# sourceMappingURL=register.js.map