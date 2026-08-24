export const OFFLINE_DB_NAME = "pwams-offline";
export const OFFLINE_DB_VERSION = 3;

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
} as const;

export type OfflineStoreName =
  (typeof OFFLINE_STORES)[keyof typeof OFFLINE_STORES];

export interface OutboxEntry<T = unknown> {
  id?: number;
  entityType: OfflineEntityType;
  operation: "CREATE" | "UPDATE";
  recordId: string;
  status: "PENDING" | "SYNCED" | "CONFLICT" | "FAILED";
  error?: string;
  version?: number;
  method: string;
  url: string;
  body?: T;
  headers?: Record<string, string>;
  createdAt: string;
}

export type OfflineEntityType =
  | "person"
  | "student"
  | "donor"
  | "aid_request"
  | "care_provided"
  | "loan"
  | "loan_repayment";

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
          if (store !== OFFLINE_STORES.outbox)
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
): Promise<void> {
  const database = await openOfflineDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(
      OFFLINE_STORES.outbox,
      "readwrite",
    );
    transaction.objectStore(OFFLINE_STORES.outbox).add(entry);
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Unable to queue offline request"));
  });
}

export async function getPendingMutations(): Promise<OutboxEntry[]> {
  const entries = await listOfflineRecords<OutboxEntry>(OFFLINE_STORES.outbox);
  return entries
    .filter((entry) => entry.status === "PENDING" || !entry.status)
    .sort((left, right) => (left.id ?? 0) - (right.id ?? 0));
}

export async function updateMutationStatus(
  id: number,
  status: OutboxEntry["status"],
  error?: string,
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
