import { useRef } from "react";
import { useSession } from "@/features/administration/auth";
import type * as EP from "@/api/generated/capability-execution-plans";

export type Target = EP.ExecutionPlanListInput;
export type PlanRow = EP.ExecutionPlanListOutput["plans"][number];
export type Plan = PlanRow["plan"];
export type Source = Pick<PlanRow, "plan" | "name" | "scopeType">;
export type Binding = EP.ExecutionPlanBindingOutput["binding"];
export const planKey = (plan: Plan) =>
  JSON.stringify([plan.planId, plan.version]);
export const selector = (
  target: Target,
  plan: Plan,
): EP.ExecutionPlanInspectInput => ({
  ...target,
  planId: plan.planId,
  planVersion: plan.version,
});
export function usePlanScope(target: Target) {
  const session = useSession();
  return [
    "execution-plans",
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.authorizationVersion,
    target.capability,
    target.capabilityVersion,
  ] as const;
}
// Keep a retry's key until the payload changes or the operation succeeds.
export function useOperationKey() {
  const key = useRef({ fingerprint: "", value: "" });
  return {
    get: (input: unknown) => {
      const fingerprint = JSON.stringify(input);
      if (key.current.fingerprint !== fingerprint) {
        key.current = { fingerprint, value: crypto.randomUUID() };
      }
      return key.current.value;
    },
    clear: () => {
      key.current = { fingerprint: "", value: "" };
    },
  };
}
