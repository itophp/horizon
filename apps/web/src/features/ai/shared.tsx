import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Check, ShieldAlert, X } from "lucide-react";
import type {
  AIRun,
  CapabilityApproval,
  CapabilityInvocation,
} from "@/api/generated/models";
import { useSession } from "@/features/administration/auth";
import { capabilityLabel, formatMoment } from "@/lib/capability";

// Query keys include the trusted session scope so switching account or
// department never reuses another context's cache.
export function useAiKey(...parts: (string | number)[]) {
  const session = useSession();
  return [
    "ai",
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    ...parts,
  ] as const;
}

const riskTone: Record<string, string> = {
  low: "is-low",
  medium: "is-medium",
  high: "is-high",
  critical: "is-critical",
};

export function RiskBadge({ risk }: { risk: string }) {
  const { t } = useTranslation("capability");
  return (
    <span className={`ai-badge ai-risk ${riskTone[risk] ?? "is-low"}`}>
      {t(`risk.${risk}`, { defaultValue: risk })}
    </span>
  );
}

export function RunStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation("ai");
  return (
    <span className={`ai-badge ai-status-${status}`}>
      {t(`runStatus.${status}`, { defaultValue: status })}
    </span>
  );
}

export function resultLabel(t: TFunction, code: string) {
  if (!code) return "";
  return t(`code.${code}`, { defaultValue: code });
}

function Invocation({ item }: { item: CapabilityInvocation }) {
  const { t } = useTranslation(["ai", "capability"]);
  const tone =
    item.status === "failed"
      ? "is-failed"
      : item.status === "awaiting_approval"
        ? "is-pending"
        : "is-succeeded";
  return (
    <li className={`ai-step ${tone}`}>
      <span className="ai-step-body">
        <strong>{capabilityLabel(t, item.capability)}</strong>
        <small>
          {t("trace.invocationMeta", {
            origin: t(`capability:invocations.origin.${item.origin}`, {
              defaultValue: item.origin,
            }),
            status: t(`capability:invocations.status.${item.status}`, {
              defaultValue: item.status,
            }),
            error: item.errorCode
              ? t("trace.errorSuffix", { code: resultLabel(t, item.errorCode) })
              : "",
            duration: item.durationMs
              ? t("trace.durationSuffix", { ms: item.durationMs })
              : "",
          })}
        </small>
      </span>
      {item.status === "succeeded" && <Check size={14} aria-hidden="true" />}
      {item.status === "awaiting_approval" && (
        <ShieldAlert size={14} aria-hidden="true" />
      )}
      {item.status === "failed" && <X size={14} aria-hidden="true" />}
    </li>
  );
}

export function RunTrace({ run }: { run: AIRun }) {
  const { t } = useTranslation("ai");
  if (!run.invocations.length) return null;
  return (
    <div className="ai-trace">
      <h4>{t("trace.title")}</h4>
      <ol className="ai-steps">
        {run.invocations.map((item) => (
          <Invocation key={item.invocationId} item={item} />
        ))}
      </ol>
      <p className="ai-trace-meta">
        {t("trace.meta", {
          tools: run.toolCalls,
          duration: run.durationMs,
          prompt: run.promptTokens,
          completion: run.completionTokens,
        })}
        {run.traceId ? ` · ${t("trace.id")}: ${run.traceId.slice(0, 12)}` : ""}
      </p>
    </div>
  );
}

export function ApprovalCard({
  approval,
  busy,
  onDecide,
}: {
  approval: CapabilityApproval;
  busy: boolean;
  onDecide: (approve: boolean, note: string) => Promise<void>;
}) {
  const { t, i18n } = useTranslation(["ai", "capability"]);
  return (
    <section className="ai-approval" aria-live="polite">
      <header>
        <ShieldAlert size={18} aria-hidden="true" />
        <div>
          <strong>{t("ai:approval.title")}</strong>
          <small>
            {t("ai:approval.meta", {
              capability: capabilityLabel(t, approval.capability),
              time: t("ai:approval.expires", {
                time: formatMoment(i18n.language, approval.expiresAt),
              }),
            })}
          </small>
        </div>
        <RiskBadge risk={approval.risk} />
      </header>
      <dl className="ai-approval-arguments">
        {Object.entries(
          (approval.arguments as Record<string, unknown> | undefined) ?? {},
        ).map(([key, value]) => (
          <div key={key}>
            <dt>{t(`ai:argument.${key}`, { defaultValue: key })}</dt>
            <dd>
              {Array.isArray(value)
                ? value
                    .map((item) =>
                      typeof item === "object" && item !== null
                        ? `${String((item as { kind?: string }).kind ?? "")}/${String((item as { id?: number }).id ?? "")}`
                        : String(item),
                    )
                    .join(", ")
                : typeof value === "boolean"
                  ? value
                    ? t("ai:yes")
                    : t("ai:no")
                  : String(value)}
            </dd>
          </div>
        ))}
      </dl>
      <div className="ai-approval-actions">
        <button
          type="button"
          className="ai-button"
          disabled={busy}
          onClick={() => void onDecide(true, "")}
        >
          {t("ai:approval.approve")}
        </button>
        <button
          type="button"
          className="ai-button is-ghost"
          disabled={busy}
          onClick={() => void onDecide(false, "")}
        >
          {t("ai:approval.reject")}
        </button>
      </div>
    </section>
  );
}
