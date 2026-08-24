import {
  getSyncCursor,
  mergePulledRecords,
  type SyncPullRecord,
} from "./db.js";
import { isOnline } from "./connectivity.js";

const PULL_ENDPOINT = "/api/v1/sync/pull";
const PULL_LIMIT = 500;
let pullPromise: Promise<void> | undefined;

interface PullResponse {
  success: boolean;
  cursor?: string;
  next_cursor?: string;
  has_more: boolean;
  records: SyncPullRecord[];
}

export function pullSync(): Promise<void> {
  if (pullPromise) return pullPromise;
  pullPromise = runPullSync().finally(() => {
    pullPromise = undefined;
  });
  return pullPromise;
}

async function runPullSync(): Promise<void> {
  if (!isOnline()) return;

  let cursor = await getSyncCursor();
  let hasMore = true;

  while (hasMore) {
    const query = new URLSearchParams({ limit: String(PULL_LIMIT) });
    if (cursor) query.set("cursor", cursor);

    let response: Response;
    try {
      response = await fetch(`${PULL_ENDPOINT}?${query.toString()}`, {
        method: "GET",
        credentials: "include",
        headers: { Accept: "application/json" },
      });
    } catch (error) {
      console.error("PWAMS pull sync network failure", error);
      return;
    }

    if (response.status === 401 || response.status === 403) {
      console.warn("PWAMS pull sync authentication failed");
      return;
    }
    if (!response.ok) {
      console.error("PWAMS pull sync failed", response.status);
      return;
    }

    const result = (await response.json()) as PullResponse;
    const nextCursor = result.next_cursor ?? result.cursor;
    if (!nextCursor) return;

    try {
      await mergePulledRecords(result.records ?? [], nextCursor);
    } catch (error) {
      console.error("PWAMS pull sync merge failed", error);
      return;
    }

    cursor = nextCursor;
    hasMore = result.has_more;
  }
}
