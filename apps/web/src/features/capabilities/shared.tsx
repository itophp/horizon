import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useSession } from "@/features/administration/auth";
import { riskTone } from "@/lib/capability";

// Query keys are scoped by the trusted session so switching account or
// department never reuses another context's cache.
export function useCapabilityKey(...parts: (string | number)[]) {
  const session = useSession();
  return [
    "capability",
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    ...parts,
  ] as const;
}

export function RiskBadge({ risk }: { risk: string }) {
  const { t } = useTranslation("capability");
  return (
    <span className={`ai-badge ai-risk ${riskTone(risk)}`}>
      {t(`risk.${risk}`, { defaultValue: risk })}
    </span>
  );
}

export function ApprovalPolicy({ policy }: { policy: string }) {
  const { t } = useTranslation("capability");
  return <span>{t(`approval.${policy}`, { defaultValue: policy })}</span>;
}

export function InvocationStatus({ status }: { status: string }) {
  const { t } = useTranslation("capability");
  return (
    <span className={`ai-badge ai-status-${status}`}>
      {t(`invocations.status.${status}`, { defaultValue: status })}
    </span>
  );
}

export function AiStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation("capability");
  if (!status) {
    return <span className="ai-badge">{t("events.aiStatus.none")}</span>;
  }
  const tone =
    status === "completed"
      ? "ai-status-completed"
      : status === "failed"
        ? "ai-status-failed"
        : status === "running" || status === "queued"
          ? "ai-status-running"
          : "";
  return (
    <span className={`ai-badge ${tone}`}>
      {t(`events.aiStatus.${status}`, { defaultValue: status })}
    </span>
  );
}

export function statusLabel(t: TFunction, status: string) {
  return t(`status.${status}`, { defaultValue: status });
}
