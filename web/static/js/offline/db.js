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
};
let databasePromise;
export function openOfflineDatabase() {
    if (databasePromise)
        return databasePromise;
    databasePromise = new Promise((resolve, reject) => {
        const request = indexedDB.open(OFFLINE_DB_NAME, OFFLINE_DB_VERSION);
        request.onerror = () => reject(request.error ?? new Error("Unable to open offline database"));
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
export function createOfflineId() {
    return crypto.randomUUID();
}
export async function saveOfflineMutation(store, record, mutation) {
    const database = await openOfflineDatabase();
    await new Promise((resolve, reject) => {
        const transaction = database.transaction([store, OFFLINE_STORES.outbox], "readwrite");
        transaction.objectStore(store).put(record);
        transaction.objectStore(OFFLINE_STORES.outbox).add(mutation);
        transaction.oncomplete = () => resolve();
        transaction.onerror = () => reject(transaction.error ?? new Error("Unable to save offline mutation"));
    });
}
export async function putOfflineRecord(store, record) {
    const database = await openOfflineDatabase();
    await new Promise((resolve, reject) => {
        const transaction = database.transaction(store, "readwrite");
        transaction.objectStore(store).put(record);
        transaction.oncomplete = () => resolve();
        transaction.onerror = () => reject(transaction.error ?? new Error("Unable to store offline record"));
    });
}
export async function listOfflineRecords(store) {
    const database = await openOfflineDatabase();
    return new Promise((resolve, reject) => {
        const request = database
            .transaction(store, "readonly")
            .objectStore(store)
            .getAll();
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error ?? new Error("Unable to read offline records"));
    });
}
export async function enqueueOfflineRequest(entry) {
    const database = await openOfflineDatabase();
    await new Promise((resolve, reject) => {
        const transaction = database.transaction(OFFLINE_STORES.outbox, "readwrite");
        transaction.objectStore(OFFLINE_STORES.outbox).add(entry);
        transaction.oncomplete = () => resolve();
        transaction.onerror = () => reject(transaction.error ?? new Error("Unable to queue offline request"));
    });
}
export async function getPendingMutations() {
    const entries = await listOfflineRecords(OFFLINE_STORES.outbox);
    return entries
        .filter((entry) => entry.status === "PENDING" || !entry.status)
        .sort((left, right) => (left.id ?? 0) - (right.id ?? 0));
}
export async function updateMutationStatus(id, status, error) {
    const database = await openOfflineDatabase();
    await new Promise((resolve, reject) => {
        const transaction = database.transaction(OFFLINE_STORES.outbox, "readwrite");
        const store = transaction.objectStore(OFFLINE_STORES.outbox);
        const request = store.get(id);
        request.onsuccess = () => {
            const entry = request.result;
            if (!entry) {
                reject(new Error("Offline mutation not found"));
                return;
            }
            entry.status = status;
            entry.error = error;
            store.put(entry);
        };
        request.onerror = () => reject(request.error ?? new Error("Unable to read offline mutation"));
        transaction.oncomplete = () => resolve();
        transaction.onerror = () => reject(transaction.error ?? new Error("Unable to update offline mutation"));
    });
}
//# sourceMappingURL=db.js.map