import {
  aiStatus,
  capabilityApprovals,
  capabilityDefinitions,
  capabilityEvents,
  capabilityInvocations,
} from "@/api/generated/baseline";
import { ApiError } from "@/api/client/request";

function project<T, R>(
  response: { status: number; data: unknown },
  data: T | undefined,
  select: (data: T) => R,
): R {
  if (data === undefined) throw new ApiError("unavailable", response.status);
  return select(data);
}

export async function readSystemData(
  adapter: string,
  input: Record<string, unknown>,
  signal: AbortSignal,
): Promise<unknown> {
  const options = { signal };
  const limit = typeof input.limit === "number" ? input.limit : 20;
  switch (adapter) {
    case "system.capability-definitions": {
      const response = await capabilityDefinitions(options);
      return project(
        response,
        response.status === 200 ? response.data : undefined,
        (rows) =>
          rows.map(
            ({
              name,
              version,
              description,
              domain,
              entity,
              permission,
              risk,
              approval,
              successEvents,
            }) => ({
              name,
              version,
              description,
              domain,
              entity,
              permission,
              risk,
              approval,
              successEvents,
            }),
          ),
      );
    }
    case "system.capability-approvals": {
      const response = await capabilityApprovals(options);
      return project(
        response,
        response.status === 200 ? response.data : undefined,
        (rows) =>
          rows.map(
            ({
              id,
              invocationId,
              capability,
              version,
              status,
              expiresAt,
              approval,
              mine,
            }) => ({
              id,
              invocationId,
              capability,
              version,
              status,
              expiresAt,
              approval,
              mine,
            }),
          ),
      );
    }
    case "system.capability-events": {
      const response = await capabilityEvents({ limit }, options);
      return project(
        response,
        response.status === 200 ? response.data : undefined,
        (rows) =>
          rows.map(
            ({
              eventId,
              type,
              aggregateType,
              aggregateId,
              createdAt,
              depth,
              aiPolicy,
              aiStatus,
            }) => ({
              eventId,
              type,
              aggregateType,
              aggregateId,
              createdAt,
              depth,
              aiPolicy,
              aiStatus,
            }),
          ),
      );
    }
    case "system.capability-invocations": {
      const response = await capabilityInvocations({ limit }, options);
      return project(
        response,
        response.status === 200 ? response.data : undefined,
        (rows) =>
          rows.map(
            ({
              invocationId,
              capability,
              version,
              correlationId,
              origin,
              status,
              durationMs,
              createdAt,
              errorCode,
            }) => ({
              invocationId,
              capability,
              version,
              correlationId,
              origin,
              status,
              durationMs,
              createdAt,
              errorCode,
            }),
          ),
      );
    }
    case "system.ai-status": {
      const response = await aiStatus(options);
      return project(
        response,
        response.status === 200 ? response.data : undefined,
        ({ enabled, configured }) => ({ enabled, configured }),
      );
    }
    default:
      throw new ApiError("invalid", 400);
  }
}
