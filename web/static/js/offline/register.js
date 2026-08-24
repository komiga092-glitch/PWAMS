import { openOfflineDatabase } from "./db.js";
import { subscribeConnectivity } from "./connectivity.js";
import { syncPendingMutations } from "./sync.js";
import { renderConflictStatus } from "./conflicts.js";
import { pullSync } from "./pull.js";
import { syncPendingMediaUploads } from "./media.js";
import { revalidateOnlineSession } from "./session.js";
async function synchronizeOfflineChanges() {
    if (!(await revalidateOnlineSession())) {
        const indicator = document.querySelector("[data-offline-status]");
        if (indicator) {
            indicator.textContent = "Authentication required";
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
    void synchronizeOfflineChanges();
});
window.addEventListener("pwams:sync-conflict", () => {
    void renderConflictStatus();
});
function updateStatusIndicator(online) {
    const indicator = document.querySelector("[data-offline-status]");
    if (!indicator)
        return;
    indicator.textContent = online ? "Online" : "Offline";
    indicator.dataset.state = online ? "online" : "offline";
    indicator.setAttribute("aria-label", online ? "Online" : "Offline");
}
window.addEventListener("pwams:offline-mutation", () => {
    const indicator = document.querySelector("[data-offline-status]");
    if (!indicator || navigator.onLine)
        return;
    indicator.textContent = "Offline - Pending changes";
    indicator.dataset.state = "offline";
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
    if (navigator.onLine) {
        await synchronizeOfflineChanges();
    }
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