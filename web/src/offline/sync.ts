import { isOnline } from "./connectivity.js";
import { getPendingMutations, updateMutationStatus, pruneSyncedOutboxEntries } from "./db.js";
import { getCsrfToken } from "./csrf.js";

const SYNC_ENDPOINT = "/api/v1/sync/push";
let syncPromise: Promise<void> | undefined;

interface SyncManagerLike {
  register(tag: string): Promise<void>;
}

export function syncPendingMutations(): Promise<void> {
  if (syncPromise) return syncPromise;
  syncPromise = runSyncPendingMutations().finally(() => {
    syncPromise = undefined;
  });
  return syncPromise;
}

export async function requestBackgroundSync(): Promise<void> {
  if (!("serviceWorker" in navigator) || !("SyncManager" in window)) {
    return;
  }

  try {
    const registration = await navigator.serviceWorker.ready;
    const syncManager = (registration as unknown as { sync?: SyncManagerLike }).sync;
    if (syncManager) {
      await syncManager.register("pwams-background-sync");
    }
  } catch {
    console.warn("Background sync registration failed");
  }
}

export function setupPeriodicSync(intervalMs = 5 * 60 * 1000): void {
  if (!isOnline()) return;

  setInterval(async () => {
    if (isOnline()) {
      await syncPendingMutations();
      await pruneSyncedOutboxEntries();
    }
  }, intervalMs);
}

async function runSyncPendingMutations(): Promise<void> {
  if (!isOnline()) {
    return;
  }

  const pending = await getPendingMutations();

  if (pending.length === 0) {
    return;
  }

  const order = [
    "person",
    "student",
    "aid_request",
    "care_provided",
    "loan",
    "loan_repayment",
    "donor",
  ];
  pending.sort(
    (left, right) =>
      order.indexOf(left.entityType) - order.indexOf(right.entityType),
  );

  for (const mutation of pending) {
    if (mutation.id === undefined) continue;
    try {
      const response = await fetch(SYNC_ENDPOINT, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": mutation.operationId,
          "X-CSRF-Token": getCsrfToken(),
        },
        body: JSON.stringify({
          operations: [
            {
              id: mutation.operationId,
              entity_type: mutation.entityType,
              operation: mutation.operation,
              record_id: mutation.recordId,
              client_version: mutation.version ?? 1,
              payload: mutation.body,
              created_at: mutation.createdAt,
            },
          ],
        }),
      });

      if (response.status === 200) {
        await updateMutationStatus(mutation.id, "SYNCED");

        continue;
      }

      if (response.status === 409) {
        const result = await response.json().catch(() => null);

        await updateMutationStatus(
          mutation.id,
          "CONFLICT",
          result?.message ?? "Synchronization conflict",
        );
        window.dispatchEvent(new CustomEvent("pwams:sync-conflict"));

        continue;
      }

      if (response.status >= 400 && response.status < 500) {
        const result = await response.json().catch(() => null);

        await updateMutationStatus(
          mutation.id,
          "FAILED",
          result?.message ?? "Synchronization failed",
        );

        continue;
      }

      // Server/network failure:
      // leave the mutation pending so it can retry later.
      break;
    } catch {
      // Network failure:
      // keep the mutation as PENDING.
      break;
    }
  }
}
