import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Copy, Eye, Check, Upload, RotateCcw, ArrowRight } from "lucide-react";
import { DataTable, RowActions } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { planKey, type Source, type PlanRow, type Binding } from "./state";

export function PlanVersions({
  rows,
  binding,
  can,
  busy,
  onSelect,
  onCopy,
  onTransition,
  onBinding,
}: {
  rows: PlanRow[];
  binding?: Binding;
  can: (action: string) => boolean;
  busy: boolean;
  onSelect: (source: Source) => void;
  onCopy: (source: Source) => void;
  onTransition: (action: "validate" | "publish", source: Source) => void;
  onBinding: (source: Source, rollback: boolean) => void;
}) {
  const { t } = useTranslation("capability");
  const data = useMemo(() => rows.map((row, id) => ({ ...row, id })), [rows]);
  return (
    <section className="admin-panel">
      <DataTable
        rows={data}
        columns={[
          {
            id: "name",
            header: t("plans.planName"),
            cell: (r) => (
              <div className="ai-cell-stack">
                <Button variant="link" onClick={() => onSelect(r)}>
                  {t("plans.namedVersion", {
                    name: r.name,
                    version: r.plan.version,
                  })}
                </Button>
                <small>
                  <code>{r.plan.planId}</code>
                </small>
              </div>
            ),
          },
          {
            id: "mode",
            header: t("plans.mode"),
            cell: (r) => t(`plans.modes.${r.plan.mode}`),
          },
          {
            id: "status",
            header: t("plans.status"),
            cell: (r) => (
              <>
                {t(`plans.states.${r.plan.status}`)}
                {binding && planKey(binding.plan) === planKey(r.plan) && (
                  <span className="ai-badge">{t("plans.active")}</span>
                )}
              </>
            ),
          },
          {
            id: "scope",
            header: t("plans.scope"),
            cell: (r) => t(`plans.scopes.${r.scopeType}`),
          },
          {
            id: "actions",
            header: t("plans.actions"),
            cell: (r) => (
              <RowActions
                actions={[
                  {
                    label: t("plans.inspect"),
                    icon: <Eye size={16} />,
                    onClick: () => onSelect(r),
                  },
                  ...(can("edit")
                    ? [
                        {
                          label: t("plans.copy"),
                          icon: <Copy size={16} />,
                          onClick: () => onCopy(r),
                        },
                      ]
                    : []),
                  ...(can("validate") &&
                  r.scopeType === "tenant" &&
                  r.plan.status === "draft" &&
                  !busy
                    ? [
                        {
                          label: t("plans.validate"),
                          icon: <Check size={16} />,
                          onClick: () => onTransition("validate", r),
                        },
                      ]
                    : []),
                  ...(can("publish") &&
                  r.scopeType === "tenant" &&
                  r.plan.status === "validated" &&
                  !busy
                    ? [
                        {
                          label: t("plans.publish"),
                          icon: <Upload size={16} />,
                          onClick: () => onTransition("publish", r),
                        },
                      ]
                    : []),
                  ...(can("bind") &&
                  binding &&
                  r.plan.status === "published" &&
                  planKey(binding.plan) !== planKey(r.plan)
                    ? [
                        {
                          label: t("plans.bind"),
                          icon: <ArrowRight size={16} />,
                          onClick: () => onBinding(r, false),
                        },
                      ]
                    : []),
                  ...(can("rollback") &&
                  binding &&
                  binding.revision > 0 &&
                  r.plan.status === "published" &&
                  planKey(binding.plan) !== planKey(r.plan)
                    ? [
                        {
                          label: t("plans.rollback"),
                          icon: <RotateCcw size={16} />,
                          onClick: () => onBinding(r, true),
                        },
                      ]
                    : []),
                ]}
              />
            ),
          },
        ]}
      />
    </section>
  );
}
