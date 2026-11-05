// Code generated from Memory Center capability definitions. DO NOT EDIT.

export type MemoryActivityCollectInput = {
  events: Array<{
    action: "view" | "open" | "submit" | "success" | "failure";
    id: string;
    kind: "page_view" | "interaction";
    occurredAt: string;
    page: string;
  }>;
};
export type MemoryActivityCollectOutput = { accepted: number };

export type MemoryEntityArchiveInput = {
  id: number;
  resourceType: "example_record";
  scopeId: number;
  scopeType: "business_entity";
  version: number;
};
export type MemoryEntityArchiveOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryEntityCorrectInput = {
  id: number;
  resourceType: "example_record";
  scopeId: number;
  scopeType: "business_entity";
  summary: string;
  title: string;
  version: number;
};
export type MemoryEntityCorrectOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryEntityCreateInput = {
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  resourceType: "example_record";
  scopeId: number;
  scopeType: "business_entity";
  sourceEventId?: string;
  summary: string;
  title: string;
  validFrom?: string;
  validTo?: string;
};
export type MemoryEntityCreateOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryEntityDeleteInput = {
  id: number;
  resourceType: "example_record";
  scopeId: number;
  scopeType: "business_entity";
  version: number;
};
export type MemoryEntityDeleteOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryEntityGetInput = {
  id: number;
  resourceType: "example_record";
  scopeId: number;
  scopeType: "business_entity";
};
export type MemoryEntityGetOutput = {
  id: number;
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  sensitivity: string;
  status: string;
  summary: string;
  title: string;
  validFrom: string;
  validTo: string;
  verificationStatus: string;
  version: number;
};

export type MemoryEntityListInput = {
  beforeId?: number;
  createdBefore?: string;
  createdFrom?: string;
  limit?: number;
  ownerId?: number;
  partition?:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  resourceType: "example_record";
  scopeId: number;
  scopeType: "business_entity";
  status?: "active" | "archived";
};
export type MemoryEntityListOutput = {
  items: Array<{
    id: number;
    memoryType: string;
    partition:
      | "agent"
      | "activity"
      | "exception"
      | "business"
      | "knowledge"
      | "execution";
    sensitivity: string;
    status: string;
    summary: string;
    title: string;
    validFrom: string;
    validTo: string;
    verificationStatus: string;
    version: number;
  }>;
};

export type MemoryEntitySourcesInput = {
  id: number;
  resourceType: "example_record";
  scopeId: number;
  scopeType: "business_entity";
};
export type MemoryEntitySourcesOutput = {
  sources: Array<{
    correlationId: string;
    eventId: string;
    eventType: string;
    occurredAt: string;
  }>;
};

export type MemoryEventListInput = {
  beforeId?: number;
  correlationId?: string;
  eventId?: string;
  eventType?: string;
  limit?: number;
  occurredBefore?: string;
  occurredFrom?: string;
  resourceId?: string;
  resourceType?: string;
};
export type MemoryEventListOutput = {
  events: Array<{
    actionName: string;
    candidatePartitions: Array<string>;
    causationId: string;
    correlationId: string;
    eventId: string;
    eventType: string;
    id: number;
    occurredAt: string;
    payload: Record<string, unknown>;
    resourceId: string;
    resourceType: string;
    result: string;
    sessionId: string;
    sourceId: string;
    sourceType: string;
    taskId: string;
    traceId: string;
    workflowInstanceId: string;
  }>;
};

export type MemoryGroupArchiveInput = {
  id: number;
  scopeId: number;
  scopeType: "department" | "organization";
  version: number;
};
export type MemoryGroupArchiveOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryGroupCorrectInput = {
  id: number;
  scopeId: number;
  scopeType: "department" | "organization";
  summary: string;
  title: string;
  version: number;
};
export type MemoryGroupCorrectOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryGroupCreateInput = {
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  scopeId: number;
  scopeType: "department" | "organization";
  sourceEventId?: string;
  summary: string;
  title: string;
  validFrom?: string;
  validTo?: string;
};
export type MemoryGroupCreateOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryGroupDeleteInput = {
  id: number;
  scopeId: number;
  scopeType: "department" | "organization";
  version: number;
};
export type MemoryGroupDeleteOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryGroupGetInput = {
  id: number;
  scopeId: number;
  scopeType: "department" | "organization";
};
export type MemoryGroupGetOutput = {
  id: number;
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  sensitivity: string;
  status: string;
  summary: string;
  title: string;
  validFrom: string;
  validTo: string;
  verificationStatus: string;
  version: number;
};

export type MemoryGroupListInput = {
  beforeId?: number;
  createdBefore?: string;
  createdFrom?: string;
  limit?: number;
  ownerId?: number;
  partition?:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  scopeId: number;
  scopeType: "department" | "organization";
  status?: "active" | "archived";
};
export type MemoryGroupListOutput = {
  items: Array<{
    id: number;
    memoryType: string;
    partition:
      | "agent"
      | "activity"
      | "exception"
      | "business"
      | "knowledge"
      | "execution";
    sensitivity: string;
    status: string;
    summary: string;
    title: string;
    validFrom: string;
    validTo: string;
    verificationStatus: string;
    version: number;
  }>;
};

