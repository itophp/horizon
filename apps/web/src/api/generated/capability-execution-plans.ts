// Code generated from Execution Plan capability definitions. DO NOT EDIT.

export type ExecutionPlanBindInput = {
  capability: string;
  capabilityVersion: string;
  expectedRevision: number;
  planId: string;
  planVersion: number;
  reason: string;
};
export type ExecutionPlanBindOutput = { planId: string; planVersion: number };

export type ExecutionPlanBindingInput = {
  capability: string;
  capabilityVersion: string;
};
export type ExecutionPlanBindingOutput = {
  binding: {
    bindingId: string;
    plan: {
      capabilityId: string;
      capabilityVersion: string;
      graphHash?: string;
      implementationRef: string;
      mode: "direct" | "workflow" | "graph";
      planId: string;
      status: "draft" | "validated" | "published" | "archived";
      version: number;
      workflowHash?: string;
    };
    revision: number;
    scopeId?: string;
    scopeType: "platform" | "tenant";
  };
};

export type ExecutionPlanCreateInput = {
  capability: string;
  capabilityVersion: string;
  graphRef?: string;
  mode?: "direct" | "workflow" | "graph";
  name: string;
  planId: string;
  workflowRef?: string;
};
export type ExecutionPlanCreateOutput = { planId: string; planVersion: number };

export type ExecutionPlanHistoryInput = {
  capability: string;
  capabilityVersion: string;
};
export type ExecutionPlanHistoryOutput = {
  changes: Array<{
    AccountID: number;
    Action: string;
    CreatedAt: string;
    ID: number;
    PlanID: string;
    PlanVersion: number;
    Reason: string;
    Revision: number;
  }>;
};

export type ExecutionPlanInspectInput = {
  capability: string;
  capabilityVersion: string;
  planId: string;
  planVersion: number;
};
export type ExecutionPlanInspectOutput = {
  budget?: Record<string, unknown>;
  diagram?: Record<string, unknown>;
  inputSchema: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  referenceHash: string;
};

export type ExecutionPlanListInput = {
  capability: string;
  capabilityVersion: string;
};
export type ExecutionPlanListOutput = {
  plans: Array<{
    createdAt: string;
    createdBy: number;
    name: string;
    plan: {
      capabilityId: string;
      capabilityVersion: string;
      graphHash?: string;
      implementationRef: string;
      mode: "direct" | "workflow" | "graph";
      planId: string;
      status: "draft" | "validated" | "published" | "archived";
      version: number;
      workflowHash?: string;
    };
    publishedAt?: string;
    publishedBy: number;
    scopeType: "platform" | "tenant";
  }>;
};

export type ExecutionPlanPreflightInput = {
  capability: string;
  capabilityVersion: string;
  input?: Record<string, unknown>;
  planId: string;
  planVersion: number;
};
export type ExecutionPlanPreflightOutput = {
  effectsExecuted: boolean;
  valid: boolean;
};

export type ExecutionPlanPublishInput = {
  capability: string;
  capabilityVersion: string;
  planId: string;
  planVersion: number;
};
export type ExecutionPlanPublishOutput = {
  planId: string;
  planVersion: number;
};

export type ExecutionPlanRecordsInput = {
  capability?: string;
  capabilityVersion?: string;
};
export type ExecutionPlanRecordsOutput = {
  records: Array<{
    binding: {
      bindingId: string;
      plan: {
        capabilityId: string;
        capabilityVersion: string;
        graphHash?: string;
        implementationRef: string;
        mode: "direct" | "workflow" | "graph";
        planId: string;
        status: "draft" | "validated" | "published" | "archived";
        version: number;
        workflowHash?: string;
      };
      revision: number;
      scopeId?: string;
      scopeType: "platform" | "tenant";
    };
    causationId: string;
    errorCode: string;
    finishedAt?: string;
    invocationId: string;
    runId?: number;
    runStatus: string;
    startedAt: string;
    status: string;
    traceId: string;
  }>;
};

export type ExecutionPlanReferencesInput = {
  capability: string;
  capabilityVersion: string;
  mode: "workflow" | "graph";
  query?: string;
};
export type ExecutionPlanReferencesOutput = {
  references: Array<{
    hash: string;
    mode: "workflow" | "graph";
    name: string;
    ref: string;
    version: string;
  }>;
};

export type ExecutionPlanRollbackInput = {
  capability: string;
  capabilityVersion: string;
  expectedRevision: number;
  planId: string;
  planVersion: number;
  reason: string;
};
export type ExecutionPlanRollbackOutput = {
  planId: string;
  planVersion: number;
};

export type ExecutionPlanTraceInput = { invocationId: string };
export type ExecutionPlanTraceOutput = {
  events: Array<{
    eventType: "STEP_STARTED" | "STEP_FINISHED";
    occurredAt: string;
    sequence: number;
    stepName: string;
  }>;
};

export type ExecutionPlanValidateInput = {
  capability: string;
  capabilityVersion: string;
  planId: string;
  planVersion: number;
};
export type ExecutionPlanValidateOutput = {
  planId: string;
  planVersion: number;
};
