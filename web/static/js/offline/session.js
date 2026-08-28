import { getOfflineSessionLastAuthenticatedAt, setOfflineSessionLastAuthenticatedAt, getOfflinePinHash, setOfflinePinHash, } from "./db.js";
export const OFFLINE_SESSION_WINDOW_MS = 48 * 60 * 60 * 1000;
export const OFFLINE_PIN_REVALIDATE_MS = 30 * 60 * 1000;
export async function isOfflineSessionValid(now = Date.now()) {
    const lastAuthenticatedAt = await getOfflineSessionLastAuthenticatedAt();
    return isOfflineSessionTimestampValid(lastAuthenticatedAt, now);
}
export function isOfflineSessionTimestampValid(lastAuthenticatedAt, now = Date.now()) {
    return (lastAuthenticatedAt !== null &&
        now >= lastAuthenticatedAt &&
        now - lastAuthenticatedAt < OFFLINE_SESSION_WINDOW_MS);
}
export async function requireOfflinePin(pin) {
    const stored = await getOfflinePinHash();
    if (!stored)
        return true;
    const hash = await hashPin(pin);
    return hash === stored;
}
export async function setupOfflinePin(pin) {
    const hash = await hashPin(pin);
    await setOfflinePinHash(hash);
}
async function hashPin(pin) {
    const encoder = new TextEncoder();
    const data = encoder.encode(pin);
    const hashBuffer = await crypto.subtle.digest("SHA-256", data);
    return Array.from(new Uint8Array(hashBuffer))
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
}
export async function revalidateOnlineSession() {
    if (!navigator.onLine)
        return false;
    try {
        const response = await fetch("/auth/me", {
            method: "GET",
            credentials: "include",
            headers: { Accept: "application/json" },
        });
        if (!response.ok)
            return false;
        await setOfflineSessionLastAuthenticatedAt(Date.now());
        return true;
    }
    catch {
        return false;
    }
}
export const OFFLINE_SESSION_EXPIRED_MESSAGE = "Your offline session has expired. Reconnect to the server and sign in again.";
//# sourceMappingURL=session.js.map