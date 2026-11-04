// Capability SDK for the console. Every write and controlled read goes through
// the gateway: the functions below only shape arguments and unwrap the uniform
// result, so a page button and an agent tool run the same code path.
import { capabilityInvoke } from "@/api/generated/baseline";
import { ApiError } from "@/api/client/request";
import type {
  ChangeEntry,
  ChangeRecord,
  DictionaryDetail,
  DictionaryOption,
  DictionaryWorkspace,
  DeliveryRecord,
  ImportPreview,
  InboxMessage,
  MCPAudit,
  MCPToken,
  MCPTokenCreated,
  ManagedFile,
  NotificationAudience,
  SentNotification,
  SettingSchedule,
  SettingsWorkspace,
  StorageDriver,
  ReauthProof,
} from "@/api/types";
import type {
  CapabilityResult,
  EffectiveConfiguration,
  FileContent,
} from "@/api/generated/models";

export type KnowledgeDocument = {
  id: number;
  title: string;
  source: string;
  chunks: number;
  createdAt: string;
};

export type CapabilityResponse<T> = {
  status: number;
  data: T;
  headers: Headers;
};

const emptyHeaders = new Headers();

function identifier(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `cap-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

/**
 * invoke runs one capability and unwraps its business result. A denial or a
 * validation problem is thrown as ApiError so callers keep their existing error
 * handling; a business failure is never reported as success.
 */
async function invoke<T>(
  name: string,
  input: Record<string, unknown> = {},
  idempotencyKey?: string,
): Promise<CapabilityResponse<T>> {
  const response = await capabilityInvoke(name, {
    input,
    idempotencyKey: idempotencyKey ?? identifier(),
  });
  const result = response.data as CapabilityResult;
  if (result.status === "awaiting_approval") {
    throw new ApiError("approval_required", 409);
  }
  if (result.status !== "succeeded") {
    throw new ApiError(result.errorCode ?? "internal", 400);
  }
  return {
    status: 200,
    data: result.data as T,
    headers: response.headers ?? emptyHeaders,
  };
}

// ---------------------------------------------------------------- cloud drive

export type DriveSpace = {
  id: number;
  kind: "personal" | "shared";
  name: string;
  role: "admin" | "editor" | "viewer";
};
export type DriveFolder = { id: number; name: string; parentId: number };
export type DriveFile = {
  id: number;
  name: string;
  folderId: number;
  size: number;
  sha256: string;
  createdAt: string;
};
export type DriveMember = {
  id: number;
  targetType: "account" | "department";
  targetId: number;
  role: "admin" | "editor" | "viewer";
};
export type DriveTrashItem = {
  id: number;
  kind: "folder" | "file";
  folderId: number;
  fileId: number;
  name: string;
  parentId: number;
  deletedAt: string;
  expiresAt: string;
};
export type DriveUploadStatus = {
  id: number;
  spaceId: number;
  folderId: number;
  name: string;
  size: number;
  state: string;
  expiresAt: string;
  completedFileId: number;
  completedChunks: { index: number; sha256: string }[];
};
export type DriveFileChunk = {
  name: string;
  size: number;
  sha256: string;
  offset: number;
  content: string;
  eof: boolean;
};

export const drivePersonalEnsure = () =>
  invoke<{ id: number }>("drive.space.personal.ensure");
export const driveSpaces = () =>
  invoke<{ items: DriveSpace[] }>("drive.space.list");
export const driveSpaceUsage = (spaceId: number) =>
  invoke<{ usedBytes: number; pendingBytes: number; quotaBytes: number }>(
    "drive.space.usage",
    { spaceId },
  );
export const driveSpaceCreate = (name: string) =>
  invoke<{ id: number }>("drive.space.create", { name });
export const driveFolders = (spaceId: number, parentId: number) =>
  invoke<{ items: DriveFolder[] }>("drive.folder.list", { spaceId, parentId });
export const driveFolderPath = (spaceId: number, folderId: number) =>
  invoke<{ items: DriveFolder[] }>("drive.folder.path", { spaceId, folderId });
export const driveFolderCreate = (
  spaceId: number,
  parentId: number,
  name: string,
) => invoke<{ id: number }>("drive.folder.create", { spaceId, parentId, name });
export const driveFolderRename = (spaceId: number, id: number, name: string) =>
  invoke<{ id: number }>("drive.folder.rename", { spaceId, id, name });
export const driveFolderMove = (
  spaceId: number,
  id: number,
  parentId: number,
) => invoke<{ id: number }>("drive.folder.move", { spaceId, id, parentId });
export const driveFolderTrash = (spaceId: number, folderId: number) =>
  invoke<{ id: number }>("drive.folder.trash", { spaceId, folderId });
export const driveFiles = (spaceId: number, folderId: number) =>
  invoke<{ items: DriveFile[] }>("drive.file.list", { spaceId, folderId });
export const driveFileRename = (
  spaceId: number,
  fileId: number,
  name: string,
) => invoke<{ id: number }>("drive.file.rename", { spaceId, fileId, name });
export const driveFileMove = (
  spaceId: number,
  fileId: number,
  folderId: number,
) => invoke<{ id: number }>("drive.file.move", { spaceId, fileId, folderId });
export const driveFileTrash = (spaceId: number, fileId: number) =>
  invoke<{ id: number }>("drive.file.trash", { spaceId, fileId });
export const driveMembers = (spaceId: number) =>
  invoke<{ items: DriveMember[] }>("drive.member.list", { spaceId });
export const driveMemberAdd = (
  spaceId: number,
  targetType: DriveMember["targetType"],
  targetId: number,
  role: DriveMember["role"],
) =>
  invoke<{ id: number }>("drive.member.add", {
    spaceId,
    targetType,
    targetId,
    role,
  });
export const driveMemberChange = (
  spaceId: number,
  memberId: number,
  role: DriveMember["role"],
) => invoke<{ id: number }>("drive.member.change", { spaceId, memberId, role });
export const driveMemberRemove = (spaceId: number, memberId: number) =>
  invoke<{ id: number }>("drive.member.remove", { spaceId, memberId });
export const driveTrash = (spaceId: number) =>
  invoke<{ items: DriveTrashItem[] }>("drive.trash.list", { spaceId });
export const driveFolderRestore = (
  spaceId: number,
  batchId: number,
  parentId: number,
  name: string,
) =>
  invoke<{ id: number }>("drive.trash.restore", {
    spaceId,
    batchId,
    parentId,
    name,
  });
export const driveFileRestore = (
  spaceId: number,
  batchId: number,
  folderId: number,
  name: string,
) =>
  invoke<{ id: number }>("drive.trash.file.restore", {
    spaceId,
    batchId,
    folderId,
    name,
  });
export const driveTrashPurge = (spaceId: number, batchId: number) =>
  invoke<{ id: number }>("drive.trash.purge.request", { spaceId, batchId });
export const driveUploadStart = (
  spaceId: number,
  folderId: number,
  name: string,
  size: number,
  key: string,
) =>
  invoke<{ id: number; chunkSize: number }>(
    "drive.upload.start",
    { spaceId, folderId, name, size },
    key,
  );
export const driveUploadStatus = (id: number) =>
  invoke<DriveUploadStatus>("drive.upload.status", { id });
export const driveUploadChunk = (
  spaceId: number,
  uploadId: number,
  index: number,
  sha256: string,
  content: string,
  key: string,
) =>
  invoke<{ index: number }>(
    "drive.upload.chunk",
    { spaceId, uploadId, index, sha256, content },
    key,
  );
export const driveUploadComplete = (
  spaceId: number,
  uploadId: number,
  key: string,
) =>
  invoke<{ id: number }>("drive.upload.complete", { spaceId, uploadId }, key);
export const driveUploadCancel = (spaceId: number, uploadId: number) =>
  invoke<{ id: number }>("drive.upload.cancel", { spaceId, uploadId });
export const driveFileChunk = (
  spaceId: number,
  fileId: number,
  offset: number,
) =>
  invoke<DriveFileChunk>("drive.file.chunk.read", { spaceId, fileId, offset });

function body(value: unknown): Record<string, unknown> {
  return (value ?? {}) as Record<string, unknown>;
}

// ---------------------------------------------------------------- settings

type Target = { targetKind?: string; targetId?: number };

export const settingsWorkspace = (params?: Target) =>
  invoke<SettingsWorkspace>("settings.workspace.read", { ...params });

export const settingsEffective = () =>
  invoke<EffectiveConfiguration>("settings.effective.read");

export const settingsPreferences = (value: unknown) =>
  invoke<EffectiveConfiguration>("settings.preferences.update", body(value));

export const settingsDraft = (value: unknown) =>
  invoke<ChangeRecord>("settings.draft.create", body(value));

export const settingsPublish = (id: number, value: unknown) =>
  invoke<ChangeRecord>("settings.draft.publish", { id, ...body(value) });

export const settingsRollback = (id: number, value: unknown) =>
  invoke<ChangeRecord>("settings.history.rollback", { id, ...body(value) });

export const settingsSchedules = () =>
  invoke<SettingSchedule[]>("settings.schedule.list");

export const settingsSchedule = (id: number, value: unknown) =>
  invoke<SettingSchedule>("settings.schedule.create", { id, ...body(value) });

export const settingsCancelSchedule = (id: number) =>
  invoke<{ id: number }>("settings.schedule.cancel", { id });

export const settingsExport = (params?: Target) =>
  invoke<ChangeEntry[]>("settings.export.read", { ...params });

export const settingsLogo = () => invoke<FileContent>("settings.logo.read");

export const settingsReauthenticate = (value: unknown) =>
  invoke<ReauthProof>("settings.password.verify", body(value));

// ------------------------------------------------------------ dictionaries

type DictionaryQuery = Target;

export const dictionariesList = (params?: DictionaryQuery) =>
  invoke<DictionaryWorkspace>("dictionary.list.read", { ...params });

export const dictionariesGet = (id: number, params?: DictionaryQuery) =>
  invoke<DictionaryDetail>("dictionary.detail.read", { id, ...params });

export const dictionariesOptions = (
  code: string,
  params?: { language?: string; ids?: number[]; search?: string },
) => invoke<DictionaryOption[]>("dictionary.options.read", { code, ...params });

export const dictionariesDraft = (value: unknown) =>
  invoke<ChangeRecord>("dictionary.draft.create", body(value));

export const dictionariesPublish = (id: number, value: unknown) =>
  invoke<ChangeRecord>("dictionary.draft.publish", { id, ...body(value) });

export const dictionariesRollback = (id: number, value: unknown) =>
  invoke<ChangeRecord>("dictionary.history.rollback", { id, ...body(value) });

export const dictionariesRemove = (id: number) =>
  invoke<{ id: number }>("dictionary.delete", { id });

export const dictionariesOverride = (id: number, value: unknown) =>
  invoke<ChangeRecord>("dictionary.override.save", { id, ...body(value) });

export const dictionariesImport = (id: number, value: unknown) =>
  invoke<ImportPreview>("dictionary.import.preview", { id, ...body(value) });

export const dictionariesImportDraft = (id: number, value: unknown) =>
  invoke<ChangeRecord>("dictionary.import.draft", { id, ...body(value) });

export const dictionariesExport = (id: number, params?: { format?: string }) =>
  invoke<{ filename: string; content: string }>("dictionary.export.read", {
    id,
    ...params,
  });

// ------------------------------------------------------------------- files

export const filesList = () => invoke<ManagedFile[]>("file.list.read");

export const filesDrivers = () => invoke<StorageDriver[]>("file.drivers.read");

export const filesUpload = (value: unknown) =>
  invoke<ManagedFile>("file.upload", body(value));

export const filesContent = (id: number) =>
  invoke<FileContent>("file.content.read", { id });

export const filesRemove = (id: number) =>
  invoke<{ id: number }>("file.delete", { id });

// ----------------------------------------------------------- notifications

export const notificationsAudience = () =>
  invoke<NotificationAudience[]>("notification.audience.read");

export const notificationsPreview = (value: unknown) =>
  invoke<{ recipientCount: number }>("notification.preview.read", body(value));

export const notificationsSend = (value: unknown) =>
  invoke<SentNotification>("notification.send", body(value));

export const notificationsSent = () =>
  invoke<SentNotification[]>("notification.sent.read");

export const notificationsAttachment = (id: number, fileId: number) =>
  invoke<FileContent>("notification.attachment.read", { id, fileId });

export const notificationsMessage = (id: number) =>
  invoke<InboxMessage>("notification.message.read", { id });

export const notificationsInbox = () =>
  invoke<InboxMessage[]>("notification.inbox.read");

export const notificationsRead = (id: number) =>
  invoke<{ id: number }>("notification.inbox.mark_read", { id });

export const notificationsDeliveries = () =>
  invoke<DeliveryRecord[]>("notification.delivery.read");

export const notificationsRetry = (id: number) =>
  invoke<{ id: number }>("notification.delivery.retry", { id });

export const notificationsTest = (value: unknown) =>
  invoke<{ status: string }>("notification.smtp.test", body(value));

// --------------------------------------------------------------------- mcp

export const mcpManagementCreate = (value: unknown) =>
  invoke<MCPTokenCreated>("mcp.token.create", body(value));

export const mcpManagementList = () => invoke<MCPToken[]>("mcp.token.list");

export const mcpManagementRevoke = (id: number, value: unknown) =>
  invoke<{ id: number }>("mcp.token.revoke", { id, ...body(value) });

export const mcpManagementRotate = (id: number, value: unknown) =>
  invoke<MCPTokenCreated>("mcp.token.rotate", { id, ...body(value) });

export const mcpManagementAdminRevoke = (id: number, value: unknown) =>
  invoke<{ id: number }>("mcp.token.admin_revoke", { id, ...body(value) });

export const mcpManagementCapabilities = (params?: { departmentId?: number }) =>
  invoke<string[]>("mcp.capability.list", { ...params });

export const mcpManagementAudit = () => invoke<MCPAudit[]>("mcp.audit.list");

// ---------------------------------------------------------------- identity

export const identityCreateAccount = (value: unknown) =>
  invoke<Record<string, unknown>>("identity.account.create", body(value));

export const identityUpdateAccount = (id: number, value: unknown) =>
  invoke<Record<string, unknown>>("identity.account.update", {
    id,
    ...body(value),
  });

export const identityDeleteAccount = (id: number) =>
  invoke<{ id: number }>("identity.account.delete", { id });

export const identityCreateOrganization = (value: unknown) =>
  invoke<Record<string, unknown>>("identity.organization.create", body(value));

export const identityUpdateOrganization = (id: number, value: unknown) =>
  invoke<Record<string, unknown>>("identity.organization.update", {
    id,
    ...body(value),
  });

export const identityDeleteOrganization = (id: number) =>
  invoke<{ id: number }>("identity.organization.delete", { id });

export const identityCreateDepartment = (value: unknown) =>
  invoke<Record<string, unknown>>("identity.department.create", body(value));

export const identityUpdateDepartment = (id: number, value: unknown) =>
  invoke<Record<string, unknown>>("identity.department.update", {
    id,
    ...body(value),
  });

export const identityDeleteDepartment = (id: number) =>
  invoke<{ id: number }>("identity.department.delete", { id });

export const identityCreateRole = (value: unknown) =>
  invoke<Record<string, unknown>>("identity.role.create", body(value));

export const identityUpdateRole = (id: number, value: unknown) =>
  invoke<Record<string, unknown>>("identity.role.update", {
    id,
    ...body(value),
  });

export const identityDeleteRole = (id: number) =>
  invoke<{ id: number }>("identity.role.delete", { id });

export const capabilityKey = (name: string) => ["capability", name] as const;

// Knowledge base management. Retrieval stays a separate capability so the agent
// can search without being able to index or delete.
export const knowledgeList = () =>
  invoke<KnowledgeDocument[]>("knowledge.list");

export const knowledgeCreate = (input: { title: string; content: string }) =>
  invoke<KnowledgeDocument>("knowledge.create", input);

export const knowledgeDelete = (id: number) =>
  invoke<{ id: number }>("knowledge.delete", { id });

export const knowledgeReindex = () =>
  invoke<{ documents: number }>("knowledge.reindex");

// Keep the Gateway envelope for flows that can legitimately await approval.
export async function invokeCapability(
  name: string,
  input: Record<string, unknown>,
  idempotencyKey?: string,
): Promise<CapabilityResult> {
  const response = await capabilityInvoke(name, { input, idempotencyKey });
  if (response.status !== 200)
    throw new ApiError("unavailable", response.status);
  const result = response.data;
  if (result.status !== "succeeded" && result.status !== "awaiting_approval")
    throw new ApiError(result.errorCode || "internal", 400);
  return result;
}

export type Article = {
  id: number;
  title: string;
  status: string;
  revision: number;
  reviewCode: string;
  body?: string;
};
export const articlesList = () => invoke<{ items: Article[] }>("article.list");
export const articleGet = (id: number) =>
  invoke<Article>("article.get", { id });
export const articleSave = (value: {
  id?: number;
  revision?: number;
  title: string;
  body: string;
}) =>
  invoke<{ id: number }>(
    value.id ? "article.update" : "article.create",
    body(value),
  );
// Publishing is one durable flow: save into review, agent review, then exactly
// one of publish or reject. The page only starts the run and reads the state.
export const articlePublish = (id: number, revision: number) =>
  invoke<{ runId: number; status: string }>("orchestration.workflow.run", {
    flow: "article-publish",
    version: "1.0.0",
    input: { id, revision },
  });
export const articleDecide = (id: number, revision: number, approve: boolean) =>
  invoke<{ id: number }>("article.decide", { id, revision, approve });

// ---------------------------------------------------------------- projects
// Every call below is a capability: the page never composes SQL or bypasses the
// gateway, and membership is decided by the server, not by which buttons render.

export type Project = {
  id: number;
  code: string;
  name: string;
  status: string;
  priority: string;
  dueDate: string;
  ownerId: number;
  ownerName: string;
  planCount: number;
  taskCount: number;
  taskDoneCount: number;
  taskOverdueCount: number;
  createdAt: string;
  updatedAt: string;
};

export type ProjectMember = {
  accountId: number;
  name: string;
  email: string;
  isOwner: boolean;
};

export type ProjectAccount = {
  accountId: number;
  name: string;
  email: string;
};

export type ProjectPlan = {
  id: number;
  projectId: number;
  planType: string;
  title: string;
  content?: string;
  status: string;
  priority: string;
  dueDate: string;
  ownerId: number;
  ownerName: string;
};

export type ProjectTask = {
  id: number;
  projectId: number;
  planId: number;
  planTitle: string;
  title: string;
  description?: string;
  status: string;
  priority: string;
  assigneeId: number;
  assigneeName: string;
  dueDate: string;
  completedAt: string;
  brief?: string;
  briefModel?: string;
};

export type ProjectPlanInput = {
  planType: string;
  title: string;
  content: string;
  priority: string;
  dueDate: string;
  ownerId: number;
};

export type ProjectTaskInput = {
  planId: number;
  title: string;
  description: string;
  priority: string;
  assigneeId: number;
  dueDate: string;
};

export const projectsList = (params?: {
  status?: string;
  priority?: string;
  keyword?: string;
  limit?: number;
}) => invoke<{ items: Project[] }>("project.list", { ...params });

export const projectGet = (id: number) =>
  invoke<Project>("project.get", { id });

export const projectCreate = (value: {
  code: string;
  name: string;
  priority: string;
  dueDate: string;
}) => invoke<{ id: number }>("project.create", body(value));

export const projectUpdate = (value: {
  id: number;
  name: string;
  priority: string;
  dueDate: string;
}) => invoke<{ id: number }>("project.update", body(value));

export const projectStatusChange = (id: number, from: string, status: string) =>
  invoke<{ id: number; status: string }>("project.status.change", {
    id,
    from,
    status,
  });

export const projectMembers = (projectId: number) =>
  invoke<{ items: ProjectMember[] }>("project.member.list", { projectId });

// The directory search exists because a plain member has no accounts:read.
export const projectMemberSearch = (projectId: number, keyword: string) =>
  invoke<{ items: ProjectAccount[] }>("project.member.search", {
    projectId,
    keyword,
  });

export const projectMemberAdd = (projectId: number, accountId: number) =>
  invoke<{ projectId: number; accountId: number; added: boolean }>(
    "project.member.add",
    { projectId, accountId },
  );

export const projectMemberRemove = (projectId: number, accountId: number) =>
  invoke<{ projectId: number; accountId: number }>("project.member.remove", {
    projectId,
    accountId,
  });

export const projectOwnerTransfer = (projectId: number, accountId: number) =>
  invoke<{ projectId: number; ownerId: number }>("project.owner.transfer", {
    projectId,
    accountId,
  });

export const projectPlans = (projectId: number) =>
  invoke<{ items: ProjectPlan[] }>("project.plan.list", { projectId });

export const projectPlanGet = (id: number) =>
  invoke<ProjectPlan>("project.plan.get", { id });

export const projectPlanCreate = (
  value: ProjectPlanInput & { projectId: number },
) => invoke<{ id: number }>("project.plan.create", body(value));

export const projectPlanUpdate = (value: ProjectPlanInput & { id: number }) =>
  invoke<{ id: number }>("project.plan.update", body(value));

export const projectPlanStatusChange = (
  id: number,
  from: string,
  status: string,
) =>
  invoke<{ id: number; status: string }>("project.plan.status.change", {
    id,
    from,
    status,
  });

export const projectTasks = (
  projectId: number,
  params?: {
    planId?: number;
    assigneeId?: number;
    status?: string;
    limit?: number;
  },
) =>
  invoke<{ items: ProjectTask[] }>("project.task.list", {
    projectId,
    ...params,
  });

export const projectTaskGet = (id: number) =>
  invoke<ProjectTask>("project.task.get", { id });

export const projectTaskCreate = (
  value: ProjectTaskInput & { projectId: number },
) => invoke<{ id: number }>("project.task.create", body(value));

export const projectTaskUpdate = (value: ProjectTaskInput & { id: number }) =>
  invoke<{ id: number }>("project.task.update", body(value));

export const projectTaskStatusChange = (
  id: number,
  from: string,
  status: string,
) =>
  invoke<{ id: number; status: string; completedAt: string }>(
    "project.task.status.change",
    { id, from, status },
  );

export const projectTaskAssign = (id: number, assigneeId: number) =>
  invoke<{ id: number; assigneeId: number }>("project.task.assign", {
    id,
    assigneeId,
  });
