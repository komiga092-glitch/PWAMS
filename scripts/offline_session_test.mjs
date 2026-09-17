/**
 * Runtime tests for the offline session lifecycle (STEP 12.5).
 *
 * Imports the REAL compiled modules (web/static/js/offline/session.js and
 * db.js) and drives them through an in-memory IndexedDB shim, proving:
 *   - last_authenticated_at is recorded only via the single
 *     recordSuccessfulAuthentication() path (server-validated revalidation)
 *   - failed auth / offline / network loss never refresh the timestamp
 *   - the 48-hour offline window boundaries are exact
 *   - clearOfflineSession() wipes timestamp, owner, entities and outbox
 *
 * Run with: node --test scripts/offline_session_test.mjs
 */
import { test } from "node:test";
import assert from "node:assert/strict";

/* ------------------------------------------------------------------ */
/* Minimal in-memory IndexedDB shim (only what db.js actually uses).   */
/* Request executors run in microtasks; a transaction completes on the  */
/* following macrotask once every created request has executed.        */
/* ------------------------------------------------------------------ */

class MemoryRequest {
  constructor(executor, transaction) {
    this.result = undefined;
    this.error = undefined;
    this.onerror = null;
    this.onsuccess = null;
    if (transaction) transaction.pending += 1;
    queueMicrotask(() => {
      try {
        executor();
        this.onsuccess?.();
      } catch (error) {
        this.error = error;
        this.onerror?.();
      } finally {
        if (transaction) {
          transaction.pending -= 1;
          if (transaction.pending === 0) transaction.scheduleComplete();
        }
      }
    });
  }
}

class MemoryObjectStore {
  constructor(store, transaction) {
    this.store = store;
    this.transaction = transaction;
  }
  get(key) {
    const request = new MemoryRequest(() => {
      request.result = this.store.data.get(key);
    }, this.transaction);
    return request;
  }
  getAll() {
    const request = new MemoryRequest(() => {
      request.result = [...this.store.data.values()];
    }, this.transaction);
    return request;
  }
  put(value) {
    const request = new MemoryRequest(() => {
      this.store.data.set(value.id ?? this.store.data.size + 1, value);
    }, this.transaction);
    return request;
  }
  add(value) {
    const request = new MemoryRequest(() => {
      this.store.data.set(value.id ?? this.store.data.size + 1, value);
    }, this.transaction);
    return request;
  }
  delete(key) {
    const request = new MemoryRequest(() => {
      this.store.data.delete(key);
    }, this.transaction);
    return request;
  }
  clear() {
    const request = new MemoryRequest(() => {
      this.store.data.clear();
    }, this.transaction);
    return request;
  }
}

class MemoryTransaction {
  constructor(database, stores, mode) {
    this.database = database;
    this.stores = stores;
    this.mode = mode;
    this.pending = 0;
    this.oncomplete = null;
    this.onerror = null;
    this.completed = false;
    this.scheduleComplete();
  }
  scheduleComplete() {
    setTimeout(() => {
      if (this.completed || this.pending !== 0) return;
      this.completed = true;
      if (this.database.closed) {
        this.error = new Error("database closed");
        this.onerror?.();
        return;
      }
      this.oncomplete?.();
    }, 0);
  }
  objectStore(name) {
    if (!this.stores.includes(name)) {
      throw new Error(`store ${name} not part of transaction`);
    }
    return new MemoryObjectStore(this.database.stores.get(name), this);
  }
}

class MemoryDatabase {
  constructor() {
    this.stores = new Map();
    this.closed = false;
  }
  transaction(stores, mode) {
    const names = typeof stores === "string" ? [stores] : [...stores];
    for (const name of names) {
      if (!this.stores.has(name)) this.stores.set(name, { data: new Map() });
    }
    return new MemoryTransaction(this, names, mode);
  }
  close() {
    this.closed = true;
  }
}

function installIndexedDBShim() {
  const database = new MemoryDatabase();
  globalThis.indexedDB = {
    open() {
      const request = new MemoryRequest(() => {});
      Object.defineProperty(request, "onsuccess", {
        get() {
          return request._onsuccess;
        },
        set(handler) {
          request._onsuccess = function () {
            request.result = database;
            handler();
          };
        },
        configurable: true,
      });
      return request;
    },
  };
  return database;
}

const HOUR = 60 * 60 * 1000;

function setNavigatorOnline(online) {
  Object.defineProperty(globalThis, "navigator", {
    value: { onLine: online },
    configurable: true,
    writable: true,
  });
}

installIndexedDBShim();
setNavigatorOnline(true);
globalThis.window = globalThis;

const {
  isOfflineSessionValid,
  isOfflineSessionTimestampValid,
  recordSuccessfulAuthentication,
  revalidateOnlineSession,
  clearOfflineSession,
} = await import("../web/static/js/offline/session.js");
const {
  getOfflineSessionLastAuthenticatedAt,
  getOfflineSessionOwner,
  putOfflineRecord,
  listOfflineRecords,
} = await import("../web/static/js/offline/db.js");

async function readTimestamp() {
  return getOfflineSessionLastAuthenticatedAt();
}

test("no timestamp recorded before any successful authentication", async () => {
  assert.equal(await readTimestamp(), null);
  assert.equal(await getOfflineSessionOwner(), null);
  assert.equal(await isOfflineSessionTimestampValid(null, Date.now()), false);
});

test("recordSuccessfulAuthentication stores timestamp and owner", async () => {
  await recordSuccessfulAuthentication("alice");
  const ts = await readTimestamp();
  assert.equal(typeof ts, "number");
  assert.ok(Math.abs(Date.now() - ts) < 5_000);
  assert.equal(await getOfflineSessionOwner(), "alice");
});

