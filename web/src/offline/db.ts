export const OFFLINE_DB_NAME = "pwams-offline";
export const OFFLINE_DB_VERSION = 4;

export const OFFLINE_STORES = {
  persons: "persons",
  students: "students",
  donors: "donors",
  donations: "donations",
  aidRequests: "aid_requests",
  loans: "loans",
  loanRepayments: "loan_repayments",
  careProvided: "care_provided",
  revenue: "revenue",
  outbox: "outbox",
  metadata: "metadata",
} as const;

export type OfflineStoreName =
  (typeof OFFLINE_STORES)[keyof typeof OFFLINE_STORES];

export interface OutboxEntry<T = unknown> {
  id?: number;
  operationId: string;
  entityType: OfflineEntityType;
  operation: "CREATE" | "UPDATE";
  recordId: string;
  status: "PENDING" | "UPLOADING" | "SYNCED" | "CONFLICT" | "FAILED";
  error?: string;
  version?: number;
  method: string;
  url: string;
  body?: T;
  headers?: Record<string, string>;
  createdAt: string;
  retryable?: boolean;
}

export interface SyncPullRecord {
  entity_type: OfflineEntityType;
  record_id: string;
  version: number;
  updated_at: string;
  is_deleted: boolean;
  payload: Record<string, unknown>;
}

const OFFLINE_SESSION_METADATA_ID = "offline_session";

export type OfflineEntityType =
  | "person"
  | "student"
  | "donor"
  | "aid_request"
  | "care_provided"
  | "loan"
  | "loan_repayment"
  | "donation"
  | "revenue"
  | "media";

let databasePromise: Promise<IDBDatabase> | undefined;

export function openOfflineDatabase(): Promise<IDBDatabase> {
  if (databasePromise) return databasePromise;

  databasePromise = new Promise((resolve, reject) => {
    const request = indexedDB.open(OFFLINE_DB_NAME, OFFLINE_DB_VERSION);
    request.onerror = () =>
      reject(request.error ?? new Error("Unable to open offline database"));
    request.onupgradeneeded = () => {
      const database = request.result;
      for (const store of Object.values(OFFLINE_STORES)) {
        if (!database.objectStoreNames.contains(store)) {
          const objectStore = database.createObjectStore(store, {
            keyPath: "id",
            autoIncrement: store === OFFLINE_STORES.outbox,
          });
          if (
            store !== OFFLINE_STORES.outbox &&
            store !== OFFLINE_STORES.metadata
          )
            objectStore.createIndex("updatedAt", "updatedAt", {
              unique: false,
            });
        }
      }
    };
    request.onsuccess = () => resolve(request.result);
  });

  return databasePromise;
}

export function createOfflineId(): string {
  return crypto.randomUUID();
}

export async function getSyncCursor(): Promise<string> {
  const database = await openOfflineDatabase();
  return new Promise<string>((resolve, reject) => {
    const request = database
      .transaction(OFFLINE_STORES.metadata, "readonly")
      .objectStore(OFFLINE_STORES.metadata)
      .get("sync_cursor");
    request.onsuccess = () =>
      resolve((request.result as { value?: string } | undefined)?.value ?? "");
    request.onerror = () =>
      reject(request.error ?? new Error("Unable to read sync cursor"));
  });
}

export async function getOfflineSessionLastAuthenticatedAt(): Promise<
  number | null
> {
  const database = await openOfflineDatabase();
  return new Promise<number | null>((resolve, reject) => {
    const request = database
      .transaction(OFFLINE_STORES.metadata, "readonly")
      .objectStore(OFFLINE_STORES.metadata)
      .get(OFFLINE_SESSION_METADATA_ID);
    request.onsuccess = () => {
      const value = request.result as
        | { lastAuthenticatedAt?: number }
        | undefined;
      resolve(
        typeof value?.lastAuthenticatedAt === "number"
          ? value.lastAuthenticatedAt
          : null,
      );
    };
    request.onerror = () =>
      reject(
        request.error ?? new Error("Unable to read offline session metadata"),
      );
  });
}

export async function setOfflineSessionLastAuthenticatedAt(
  timestamp: number,
): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.metadata,
      "readwrite",
    );
    transaction
      .objectStore(OFFLINE_STORES.metadata)
      .put({ id: OFFLINE_SESSION_METADATA_ID, lastAuthenticatedAt: timestamp });
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(
        transaction.error ??
          new Error("Unable to store offline session metadata"),
      );
  });
}

