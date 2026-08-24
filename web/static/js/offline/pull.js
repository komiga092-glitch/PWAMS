import { getSyncCursor, mergePulledRecords, } from "./db.js";
import { isOnline } from "./connectivity.js";
const PULL_ENDPOINT = "/api/v1/sync/pull";
const PULL_LIMIT = 500;
let pullPromise;
export function pullSync() {
    if (pullPromise)
        return pullPromise;
    pullPromise = runPullSync().finally(() => {
        pullPromise = undefined;
    });
    return pullPromise;
}
async function runPullSync() {
    if (!isOnline())
        return;
    let cursor = await getSyncCursor();
    let hasMore = true;
    while (hasMore) {
        const query = new URLSearchParams({ limit: String(PULL_LIMIT) });
        if (cursor)
            query.set("cursor", cursor);
        let response;
        try {
            response = await fetch(`${PULL_ENDPOINT}?${query.toString()}`, {
                method: "GET",
                credentials: "include",
                headers: { Accept: "application/json" },
            });
        }
        catch (error) {
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
        const result = (await response.json());
        const nextCursor = result.next_cursor ?? result.cursor;
        if (!nextCursor)
            return;
        try {
            await mergePulledRecords(result.records ?? [], nextCursor);
        }
        catch (error) {
            console.error("PWAMS pull sync merge failed", error);
            return;
        }
        cursor = nextCursor;
        hasMore = result.has_more;
    }
}
//# sourceMappingURL=pull.js.map