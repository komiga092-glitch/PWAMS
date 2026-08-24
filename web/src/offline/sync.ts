import { isOnline } from "./connectivity.js";
import { getPendingMutations, updateMutationStatus } from "./db.js";

const SYNC_ENDPOINT = "/api/v1/sync/push";
let syncPromise: Promise<void> | undefined;

export function syncPendingMutations(): Promise<void> {
  if (syncPromise) return syncPromise;
  syncPromise = runSyncPendingMutations().finally(() => {
    syncPromise = undefined;
  });
  return syncPromise;
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
