import {
  createOfflineId,
  enqueueOfflineRequest,
  saveOfflineMutation,
  type OfflineEntityType,
  type OfflineStoreName,
} from "./db.js";
import {
  isOfflineSessionValid,
  offlineSessionExpiredMessage,
} from "./session.js";

const STORE_BY_ENTITY: Record<OfflineEntityType, OfflineStoreName> = {
  person: "persons",
  student: "students",
  donor: "donors",
  aid_request: "aid_requests",
  care_provided: "care_provided",
  loan: "loans",
  loan_repayment: "loan_repayments",
  donation: "donations",
  revenue: "revenue",
  media: "outbox",
};

const ENDPOINT_BY_ENTITY: Record<OfflineEntityType, string> = {
  person: "persons",
  student: "students",
  donor: "donors",
  aid_request: "aid-requests",
  care_provided: "care-provided",
  loan: "loans",
  loan_repayment: "loan-repayments",
  donation: "donations",
  revenue: "revenue",
  media: "files/upload",
};

export function offlineMutationsEnabled(): boolean {
  return !navigator.onLine;
}

export async function savePendingMutation<T extends Record<string, unknown>>(
  entityType: OfflineEntityType,
  operation: "CREATE" | "UPDATE",
  payload: T,
  recordId = typeof payload.id === "string" ? payload.id : createOfflineId(),
): Promise<string> {
  if (!(await isOfflineSessionValid())) {
    throw new Error(offlineSessionExpiredMessage());
  }

  const now = new Date().toISOString();
  const operationId = createOfflineId();
  const record = { ...payload, id: recordId, updatedAt: now } as T & {
    id: string;
    updatedAt: string;
  };
  await saveOfflineMutation(STORE_BY_ENTITY[entityType], record, {
    operationId,
    entityType,
    operation,
    recordId,
    status: "PENDING",
    method: operation === "CREATE" ? "POST" : "PUT",
    url: `/${ENDPOINT_BY_ENTITY[entityType]}${operation === "UPDATE" ? `/${recordId}` : ""}`,
    body: payload,
    headers: { "Content-Type": "application/json" },
    createdAt: now,
    version: 1,
  });
  window.dispatchEvent(
    new CustomEvent("pwams:offline-mutation", {
      detail: { entityType, operation, recordId, record },
    }),
  );
  return recordId;
}

export function offlineSuccessMessage(
  entityType: OfflineEntityType,
  operation: "CREATE" | "UPDATE",
): string {
  const names: Record<OfflineEntityType, string> = {
    person: "Care seeker",
    student: "Student",
    donor: "Donor",
    aid_request: "Aid request",
    care_provided: "Care record",
    loan: "Loan",
    loan_repayment: "Loan repayment",
    donation: "Donation",
    revenue: "Revenue record",
    media: "Media file",
  };
  return `${names[entityType]} ${operation === "CREATE" ? "saved" : "updated"} offline. It is queued for later synchronization.`;
}
