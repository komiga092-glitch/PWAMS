declare module "/static/js/offline/mutations.js" {
  export function savePendingMutation<T extends Record<string, unknown>>(
    entityType:
      | "person"
      | "student"
      | "donor"
      | "aid_request"
      | "care_provided"
      | "loan"
      | "loan_repayment",
    operation: "CREATE" | "UPDATE",
    payload: T,
    recordId?: string,
  ): Promise<string>;

  export function offlineSuccessMessage(
    entityType:
      | "person"
      | "student"
      | "donor"
      | "aid_request"
      | "care_provided"
      | "loan"
      | "loan_repayment",
    operation: "CREATE" | "UPDATE",
  ): string;
}
