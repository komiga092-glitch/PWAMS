import { listOfflineRecords, type OutboxEntry } from "./db.js";

export type ConflictEntry = OutboxEntry & { status: "CONFLICT" };

const ENTITY_LABELS: Record<ConflictEntry["entityType"], string> = {
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

export function getConflictMessage(conflict: ConflictEntry): string {
  const entity = ENTITY_LABELS[conflict.entityType] ?? conflict.entityType;
  const operation = conflict.operation.toLowerCase();
  return `${entity} ${conflict.recordId} could not be ${operation}d because the server record changed. ${conflict.error ?? "Review this change before trying again."}`;
}

export async function listConflictMutations(): Promise<ConflictEntry[]> {
  const entries = await listOfflineRecords<OutboxEntry>("outbox");
  return entries.filter(
    (entry): entry is ConflictEntry => entry.status === "CONFLICT",
  );
}

function escapeHTML(value: string): string {
  const element = document.createElement("div");
  element.textContent = value;
  return element.innerHTML;
}

export async function renderConflictStatus(): Promise<void> {
  const container = document.querySelector<HTMLElement>(
    "[data-sync-conflicts]",
  );
  if (!container) return;

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
        .map(
          (conflict) => `<li>${escapeHTML(getConflictMessage(conflict))}</li>`,
        )
        .join("")}
    </ul>
  `;
}
