import {
  getOfflineSessionLastAuthenticatedAt,
  setOfflineSessionLastAuthenticatedAt,
} from "./db.js";

export const OFFLINE_SESSION_WINDOW_MS = 48 * 60 * 60 * 1000;

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
