import {
  clearMutationBody,
  enqueueOfflineRequest,
  getPendingMediaMutations,
  updateMutationStatus,
  createOfflineId,
  type OutboxEntry,
} from "./db.js";
import { isOnline } from "./connectivity.js";
import { isOfflineSessionValid, revalidateOnlineSession } from "./session.js";
import { getCsrfToken } from "./csrf.js";

export const MAX_LOCAL_MEDIA_SIZE = 2 * 1024 * 1024;
const SUPPORTED_MEDIA_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/webp",
  "application/pdf",
]);

export interface MediaQueueMetadata {
  originalName: string;
  contentType: string;
  size: number;
  relatedEntityType?: string;
  relatedRecordId?: string;
}

interface MediaPayload extends MediaQueueMetadata {
  blob: Blob;
}

export function buildMediaQueueEntry(
  payload: MediaPayload,
  recordId: string,
): OutboxEntry<MediaPayload> {
  return {
    entityType: "media",
    operationId: recordId,
    operation: "CREATE",
    recordId,
    status: "PENDING",
    method: "POST",
    url: "/files/upload",
    body: payload,
    headers: { "Idempotency-Key": recordId },
    createdAt: new Date().toISOString(),
  };
}

export function mediaRetryStatus(
  statusCode: number,
): "SYNCED" | "FAILED" | "PENDING" {
  if (statusCode >= 200 && statusCode < 300) return "SYNCED";
  if (statusCode >= 400 && statusCode < 500) return "FAILED";
  return "PENDING";
}

export function validateMediaSize(size: number): void {
  if (!Number.isFinite(size) || size <= 0) throw new Error("File is empty");
  if (size > MAX_LOCAL_MEDIA_SIZE)
    throw new Error("File size must not exceed 2 MB");
}

async function compressImage(file: File): Promise<Blob> {
  const objectURL = URL.createObjectURL(file);
  try {
    const image = new Image();
    image.src = objectURL;
    await image.decode();
    const scale = Math.min(
      1,
      1920 / Math.max(image.naturalWidth, image.naturalHeight),
    );
    const canvas = document.createElement("canvas");
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
    canvas
      .getContext("2d")
      ?.drawImage(image, 0, 0, canvas.width, canvas.height);
    const compressed = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, file.type, 0.8),
    );
    if (!compressed) throw new Error("Unable to compress image");
    return compressed;
  } finally {
    URL.revokeObjectURL(objectURL);
  }
}

export async function prepareMedia(
  file: File,
): Promise<{ blob: Blob; metadata: MediaQueueMetadata }> {
  if (!SUPPORTED_MEDIA_TYPES.has(file.type))
    throw new Error("Unsupported file type");
  const blob = file.type.startsWith("image/")
    ? await compressImage(file)
    : file;
  validateMediaSize(blob.size);
  return {
    blob,
    metadata: {
      originalName: file.name,
      contentType: blob.type || file.type,
      size: blob.size,
    },
  };
}

export async function queueMediaUpload(
  file: File,
  related?: Pick<MediaQueueMetadata, "relatedEntityType" | "relatedRecordId">,
): Promise<number> {
  if (navigator.onLine)
    throw new Error("Media should use the online upload flow while connected");
  if (!(await isOfflineSessionValid()))
    throw new Error("Offline session has expired. Reconnect to authenticate.");
  const prepared = await prepareMedia(file);
  const recordId = createOfflineId();
  const entry = buildMediaQueueEntry(
    { ...prepared.metadata, ...related, blob: prepared.blob },
    recordId,
  );
  const id = await enqueueOfflineRequest(entry);
  window.dispatchEvent(
    new CustomEvent("pwams:offline-mutation", {
      detail: { entityType: "media", operation: "CREATE", recordId },
    }),
  );
  return id;
}

export async function syncPendingMediaUploads(): Promise<void> {
  if (!isOnline() || !(await revalidateOnlineSession())) return;
  const entries = await getPendingMediaMutations();
  for (const entry of entries) {
    if (entry.id === undefined || !entry.body) continue;
    const payload = entry.body as MediaPayload;
    await updateMutationStatus(entry.id, "UPLOADING");
    const formData = new FormData();
    formData.append("file", payload.blob, payload.originalName);
    try {
      const response = await fetch(entry.url, {
        method: "POST",
        credentials: "include",
        headers: {
          "Idempotency-Key": entry.operationId,
          "X-CSRF-Token": getCsrfToken(),
        },
        body: formData,
      });
      const resultStatus = mediaRetryStatus(response.status);
      if (resultStatus === "SYNCED") {
        await updateMutationStatus(entry.id, "SYNCED");
        await clearMutationBody(entry.id);
      } else if (resultStatus === "FAILED") {
        const result = (await response.json().catch(() => null)) as {
          message?: string;
        } | null;
        await updateMutationStatus(
          entry.id,
          "FAILED",
          result?.message ?? "Media upload failed",
        );
      } else {
        await updateMutationStatus(
          entry.id,
          "PENDING",
          "Temporary media upload failure",
          true,
        );
        break;
      }
    } catch {
      await updateMutationStatus(
        entry.id,
        "PENDING",
        "Network unavailable",
        true,
      );
      break;
    }
  }
}
