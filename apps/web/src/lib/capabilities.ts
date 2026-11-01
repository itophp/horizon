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
): Promise<CapabilityResponse<T>> {
  const response = await capabilityInvoke(name, {
    input,
    idempotencyKey: identifier(),
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
export const articleSubmit = (id: number, revision: number) =>
  invoke<{ id: number }>("article.submit", { id, revision });
export const articleAssess = (id: number) =>
  invoke<{ id: number }>("article.assess", { id });
export const articleDecide = (id: number, revision: number, approve: boolean) =>
  invoke<{ id: number }>("article.decide", { id, revision, approve });
