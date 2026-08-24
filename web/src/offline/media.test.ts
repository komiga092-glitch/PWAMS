import {
  buildMediaQueueEntry,
  mediaRetryStatus,
  MAX_LOCAL_MEDIA_SIZE,
  validateMediaSize,
} from "./media.js";

validateMediaSize(MAX_LOCAL_MEDIA_SIZE);

try {
  validateMediaSize(MAX_LOCAL_MEDIA_SIZE + 1);
  throw new Error("Oversized media should be rejected");
} catch (error) {
  if (!(error instanceof Error) || !error.message.includes("2 MB")) throw error;
}

const entry = buildMediaQueueEntry(
  {
    originalName: "photo.jpg",
    contentType: "image/jpeg",
    size: 10,
    blob: new Blob(["data"], { type: "image/jpeg" }),
  },
  "media-operation-id",
);
if (
  entry.status !== "PENDING" ||
  entry.entityType !== "media" ||
  entry.recordId !== "media-operation-id"
) {
  throw new Error("Media queue entry metadata is incomplete");
}
if (
  mediaRetryStatus(201) !== "SYNCED" ||
  mediaRetryStatus(422) !== "FAILED" ||
  mediaRetryStatus(503) !== "PENDING"
) {
  throw new Error("Media retry classification is incorrect");
}
