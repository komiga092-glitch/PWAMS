import { createOfflineId, saveOfflineMutation, } from "./db.js";
const STORE_BY_ENTITY = {
    person: "persons",
    student: "students",
    donor: "donors",
    aid_request: "aid_requests",
    care_provided: "care_provided",
    loan: "loans",
    loan_repayment: "loan_repayments",
};
const ENDPOINT_BY_ENTITY = {
    person: "persons",
    student: "students",
    donor: "donors",
    aid_request: "aid-requests",
    care_provided: "care-provided",
    loan: "loans",
    loan_repayment: "loan-repayments",
};
export function offlineMutationsEnabled() {
    return !navigator.onLine;
}
export async function savePendingMutation(entityType, operation, payload, recordId = typeof payload.id === "string" ? payload.id : createOfflineId()) {
    const now = new Date().toISOString();
    const record = { ...payload, id: recordId, updatedAt: now };
    await saveOfflineMutation(STORE_BY_ENTITY[entityType], record, {
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
    window.dispatchEvent(new CustomEvent("pwams:offline-mutation", {
        detail: { entityType, operation, recordId, record },
    }));
    return recordId;
}
export function offlineSuccessMessage(entityType, operation) {
    const names = {
        person: "Care seeker",
        student: "Student",
        donor: "Donor",
        aid_request: "Aid request",
        care_provided: "Care record",
        loan: "Loan",
        loan_repayment: "Loan repayment",
    };
    return `${names[entityType]} ${operation === "CREATE" ? "saved" : "updated"} offline. It is queued for later synchronization.`;
}
//# sourceMappingURL=mutations.js.map