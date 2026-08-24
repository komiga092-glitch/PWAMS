import { listOfflineRecords } from "./db.js";
const ENTITY_LABELS = {
    person: "Care seeker",
    student: "Student",
    donor: "Donor",
    aid_request: "Aid request",
    care_provided: "Care record",
    loan: "Loan",
    loan_repayment: "Loan repayment",
};
export function getConflictMessage(conflict) {
    const entity = ENTITY_LABELS[conflict.entityType] ?? conflict.entityType;
    const operation = conflict.operation.toLowerCase();
    return `${entity} ${conflict.recordId} could not be ${operation}d because the server record changed. ${conflict.error ?? "Review this change before trying again."}`;
}
export async function listConflictMutations() {
    const entries = await listOfflineRecords("outbox");
    return entries.filter((entry) => entry.status === "CONFLICT");
}
function escapeHTML(value) {
    const element = document.createElement("div");
    element.textContent = value;
    return element.innerHTML;
}
export async function renderConflictStatus() {
    const container = document.querySelector("[data-sync-conflicts]");
    if (!container)
        return;
    const conflicts = await listConflictMutations();
    container.hidden = conflicts.length === 0;
    if (conflicts.length === 0) {
        container.replaceChildren();
        return;
    }
    container.innerHTML = `
    <strong>Synchronization conflicts (${conflicts.length})</strong>
    <ul>
      ${conflicts
        .map((conflict) => `<li>${escapeHTML(getConflictMessage(conflict))}</li>`)
        .join("")}
    </ul>
  `;
}
//# sourceMappingURL=conflicts.js.map