export type MemoryGroupSourcesInput = {
  id: number;
  scopeId: number;
  scopeType: "department" | "organization";
};
export type MemoryGroupSourcesOutput = {
  sources: Array<{
    correlationId: string;
    eventId: string;
    eventType: string;
    occurredAt: string;
  }>;
};

export type MemoryIngestionFailuresInput = { limit?: number };
export type MemoryIngestionFailuresOutput = {
  failures: Array<{
    attempts: number;
    deadAt: string;
    errorCode: string;
    eventId: string;
    sourceEventId: number;
  }>;
};

export type MemoryIngestionReplayInput = { eventId: string };
export type MemoryIngestionReplayOutput = { queued: boolean };

export type MemoryIngestionReplayRangeInput = {
  firstSourceId: number;
  lastSourceId: number;
  limit: number;
};
export type MemoryIngestionReplayRangeOutput = {
  lastQueuedSourceId: number;
  queued: number;
};

export type MemoryItemArchiveInput = { id: number; version: number };
export type MemoryItemArchiveOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryItemCorrectInput = {
  id: number;
  summary: string;
  title: string;
  version: number;
};
export type MemoryItemCorrectOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryItemCreateInput = {
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  sourceEventId?: string;
  summary: string;
  title: string;
  validFrom?: string;
  validTo?: string;
};
export type MemoryItemCreateOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryItemDeleteInput = { id: number; version: number };
export type MemoryItemDeleteOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryItemGetInput = { id: number };
export type MemoryItemGetOutput = {
  id: number;
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  sensitivity: string;
  status: string;
  summary: string;
  title: string;
  validFrom: string;
  validTo: string;
  verificationStatus: string;
  version: number;
};

export type MemoryItemListInput = {
  beforeId?: number;
  createdBefore?: string;
  createdFrom?: string;
  limit?: number;
  ownerId?: number;
  partition?:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  status?: "active" | "archived";
};
export type MemoryItemListOutput = {
  items: Array<{
    id: number;
    memoryType: string;
    partition:
      | "agent"
      | "activity"
      | "exception"
      | "business"
      | "knowledge"
      | "execution";
    sensitivity: string;
    status: string;
    summary: string;
    title: string;
    validFrom: string;
    validTo: string;
    verificationStatus: string;
    version: number;
  }>;
};

export type MemoryItemSourcesInput = { id: number };
export type MemoryItemSourcesOutput = {
  sources: Array<{
    correlationId: string;
    eventId: string;
    eventType: string;
    occurredAt: string;
  }>;
};

export type MemoryPolicyListInput = {};
export type MemoryPolicyListOutput = {
  policies: Array<{
    partition:
      | "agent"
      | "activity"
      | "exception"
      | "business"
      | "knowledge"
      | "execution";
    retentionDays: number;
    revision: number;
  }>;
};

export type MemoryPolicySetInput = {
  expectedRevision: number;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  retentionDays: number;
};
export type MemoryPolicySetOutput = {
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  retentionDays: number;
  revision: number;
};

export type MemoryProjectArchiveInput = {
  id: number;
  projectId: number;
  version: number;
};
export type MemoryProjectArchiveOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryProjectCorrectInput = {
  id: number;
  projectId: number;
  summary: string;
  title: string;
  version: number;
};
export type MemoryProjectCorrectOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryProjectCreateInput = {
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  projectId: number;
  sourceEventId?: string;
  summary: string;
  title: string;
  validFrom?: string;
  validTo?: string;
};
export type MemoryProjectCreateOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryProjectDeleteInput = {
  id: number;
  projectId: number;
  version: number;
};
export type MemoryProjectDeleteOutput = {
  id: number;
  status: string;
  version: number;
};

export type MemoryProjectGetInput = { id: number; projectId: number };
export type MemoryProjectGetOutput = {
  id: number;
  memoryType: string;
  partition:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  sensitivity: string;
  status: string;
  summary: string;
  title: string;
  validFrom: string;
  validTo: string;
  verificationStatus: string;
  version: number;
};

export type MemoryProjectListInput = {
  beforeId?: number;
  createdBefore?: string;
  createdFrom?: string;
  limit?: number;
  ownerId?: number;
  partition?:
    "agent" | "activity" | "exception" | "business" | "knowledge" | "execution";
  projectId: number;
  status?: "active" | "archived";
};
export type MemoryProjectListOutput = {
  items: Array<{
    id: number;
    memoryType: string;
    partition:
      | "agent"
      | "activity"
      | "exception"
      | "business"
      | "knowledge"
      | "execution";
    sensitivity: string;
    status: string;
    summary: string;
    title: string;
    validFrom: string;
    validTo: string;
    verificationStatus: string;
    version: number;
  }>;
};

export type MemoryProjectSourcesInput = { id: number; projectId: number };
export type MemoryProjectSourcesOutput = {
  sources: Array<{
    correlationId: string;
    eventId: string;
    eventType: string;
    occurredAt: string;
  }>;
};
