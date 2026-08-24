import {
  isOfflineSessionTimestampValid,
  OFFLINE_SESSION_WINDOW_MS,
} from "./session.js";

const authenticatedAt = 1_000_000;

if (!isOfflineSessionTimestampValid(authenticatedAt, authenticatedAt)) {
  throw new Error("A session must be valid at its authentication time");
}

if (
  !isOfflineSessionTimestampValid(
    authenticatedAt,
    authenticatedAt + OFFLINE_SESSION_WINDOW_MS - 1,
  )
) {
  throw new Error("A session must remain valid before the 48-hour boundary");
}

if (
  isOfflineSessionTimestampValid(
    authenticatedAt,
    authenticatedAt + OFFLINE_SESSION_WINDOW_MS,
  )
) {
  throw new Error("A session must expire at the 48-hour boundary");
}

if (isOfflineSessionTimestampValid(null, authenticatedAt)) {
  throw new Error("A session without authentication metadata must be invalid");
}
