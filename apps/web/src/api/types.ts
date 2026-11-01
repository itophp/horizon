// Business types of the capability modules. The transport layer no longer
// generates them: the capability contract is the API, so these shapes are
// owned by the console and mirror the Go domain types.

export interface ChangeEntry {
  /**
   * @minLength 1
   * @maxLength 100
   */
  key: string;
  operation: ChangeEntryOperation;
  /** @maxLength 1048576 */
  value: string;
}

export type ChangeEntryOperation =
  (typeof ChangeEntryOperation)[keyof typeof ChangeEntryOperation];

export const ChangeEntryOperation = {
  set: "set",
  reset: "reset",
  clear: "clear",
} as const;

export interface ChangeRecord {
  actorId: number;
  baseRevision: number;
  changes: ChangeEntry[];
  createdAt: string;
  document?: DictionaryDocument;
  domain: string;
  id: number;
  overrides?: DictionaryOverride[];
  reason: string;
  revision: number;
  sensitive: boolean;
  status: string;
  target: ConfigTarget;
}

export interface ConfigDefinition {
  choices: string[];
  defaultValue: string;
  group: string;
  key: string;
  kind: string;
  scopes: string[];
  secret: boolean;
  sensitive: boolean;
}

export interface ConfigTarget {
  /** @minimum 0 */
  id: number;
  kind: ConfigTargetKind;
}

export type ConfigTargetKind =
  (typeof ConfigTargetKind)[keyof typeof ConfigTargetKind];

export const ConfigTargetKind = {
  workspace: "workspace",
  enterprise: "enterprise",
  company: "company",
  department: "department",
  account: "account",
} as const;

export interface DeliveryRecord {
  attempts: number;
  channel: string;
  createdAt: string;
  errorCode: string;
  id: number;
  messageId: number;
  nextAttemptAt: string;
  recipientName: string;
  status: string;
  title: string;
}

export interface DictionaryDefinition {
  allowExtensions: boolean;
  /** @maxLength 80 */
  category: string;
  /** @pattern ^[a-z][a-z0-9_.-]{1,79}$ */
  code: string;
  /** @maxLength 80 */
  consumerPermission: string;
  /** @maxLength 500 */
  description: string;
  /** @minimum 0 */
  id: number;
  kind: DictionaryDefinitionKind;
  /**
   * @minLength 1
   * @maxLength 100
   */
  nameEn: string;
  /** @maxLength 100 */
  nameZh: string;
  provider: DictionaryDefinitionProvider;
  revision: number;
  status: DictionaryDefinitionStatus;
  target: ConfigTarget;
  visibility: DictionaryDefinitionVisibility;
}

export type DictionaryDefinitionKind =
  (typeof DictionaryDefinitionKind)[keyof typeof DictionaryDefinitionKind];

export const DictionaryDefinitionKind = {
  flat: "flat",
  tree: "tree",
  dynamic: "dynamic",
} as const;

export type DictionaryDefinitionProvider =
  (typeof DictionaryDefinitionProvider)[keyof typeof DictionaryDefinitionProvider];

export const DictionaryDefinitionProvider = {
  "": "",
  organizations: "organizations",
  departments: "departments",
  roles: "roles",
  accounts: "accounts",
} as const;

export type DictionaryDefinitionStatus =
  (typeof DictionaryDefinitionStatus)[keyof typeof DictionaryDefinitionStatus];

export const DictionaryDefinitionStatus = {
  active: "active",
  inactive: "inactive",
  archived: "archived",
} as const;

export type DictionaryDefinitionVisibility =
  (typeof DictionaryDefinitionVisibility)[keyof typeof DictionaryDefinitionVisibility];

export const DictionaryDefinitionVisibility = {
  shared: "shared",
  management: "management",
} as const;

export interface DictionaryDetail {
  document: DictionaryDocument;
  drafts: ChangeRecord[];
  history: ChangeRecord[];
  overrideRevision: number;
  overrides: DictionaryOverride[];
  references: number;
}

export interface DictionaryDocument {
  definition: DictionaryDefinition;
  /** @maxItems 10000 */
  items: DictionaryItem[];
}

export interface DictionaryItem {
  /** @pattern ^[A-Za-z0-9_.-]{1,80}$ */
  code: string;
  /** @maxLength 30 */
  color: string;
  /** @maxLength 500 */
  descriptionEn: string;
  /** @maxLength 500 */
  descriptionZh: string;
  endsAt: string;
  /** @maxLength 40 */
  icon: string;
  /** @minimum 0 */
  id: number;
  isDefault: boolean;
  /**
   * @minLength 1
   * @maxLength 200
   */
  labelEn: string;
  /** @maxLength 200 */
  labelZh: string;
  owner?: ConfigTarget;
  /** @maxLength 80 */
  parentCode: string;
  sort: number;
  startsAt: string;
  status: DictionaryItemStatus;
}

