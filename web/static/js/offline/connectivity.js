let online = navigator.onLine;
const listeners = new Set();
function publish(nextOnline) {
    if (online === nextOnline)
        return;
    online = nextOnline;
    listeners.forEach((listener) => listener(online));
}
window.addEventListener("online", () => publish(true));
window.addEventListener("offline", () => publish(false));
export function isOnline() {
    return online;
}
export function subscribeConnectivity(listener) {
    listeners.add(listener);
    listener(online);
    return () => listeners.delete(listener);
}
//# sourceMappingURL=connectivity.js.map