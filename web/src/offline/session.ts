import {
  getOfflineSessionLastAuthenticatedAt,
  setOfflineSessionLastAuthenticatedAt,
  getOfflineSessionOwner,
  setOfflineSessionOwner,
  clearOfflineIdentity,
  getOfflinePinHash,
  setOfflinePinHash,
} from "./db.js";
import { localize } from "./i18n.js";

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
  const outcome = await verifyPin(pin, stored);
  if (outcome.valid && outcome.upgradedHash) {
    // Transparent KDF migration: a v1 (legacy) PIN hash that verifies
    // is immediately re-stored in the hardened v2 format.
    await setOfflinePinHash(outcome.upgradedHash);
  }
  return outcome.valid;
}

export async function setupOfflinePin(pin: string): Promise<void> {
  await setOfflinePinHash(await hashPin(pin));
}

// ---------------------------------------------------------------------------
// Offline PIN key derivation (KDF)
//
// The stored value is self-describing and versioned:
//   v2 (current): "pbkdf2-sha256$<iterations>$<salt-b64>$<hash-b64>"
//     PBKDF2-HMAC-SHA-256 via WebCrypto, 16-byte random per-device salt,
//     600000 iterations (OWASP guidance for PBKDF2-HMAC-SHA-256),
//     32-byte derived output, constant-time comparison.
//   v1 (legacy): bare 64-char lowercase hex — an unsalted single-pass
//     SHA-256 digest. It is only accepted for verification and is
//     transparently upgraded to v2 on the next successful PIN entry;
//     new PINs are always stored in the v2 format.
// ---------------------------------------------------------------------------

const PIN_HASH_V2_PREFIX = "pbkdf2-sha256";
const PBKDF2_ITERATIONS = 600_000;
const PBKDF2_SALT_BYTES = 16;
const PBKDF2_OUTPUT_BYTES = 32;

interface PinVerification {
  valid: boolean;
  /** Non-null when a legacy v1 hash verified and must be re-stored as v2. */
  upgradedHash: string | null;
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array<ArrayBuffer> {
  return Uint8Array.from(atob(value), (c) => c.charCodeAt(0));
}

function constantTimeEqual(left: Uint8Array, right: Uint8Array): boolean {
  if (left.length !== right.length) return false;
  let diff = 0;
  for (let i = 0; i < left.length; i++) {
    diff |= (left[i] ?? 0) ^ (right[i] ?? 0);
  }
  return diff === 0;
}

async function derivePinBits(
  pin: string,
  salt: BufferSource,
  iterations: number,
  outputBytes: number,
): Promise<Uint8Array<ArrayBuffer>> {
  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(pin),
    { name: "PBKDF2" },
    false,
    ["deriveBits"],
  );
  const bits = await crypto.subtle.deriveBits(
    { name: "PBKDF2", hash: "SHA-256", salt, iterations },
    keyMaterial,
    outputBytes * 8,
  );
  return new Uint8Array(bits);
}

async function hashPin(pin: string): Promise<string> {
  const salt = crypto.getRandomValues(
    new Uint8Array(new ArrayBuffer(PBKDF2_SALT_BYTES)),
  );
  const bits = await derivePinBits(
    pin,
    salt,
    PBKDF2_ITERATIONS,
    PBKDF2_OUTPUT_BYTES,
  );
  return `${PIN_HASH_V2_PREFIX}$${PBKDF2_ITERATIONS}$${bytesToBase64(salt)}$${bytesToBase64(bits)}`;
}

async function verifyPin(pin: string, stored: string): Promise<PinVerification> {
  if (stored.startsWith(`${PIN_HASH_V2_PREFIX}$`)) {
    const parts = stored.split("$");
    if (parts.length !== 4) {
      return { valid: false, upgradedHash: null };
    }
    const iterations = Number.parseInt(parts[1] ?? "", 10);
    if (!Number.isFinite(iterations) || iterations < 1) {
      return { valid: false, upgradedHash: null };
    }
    let salt: Uint8Array<ArrayBuffer>;
    let expected: Uint8Array<ArrayBuffer>;
    try {
      salt = base64ToBytes(parts[2] ?? "");
      expected = base64ToBytes(parts[3] ?? "");
    } catch {
      return { valid: false, upgradedHash: null };
    }
    const bits = await derivePinBits(
      pin,
      salt,
      iterations,
      expected.length,
    );
    return { valid: constantTimeEqual(bits, expected), upgradedHash: null };
  }

  // Legacy v1: unsalted hex SHA-256 — verify, then upgrade to v2.
  if (/^[0-9a-f]{64}$/.test(stored)) {
    const digest = new Uint8Array(
      await crypto.subtle.digest(
        "SHA-256",
        new TextEncoder().encode(pin),
      ),
    );
    const expected = Uint8Array.from(
      (stored.match(/../g) ?? []).map((hex) => parseInt(hex, 16)),
    );
    if (constantTimeEqual(digest, expected)) {
      return { valid: true, upgradedHash: await hashPin(pin) };
    }
    return { valid: false, upgradedHash: null };
  }

  return { valid: false, upgradedHash: null };
}

/**
 * The ONE authentication-recording path for the offline session window.
 *
 * Called exclusively after a server-validated authentication success:
 * 1. the login flow (recorded by the post-login /auth/me revalidation), and
 * 2. `revalidateOnlineSession()` whenever /auth/me confirms the session.
 *
 * It is never called on page load, offline transition, failed login, or
 * local navigation. Client code cannot inject an arbitrary timestamp.
 */
export async function recordSuccessfulAuthentication(
  username?: string,
): Promise<void> {
  await setOfflineSessionLastAuthenticatedAt(Date.now());
  if (typeof username === "string" && username.length > 0) {
    await setOfflineSessionOwner(username);
  }
}

/**
 * Account-boundary wipe: clears the offline timestamp, the owning account,
 * all cached entity data and the pending mutation outbox (logout / account
 * switch). See `clearOfflineIdentity` in db.ts for the security model.
 */
export async function clearOfflineSession(): Promise<void> {
  await clearOfflineIdentity();
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
    const payload = (await response.json().catch(() => null)) as
      | { username?: string }
      | null;
    const authenticatedUser =
      typeof payload?.username === "string" && payload.username.length > 0
        ? payload.username
        : ((await getOfflineSessionOwner()) ?? undefined);
    await recordSuccessfulAuthentication(authenticatedUser);
    return true;
  } catch {
    return false;
  }
}

const OFFLINE_SESSION_EXPIRED_FALLBACK =
  "Your offline session has expired. Reconnect to the server and sign in again.";

export const OFFLINE_SESSION_EXPIRED_MESSAGE = OFFLINE_SESSION_EXPIRED_FALLBACK;

/** Localised (EN/TA/SI via window.PWAMS_I18N) offline session expiry message. */
export function offlineSessionExpiredMessage(): string {
  return localize("offline.session_expired", OFFLINE_SESSION_EXPIRED_FALLBACK);
}