export type DictionaryItemStatus =
  (typeof DictionaryItemStatus)[keyof typeof DictionaryItemStatus];

export const DictionaryItemStatus = {
  active: "active",
  inactive: "inactive",
  archived: "archived",
} as const;

export interface DictionaryOverride {
  /** @maxItems 6 */
  fields: string[];
  isDefault: boolean;
  /** @minimum 1 */
  itemId: number;
  /** @maxLength 200 */
  labelEn: string;
  /** @maxLength 200 */
  labelZh: string;
  reset: boolean;
  sort: number;
  status: DictionaryOverrideStatus;
}

export type DictionaryOverrideStatus =
  (typeof DictionaryOverrideStatus)[keyof typeof DictionaryOverrideStatus];

export const DictionaryOverrideStatus = {
  active: "active",
  inactive: "inactive",
} as const;

export interface ImportPreview {
  added: number;
  document: DictionaryDocument;
  errors: string[];
  updated: number;
}

export interface InboxMessage {
  applicantName: string;
  attachments: NotificationAttachment[];
  createdAt: string;
  departmentName: string;
  event: string;
  id: number;
  markdown: string;
  read: boolean;
  requestId: number;
  status: string;
  title: string;
}

export interface MCPToken {
  createdAt: string;
  departmentId: number;
  expiresAt: string;
  id: number;
  lastUsedAt: string;
  name: string;
  prefix: string;
  revokedAt: string;
  revokedBy: number;
  tools: string[];
}

export interface ManagedFile {
  createdAt: string;
  departmentId: number;
  id: number;
  link: string;
  mime: string;
  name: string;
  ownerId: number;
  purpose: string;
  size: number;
  storageVersion: number;
}

export interface NotificationAttachment {
  id: number;
  mime: string;
  name: string;
}

export interface SentNotification {
  attachments: NotificationAttachment[];
  createdAt: string;
  id: number;
  markdown: string;
  recipientCount: number;
  title: string;
}

export interface SettingSchedule {
  actorId: number;
  baseRevision: number;
  createdAt: string;
  departmentId: number;
  draftId: number;
  errorCode: string;
  finishedAt: string;
  id: number;
  reason: string;
  scheduledAt: string;
  status: SettingScheduleStatus;
  versionId: number;
}

export type SettingScheduleStatus =
  (typeof SettingScheduleStatus)[keyof typeof SettingScheduleStatus];

export const SettingScheduleStatus = {
  pending: "pending",
  published: "published",
  cancelled: "cancelled",
  failed: "failed",
} as const;

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface SettingsWorkspace {
  definitions: ConfigDefinition[];
  drafts: ChangeRecord[];
  entries: ConfigEntry[];
  history: ChangeRecord[];
  revision: number;
  target: ConfigTarget;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface ConfigEntry {
  configured: boolean;
  key: string;
  overridden: boolean;
  source: string;
  value: string;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface ReauthProof {
  expiresAt: string;
  proof: string;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface DictionaryWorkspace {
  definitions: DictionaryDefinition[];
  target: ConfigTarget;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface DictionaryOption {
  code: string;
  color: string;
  disabled: boolean;
  icon: string;
  id: number;
  isDefault: boolean;
  label: string;
  parentCode: string;
  source: string;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface StorageDriver {
  code: string;
  configKeys: string[];
  writable: boolean;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface NotificationAudience {
  /** @minimum 1 */
  id: number;
  kind: NotificationAudienceKind;
  name: string;
  recipients: number;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export type NotificationAudienceKind =
  (typeof NotificationAudienceKind)[keyof typeof NotificationAudienceKind];

export const NotificationAudienceKind = {
  account: "account",
  department: "department",
  company: "company",
  enterprise: "enterprise",
} as const;

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface NotificationTarget {
  /** @minimum 1 */
  id: number;
  kind: NotificationTargetKind;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export type NotificationTargetKind =
  (typeof NotificationTargetKind)[keyof typeof NotificationTargetKind];

export const NotificationTargetKind = {
  account: "account",
  department: "department",
  company: "company",
  enterprise: "enterprise",
} as const;

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface MCPTokenCreated {
  secret: string;
  token: MCPToken;
}

/**
 * Generated by orval 🍺
 * Do not edit manually.
 * Baseline API
 * Application infrastructure and technical demonstrations
 * OpenAPI spec version: 0.0.1
 */

export interface MCPAudit {
  createdAt: string;
  departmentId: number;
  durationMs: number;
  id: number;
  requestId: string;
  resultCode: string;
  tokenId: number;
  tool: string;
}