export async function clearOfflineSessionLastAuthenticatedAt(): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.metadata,
      "readwrite",
    );
    transaction
      .objectStore(OFFLINE_STORES.metadata)
      .delete(OFFLINE_SESSION_METADATA_ID);
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(
        transaction.error ??
          new Error("Unable to clear offline session metadata"),
      );
  });
}

const STORE_BY_ENTITY: Record<OfflineEntityType, OfflineStoreName> = {
  person: OFFLINE_STORES.persons,
  student: OFFLINE_STORES.students,
  donor: OFFLINE_STORES.donors,
  aid_request: OFFLINE_STORES.aidRequests,
  care_provided: OFFLINE_STORES.careProvided,
  loan: OFFLINE_STORES.loans,
  loan_repayment: OFFLINE_STORES.loanRepayments,
  donation: OFFLINE_STORES.donations,
  revenue: OFFLINE_STORES.revenue,
  media: OFFLINE_STORES.outbox,
};

export async function mergePulledRecords(
  records: SyncPullRecord[],
  cursor: string,
): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const stores = [
      ...new Set(
        records
          .map((record) => STORE_BY_ENTITY[record.entity_type])
          .filter((store): store is OfflineStoreName => Boolean(store)),
      ),
      OFFLINE_STORES.outbox,
      OFFLINE_STORES.metadata,
    ];
    const transaction = database.transaction(stores, "readwrite");
    const outboxRequest = transaction
      .objectStore(OFFLINE_STORES.outbox)
      .getAll();
    outboxRequest.onsuccess = () => {
      const protectedRecords = new Set(
        (outboxRequest.result as OutboxEntry[])
          .filter(
            (entry) =>
              entry.status === "PENDING" || entry.status === "CONFLICT" || entry.status === "FAILED",
          )
          .map((entry) => `${entry.entityType}:${entry.recordId}`),
      );

      for (const pulled of records) {
        const storeName = STORE_BY_ENTITY[pulled.entity_type];
        if (
          !storeName ||
          protectedRecords.has(`${pulled.entity_type}:${pulled.record_id}`)
        )
          continue;
        const store = transaction.objectStore(storeName);
        const existingRequest = store.get(pulled.record_id);
        existingRequest.onsuccess = () => {
          const existing = existingRequest.result as
            | { version?: number }
            | undefined;
          if ((existing?.version ?? 0) >= pulled.version) return;
          store.put({
            ...pulled.payload,
            id: pulled.record_id,
            version: pulled.version,
            updatedAt: pulled.updated_at,
            is_deleted: pulled.is_deleted,
          });
        };
      }

      transaction
        .objectStore(OFFLINE_STORES.metadata)
        .put({ id: "sync_cursor", value: cursor });
    };
    outboxRequest.onerror = () =>
      reject(
        outboxRequest.error ?? new Error("Unable to inspect offline mutations"),
      );
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Unable to merge pulled records"));
  });
}

export async function saveOfflineMutation<T extends Record<string, unknown>>(
  store: OfflineStoreName,
  record: T & { id: string; updatedAt: string },
  mutation: Omit<OutboxEntry<T>, "id">,
): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      [store, OFFLINE_STORES.outbox],
      "readwrite",
    );
    transaction.objectStore(store).put(record);
    transaction.objectStore(OFFLINE_STORES.outbox).add(mutation);
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Unable to save offline mutation"));
  });
}

export async function putOfflineRecord<T extends { id: IDBValidKey }>(
  store: OfflineStoreName,
  record: T,
): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(store, "readwrite");
    transaction.objectStore(store).put(record);
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Unable to store offline record"));
  });
}

export async function listOfflineRecords<T>(
  store: OfflineStoreName,
): Promise<T[]> {
  const database = await openOfflineDatabase();
  return new Promise<T[]>((resolve, reject) => {
    const request = database
      .transaction(store, "readonly")
      .objectStore(store)
      .getAll();
    request.onsuccess = () => resolve(request.result as T[]);
    request.onerror = () =>
      reject(request.error ?? new Error("Unable to read offline records"));
  });
}

