export type OfflineReadStore =
  | "persons"
  | "students"
  | "donors"
  | "donations"
  | "aid_requests"
  | "loans"
  | "loan_repayments"
  | "care_provided";

export async function readOfflineStore<T>(
  store: OfflineReadStore,
): Promise<T[]> {
  const database = await import(String("/static/js/offline/db.js"));
  const listOfflineRecords = database.listOfflineRecords as <R>(
    name: string,
  ) => Promise<R[]>;
  return listOfflineRecords<T>(store);
}
