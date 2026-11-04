import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { downloadDriveFile, uploadDriveFile } from "./transfer";

const api = vi.hoisted(() => ({
  start: vi.fn(),
  status: vi.fn(),
  chunk: vi.fn(),
  complete: vi.fn(),
  read: vi.fn(),
}));
vi.mock("@/lib/capabilities", () => ({
  driveUploadStart: api.start,
  driveUploadStatus: api.status,
  driveUploadChunk: api.chunk,
  driveUploadComplete: api.complete,
  driveFileChunk: api.read,
}));

const file = {
  name: "data.bin",
  size: 4,
  slice: () => ({
    size: 4,
    arrayBuffer: async () => new Uint8Array([1, 2, 3, 4]).buffer,
  }),
} as unknown as File;

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("crypto", {
    randomUUID: vi
      .fn()
      .mockReturnValueOnce("chunk-attempt")
      .mockReturnValueOnce("complete-attempt"),
    subtle: {
      digest: vi.fn().mockResolvedValue(new Uint8Array(32).fill(1).buffer),
    },
  });
  api.status.mockResolvedValue({
    data: {
      id: 5,
      spaceId: 2,
      folderId: 0,
      name: "data.bin",
      size: 4,
      state: "open",
      completedChunks: [],
    },
  });
  api.chunk.mockResolvedValue({ data: { index: 0 } });
  api.complete.mockResolvedValue({ data: { id: 9 } });
});
afterEach(() => vi.unstubAllGlobals());

it("rejects a resumed file whose completed chunk has different content", async () => {
  api.status.mockResolvedValue({
    data: {
      id: 5,
      spaceId: 2,
      folderId: 0,
      name: "data.bin",
      size: 4,
      state: "open",
      completedChunks: [{ index: 0, sha256: "00".repeat(32) }],
    },
  });
  await expect(uploadDriveFile(file, 2, 0, 5, () => undefined)).rejects.toThrow(
    "upload_mismatch",
  );
  expect(api.chunk).not.toHaveBeenCalled();
  expect(api.complete).not.toHaveBeenCalled();
});

it("uploads a chunk and completes using separate attempt keys", async () => {
  const progress = vi.fn();
  await expect(uploadDriveFile(file, 2, 0, 5, progress)).resolves.toBe(9);
  expect(api.chunk).toHaveBeenCalledWith(
    2,
    5,
    0,
    "01".repeat(32),
    "AQIDBA==",
    "chunk-attempt",
  );
  expect(api.complete).toHaveBeenCalledWith(2, 5, "complete-attempt");
  expect(progress).toHaveBeenLastCalledWith({
    uploadId: 5,
    uploaded: 4,
    total: 4,
  });
});

it("returns the completed file when the completion response was lost", async () => {
  api.status.mockResolvedValue({
    data: {
      id: 5,
      spaceId: 2,
      folderId: 0,
      name: "data.bin",
      size: 4,
      state: "cleaned",
      completedFileId: 9,
      completedChunks: [],
    },
  });
  await expect(uploadDriveFile(file, 2, 0, 5, () => undefined)).resolves.toBe(
    9,
  );
  expect(api.complete).not.toHaveBeenCalled();
});

it("checks authorization even when downloading an empty file", async () => {
  api.read.mockRejectedValue(new Error("denied"));
  await expect(
    downloadDriveFile(2, { id: 9, name: "empty", size: 0 }),
  ).rejects.toThrow("denied");
  expect(api.read).toHaveBeenCalledWith(2, 9, 0);
});