export async function enqueueOfflineRequest<T>(
  entry: OutboxEntry<T>,
): Promise<number> {
  const database = await openOfflineDatabase();
  return new Promise<number>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.outbox,
      "readwrite",
    );
    const request = transaction.objectStore(OFFLINE_STORES.outbox).add(entry);
    transaction.oncomplete = () => resolve(Number(request.result));
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Unable to queue offline request"));
  });
}

export async function getPendingMutations(): Promise<OutboxEntry[]> {
  const entries = await listOfflineRecords<OutboxEntry>(OFFLINE_STORES.outbox);
  return entries
    .filter(
      (entry) =>
        entry.entityType !== "media" &&
        (entry.status === "PENDING" || !entry.status),
    )
    .sort((left, right) => (left.id ?? 0) - (right.id ?? 0));
}

export async function getPendingMediaMutations(): Promise<OutboxEntry[]> {
  const entries = await listOfflineRecords<OutboxEntry>(OFFLINE_STORES.outbox);
  return entries
    .filter(
      (entry) =>
        entry.entityType === "media" &&
        (entry.status === "PENDING" ||
          entry.status === "UPLOADING" ||
          (entry.status === "FAILED" && entry.retryable)),
    )
    .sort((left, right) => (left.id ?? 0) - (right.id ?? 0));
}

export async function updateMutationStatus(
  id: number,
  status: OutboxEntry["status"],
  error?: string,
  retryable = false,
): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.outbox,
      "readwrite",
    );
    const store = transaction.objectStore(OFFLINE_STORES.outbox);
    const request = store.get(id);
    request.onsuccess = () => {
      const entry = request.result as OutboxEntry | undefined;
      if (!entry) {
        reject(new Error("Offline mutation not found"));
        return;
      }
      entry.status = status;
      entry.error = error;
      entry.retryable = retryable;
      store.put(entry);
    };
    request.onerror = () =>
      reject(request.error ?? new Error("Unable to read offline mutation"));
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(
        transaction.error ?? new Error("Unable to update offline mutation"),
      );
  });
}

export async function clearMutationBody(id: number): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.outbox,
      "readwrite",
    );
    const store = transaction.objectStore(OFFLINE_STORES.outbox);
    const request = store.get(id);
    request.onsuccess = () => {
      const entry = request.result as OutboxEntry | undefined;
      if (!entry) {
        reject(new Error("Offline mutation not found"));
        return;
      }
      delete entry.body;
      store.put(entry);
    };
    request.onerror = () =>
      reject(request.error ?? new Error("Unable to read offline mutation"));
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Unable to clear temporary media"));
  });
}

export async function pruneSyncedOutboxEntries(): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.outbox,
      "readwrite",
    );
    const store = transaction.objectStore(OFFLINE_STORES.outbox);
    const request = store.getAll();
    request.onsuccess = () => {
      const entries = request.result as OutboxEntry[];
      const now = Date.now();
      const maxAge = 7 * 24 * 60 * 60 * 1000;
      for (const entry of entries) {
        if (entry.status === "SYNCED") {
          const age = now - new Date(entry.createdAt).getTime();
          if (age > maxAge) {
            store.delete(entry.id!);
          }
        }
      }
    };
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Unable to prune outbox"));
  });
}

const OFFLINE_PIN_METADATA_ID = "offline_pin_hash";

export async function getOfflinePinHash(): Promise<string | null> {
  const database = await openOfflineDatabase();
  return new Promise<string | null>((resolve, reject) => {
    const request = database
      .transaction(OFFLINE_STORES.metadata, "readonly")
      .objectStore(OFFLINE_STORES.metadata)
      .get(OFFLINE_PIN_METADATA_ID);
    request.onsuccess = () =>
      resolve(
        (request.result as { value?: string } | undefined)?.value ?? null,
      );
    request.onerror = () =>
      reject(
        request.error ?? new Error("Unable to read offline PIN metadata"),
      );
  });
}

export async function setOfflinePinHash(hash: string): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.metadata,
      "readwrite",
    );
    transaction
      .objectStore(OFFLINE_STORES.metadata)
      .put({ id: OFFLINE_PIN_METADATA_ID, value: hash });
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(
        transaction.error ??
          new Error("Unable to store offline PIN metadata"),
      );
  });
}
