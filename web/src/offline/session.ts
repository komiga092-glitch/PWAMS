import {
  getOfflineSessionLastAuthenticatedAt,
  setOfflineSessionLastAuthenticatedAt,
  getOfflinePinHash,
  setOfflinePinHash,
} from "./db.js";

export const OFFLINE_SESSION_WINDOW_MS = 48 * 60 * 60 * 1000;
export const OFFLINE_PIN_REVALIDATE_MS = 30 * 60 * 1000;

export async function isOfflineSessionValid(
  now = Date.now(),
): Promise<boolean> {
  const lastAuthenticatedAt = await getOfflineSessionLastAuthenticatedAt();
  return isOfflineSessionTimestampValid(lastAuthenticatedAt, now);
}

export function isOfflineSessionTimestampValid(
  lastAuthenticatedAt: number | null,
  now = Date.now(),
): boolean {
  return (
    lastAuthenticatedAt !== null &&
    now >= lastAuthenticatedAt &&
    now - lastAuthenticatedAt < OFFLINE_SESSION_WINDOW_MS
  );
}

export async function requireOfflinePin(pin: string): Promise<boolean> {
  const stored = await getOfflinePinHash();
  if (!stored) return true;
  const hash = await hashPin(pin);
  return hash === stored;
}

export async function setupOfflinePin(pin: string): Promise<void> {
  const hash = await hashPin(pin);
  await setOfflinePinHash(hash);
}

async function hashPin(pin: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(pin);
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(hashBuffer))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export async function revalidateOnlineSession(): Promise<boolean> {
  if (!navigator.onLine) return false;
  try {
    const response = await fetch("/auth/me", {
      method: "GET",
      credentials: "include",
      headers: { Accept: "application/json" },
    });
    if (!response.ok) return false;
    await setOfflineSessionLastAuthenticatedAt(Date.now());
    return true;
  } catch {
    return false;
  }
}

export const OFFLINE_SESSION_EXPIRED_MESSAGE =
  "Your offline session has expired. Reconnect to the server and sign in again.";