test("48h boundary: 47h59m59s999 is valid", () => {
  const base = Date.now();
  assert.equal(
    isOfflineSessionTimestampValid(base - (48 * HOUR - 1), base),
    true,
  );
});

test("48h boundary: exactly 48h is expired", () => {
  const base = Date.now();
  assert.equal(isOfflineSessionTimestampValid(base - 48 * HOUR, base), false);
});

test("48h boundary: 48h + 1ms is expired", () => {
  const base = Date.now();
  assert.equal(
    isOfflineSessionTimestampValid(base - (48 * HOUR + 1), base),
    false,
  );
});

test("48h boundary: clock rollback (timestamp in the future) is invalid", () => {
  const base = Date.now();
  assert.equal(isOfflineSessionTimestampValid(base + 60_000, base), false);
});

test("failed revalidation (HTTP 401) does not refresh the timestamp", async () => {
  const before = await readTimestamp();
  globalThis.fetch = async () => ({ ok: false, status: 401 });
  assert.equal(await revalidateOnlineSession(), false);
  assert.equal(await readTimestamp(), before);
});

test("network failure does not refresh the timestamp", async () => {
  const before = await readTimestamp();
  globalThis.fetch = async () => {
    throw new Error("network down");
  };
  assert.equal(await revalidateOnlineSession(), false);
  assert.equal(await readTimestamp(), before);
});

test("offline navigator never triggers revalidation", async () => {
  const before = await readTimestamp();
  setNavigatorOnline(false);
  let fetchCalled = false;
  globalThis.fetch = async () => {
    fetchCalled = true;
    return { ok: true, status: 200, json: async () => ({ username: "x" }) };
  };
  assert.equal(await revalidateOnlineSession(), false);
  assert.equal(fetchCalled, false);
  assert.equal(await readTimestamp(), before);
  setNavigatorOnline(true);
});

test("successful /auth/me revalidation records timestamp + owner (single path)", async () => {
  await clearOfflineSession();
  assert.equal(await readTimestamp(), null);
  globalThis.fetch = async () => ({
    ok: true,
    status: 200,
    json: async () => ({ success: true, username: "bob" }),
  });
  assert.equal(await revalidateOnlineSession(), true);
  assert.ok((await readTimestamp()) > 0);
  assert.equal(await getOfflineSessionOwner(), "bob");
});

test("logout wipe removes timestamp, owner, entities and outbox atomically", async () => {
  await recordSuccessfulAuthentication("alice");
  await putOfflineRecord("persons", { id: "p1", full_name: "Alice" });
  await putOfflineRecord("outbox", {
    id: 1,
    operationId: "op-1",
    entityType: "person",
    recordId: "p1",
    status: "PENDING",
  });
  assert.equal((await listOfflineRecords("persons")).length, 1);
  assert.equal((await listOfflineRecords("outbox")).length, 1);

  await clearOfflineSession();

  assert.equal(await readTimestamp(), null);
  assert.equal(await getOfflineSessionOwner(), null);
  assert.equal((await listOfflineRecords("persons")).length, 0);
  assert.equal((await listOfflineRecords("outbox")).length, 0);
});

test("session valid again only after a fresh successful authentication", async () => {
  await clearOfflineSession();
  assert.equal(await isOfflineSessionValid(), false);
  await recordSuccessfulAuthentication("alice");
  assert.equal(await isOfflineSessionValid(), true);
});

/* ------------------------------------------------------------------ */
/* Offline PIN / KDF (STEP 12.6)                                       */
/* ------------------------------------------------------------------ */

import { createHash } from "node:crypto";

const {
  requireOfflinePin,
  setupOfflinePin,
} = await import("../web/static/js/offline/session.js");
const {
  getOfflinePinHash,
  setOfflinePinHash,
} = await import("../web/static/js/offline/db.js");

test("setupOfflinePin stores the hardened v2 PBKDF2 format and verifies", async () => {
  await setupOfflinePin("2468");
  const stored = await getOfflinePinHash();
  assert.match(stored, /^pbkdf2-sha256\$600000\$[A-Za-z0-9+/=]+\$[A-Za-z0-9+/=]+$/);
  assert.equal(await requireOfflinePin("2468"), true);
  assert.equal(await requireOfflinePin("0000"), false);
  assert.equal(await requireOfflinePin(""), false);
});

test("legacy v1 (unsalted SHA-256) PIN hash verifies and is transparently upgraded to v2", async () => {
  const legacyDigest = createHash("sha256").update("1357").digest("hex");
  await setOfflinePinHash(legacyDigest);
  assert.equal(await getOfflinePinHash(), legacyDigest);

  assert.equal(await requireOfflinePin("1357"), true);

  const upgraded = await getOfflinePinHash();
  assert.notEqual(upgraded, legacyDigest);
  assert.match(upgraded, /^pbkdf2-sha256\$600000\$/);

  // Still valid after the upgrade, wrong PIN still rejected.
  assert.equal(await requireOfflinePin("1357"), true);
  assert.equal(await requireOfflinePin("9753"), false);
});

test("corrupt PIN hash never verifies (fail closed)", async () => {
  await setOfflinePinHash("not-a-valid-pin-hash");
  assert.equal(await requireOfflinePin("anything"), false);
  await setOfflinePinHash("pbkdf2-sha256$600000$!!!bad-base64!!!$also-bad");
  assert.equal(await requireOfflinePin("anything"), false);
});
