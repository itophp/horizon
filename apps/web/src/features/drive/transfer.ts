import {
  driveFileChunk,
  driveUploadChunk,
  driveUploadComplete,
  driveUploadStart,
  driveUploadStatus,
} from "@/lib/capabilities";

const chunkSize = 4 * 1024 * 1024;
const fallbackLimit = 64 * 1024 * 1024;

function base64(bytes: Uint8Array<ArrayBuffer>): string {
  let binary = "";
  for (let start = 0; start < bytes.length; start += 32768) {
    binary += String.fromCharCode(...bytes.subarray(start, start + 32768));
  }
  return btoa(binary);
}

function decode(content: string): Uint8Array<ArrayBuffer> {
  const binary = atob(content);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

async function digest(bytes: Uint8Array<ArrayBuffer>): Promise<string> {
  const hash = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(hash), (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
}

export type UploadProgress = {
  uploadId: number;
  uploaded: number;
  total: number;
};

/** Resume by selecting the same file and supplying its existing upload ID. */
export async function uploadDriveFile(
  file: File,
  spaceId: number,
  folderId: number,
  resumeId: number | undefined,
  onProgress: (progress: UploadProgress) => void,
): Promise<number> {
  const uploadId =
    resumeId ??
    (
      await driveUploadStart(
        spaceId,
        folderId,
        file.name,
        file.size,
        crypto.randomUUID(),
      )
    ).data.id;
  onProgress({ uploadId, uploaded: 0, total: file.size });
  const state = (await driveUploadStatus(uploadId)).data;
  if (
    state.spaceId !== spaceId ||
    state.folderId !== folderId ||
    state.name !== file.name ||
    state.size !== file.size
  ) {
    throw new Error("upload_mismatch");
  }
  if (
    (state.state === "completed" || state.state === "cleaned") &&
    state.completedFileId > 0
  ) {
    return state.completedFileId;
  }
  if (state.state !== "open") throw new Error("upload_mismatch");
  const completed = new Map(
    state.completedChunks.map((part) => [part.index, part.sha256]),
  );
  let uploaded = 0;
  onProgress({ uploadId, uploaded, total: file.size });
  for (
    let offset = 0, index = 0;
    offset < file.size;
    offset += chunkSize, index++
  ) {
    const part = file.slice(offset, Math.min(offset + chunkSize, file.size));
    const bytes = new Uint8Array(await part.arrayBuffer());
    const hash = await digest(bytes);
    if (completed.has(index)) {
      if (completed.get(index) !== hash) throw new Error("upload_mismatch");
    } else {
      await driveUploadChunk(
        spaceId,
        uploadId,
        index,
        hash,
        base64(bytes),
        crypto.randomUUID(),
      );
    }
    uploaded += part.size;
    onProgress({ uploadId, uploaded, total: file.size });
  }
  return (await driveUploadComplete(spaceId, uploadId, crypto.randomUUID()))
    .data.id;
}

type SaveHandle = {
  createWritable(): Promise<{
    write(data: Uint8Array<ArrayBuffer>): Promise<void>;
    close(): Promise<void>;
    abort(): Promise<void>;
  }>;
};
type PickerWindow = Window & {
  showSaveFilePicker?: (options: {
    suggestedName: string;
  }) => Promise<SaveHandle>;
};

async function copyFileChunks(
  spaceId: number,
  file: { id: number; size: number },
  write: (bytes: Uint8Array<ArrayBuffer>) => Promise<void>,
): Promise<void> {
  let offset = 0;
  do {
    const part = (await driveFileChunk(spaceId, file.id, offset)).data;
    const bytes = decode(part.content);
    const expected = Math.min(chunkSize, file.size - offset);
    if (
      part.size !== file.size ||
      part.offset !== offset ||
      bytes.length !== expected ||
      part.eof !== (offset + expected === file.size)
    ) {
      throw new Error("download_incomplete");
    }
    await write(bytes);
    offset += bytes.length;
  } while (offset < file.size);
}

/** Uses a native writable stream for large files; the fallback is bounded. */
export async function downloadDriveFile(
  spaceId: number,
  file: { id: number; name: string; size: number },
): Promise<void> {
  const picker = (window as PickerWindow).showSaveFilePicker;
  if (picker) {
    const handle = await picker({ suggestedName: file.name });
    const writable = await handle.createWritable();
    try {
      await copyFileChunks(spaceId, file, (bytes) => writable.write(bytes));
      await writable.close();
    } catch (error) {
      await writable.abort();
      throw error;
    }
    return;
  }
  if (file.size > fallbackLimit)
    throw new Error("streaming_download_unavailable");
  const parts: BlobPart[] = [];
  await copyFileChunks(spaceId, file, async (bytes) => {
    parts.push(bytes);
  });
  const url = URL.createObjectURL(
    new Blob(parts, { type: "application/octet-stream" }),
  );
  try {
    const link = document.createElement("a");
    link.href = url;
    link.download = file.name;
    link.click();
  } finally {
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
  }
}
