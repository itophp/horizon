import { useState } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { capabilityDefinitions } from "@/api/generated/baseline";
import type { CapabilityContract } from "@/api/generated/models";
import { executionPlans } from "@/lib/capabilities";
import { capabilityLabel } from "@/lib/capability";
import { useSession } from "@/features/administration/auth";
import {
  ErrorBanner,
  Loading,
  PageHeading,
  SuccessNotice,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { DraftDialog } from "./forms";
import { BindingDialog } from "./binding-dialog";
import { PlanVersions } from "./versions";
import { PlanInspector } from "./inspector";
import { PlanActivity } from "./activity";
import {
  planKey,
  selector,
  useOperationKey,
  usePlanScope,
  type Source,
  type Target,
  type Binding,
} from "./state";

export function CapabilityPlansPage() {
  const { t } = useTranslation("capability");
  const session = useSession();
  const { name } = useParams();
  const [params] = useSearchParams();
  const allowed = session.permissions.includes("executionPlans:read");
  const definitions = useQuery({
    queryKey: [
      "capability",
      session.tenantId,
      session.account.id,
      session.activeDepartmentId,
      session.authorizationVersion,
      "plan-definitions",
    ],
    queryFn: () => capabilityDefinitions(),
    enabled: allowed,
    retry: false,
  });
  if (!allowed) return <p role="status">{t("plans.forbidden")}</p>;
  if (definitions.isPending) return <Loading />;
  if (definitions.error) return <ErrorBanner error={definitions.error} />;
  const contract =
    definitions.data?.status === 200
      ? definitions.data.data.find(
          (item) =>
            item.name === name &&
            (!params.get("version") || item.version === params.get("version")),
        )
      : undefined;
  if (!contract) return <p role="status">{t("plans.unavailable")}</p>;
  return (
    <PlanWorkspace
      key={`${contract.name}@${contract.version}`}
      contract={contract}
    />
  );
}
function PlanWorkspace({ contract }: { contract: CapabilityContract }) {
  const { t, i18n } = useTranslation("capability");
  const session = useSession();
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const target: Target = {
    capability: contract.name,
    capabilityVersion: contract.version,
  };
  const scope = usePlanScope(target);
  const keys = useOperationKey();
  const [draft, setDraft] = useState<{ source?: Source }>();
  const [change, setChange] = useState<{
    source: Source;
    binding: Binding;
    rollback: boolean;
  }>();
  const [notice, setNotice] = useState("");
  const can = (action: string) =>
    session.permissions.includes(`executionPlans:${action}`);
  const list = useQuery({
    queryKey: [...scope, "list"],
    queryFn: ({ signal }) => executionPlans.list(target, signal),
    retry: false,
  });
  const binding = useQuery({
    queryKey: [...scope, "binding"],
    queryFn: ({ signal }) => executionPlans.binding(target, signal),
    retry: false,
  });
  const rows = list.data?.data.plans ?? [];
  const current = binding.data?.data.binding;
  const active: Source | undefined = current
    ? {
        plan: current.plan,
        scopeType: current.scopeType,
        name:
          rows.find((r) => planKey(r.plan) === planKey(current.plan))?.name ??
          t("plans.current"),
      }
    : undefined;
  const explicitSelection = params.has("planId") || params.has("planVersion");
  const selected =
    rows.find(
      (r) =>
        r.plan.planId === params.get("planId") &&
        String(r.plan.version) === params.get("planVersion"),
    ) ??
    (active &&
    (!explicitSelection ||
      (active.plan.planId === params.get("planId") &&
        String(active.plan.version) === params.get("planVersion")))
      ? active
      : undefined);

  const tab =
    params.get("tab") === "history"
      ? "history"
      : params.get("tab") === "records" && can("records")
        ? "records"
        : "versions";
  const update = (values: Record<string, string>) =>
    setParams((old) => {
      const next = new URLSearchParams(old);
      for (const [key, value] of Object.entries(values)) next.set(key, value);
      return next;
    });
  const refresh = async () => {
    await client.invalidateQueries({ queryKey: scope });
  };
  const saved = () => {
    setNotice(t("plans.saved"));
    void refresh();
  };
  const transition = useMutation({
    mutationFn: ({
      action,
      source,
    }: {
      action: "validate" | "publish";
      source: Source;
    }) => {
      const input = selector(target, source.plan);
      return executionPlans[action](input, keys.get({ action, input }));
    },
    onSuccess: () => {
      keys.clear();
      saved();
    },
  });
  const error = list.error ?? binding.error ?? transition.error;
  const select = (source: Source) =>
    update({
      planId: source.plan.planId,
      planVersion: String(source.plan.version),
      tab: "versions",
    });
  return (
    <>
      <PageHeading
        title={capabilityLabel(
          i18n,
          contract.name,
          contract.title || contract.name,
        )}
        subtitle={t("plans.subtitle")}
      >
        <Link to="/capabilities">{t("plans.back")}</Link>
        {can("edit") && (
          <Button onClick={() => setDraft({})}>{t("plans.create")}</Button>
        )}
      </PageHeading>
      {!!notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
      {!!error && <ErrorBanner error={error} onRetry={() => void refresh()} />}
      {list.isPending || binding.isPending ? (
        <Loading />
      ) : (
        <>
          {active && current && (
            <section className="admin-panel p-4 space-y-2">
              <h3>{t("plans.current")}</h3>
              <p>
                {t("plans.bindingSummary", {
                  name: active.name,
                  mode: t(`plans.modes.${active.plan.mode}`),
                  version: active.plan.version,
                  scope: t(`plans.scopes.${current.scopeType}`),
                })}
              </p>
              <p>{t("plans.revision", { revision: current.revision })}</p>
              <Button variant="outline" onClick={() => select(active)}>
                {t("plans.inspect")}
              </Button>
              {can("edit") && (
                <Button
                  variant="ghost"
                  onClick={() => setDraft({ source: active })}
                >
                  {t("plans.copy")}
                </Button>
              )}
            </section>
          )}
          <Tabs
            value={tab}
            onValueChange={(value) => {
              if (typeof value === "string") update({ tab: value });
            }}
          >
            <TabsList>
              <TabsTrigger value="versions">{t("plans.versions")}</TabsTrigger>
              <TabsTrigger value="history">{t("plans.history")}</TabsTrigger>
              {can("records") && (
                <TabsTrigger value="records">{t("plans.records")}</TabsTrigger>
              )}
            </TabsList>
            <TabsContent value="versions">
              <PlanVersions
                rows={rows}
                binding={current}
                can={can}
                busy={transition.isPending}
                onSelect={select}
                onCopy={(source) => setDraft({ source })}
                onTransition={(action, source) =>
                  transition.mutate({ action, source })
                }
                onBinding={(source, rollback) => {
                  if (current)
                    setChange({ source, binding: current, rollback });
                }}
              />
              {!selected && explicitSelection && (
                <p role="status">{t("plans.unavailable")}</p>
              )}
              {selected && active && (
                <PlanInspector
                  key={planKey(selected.plan)}
                  target={target}
                  source={selected}
                  current={active}
                  canValidate={can("validate")}
                />
              )}
            </TabsContent>
            <TabsContent value="history">
              {tab === "history" && (
                <PlanActivity target={target} plans={rows} kind="history" />
              )}
            </TabsContent>
            {can("records") && (
              <TabsContent value="records">
                {tab === "records" && (
                  <PlanActivity target={target} plans={rows} kind="records" />
                )}
              </TabsContent>
            )}
          </Tabs>
        </>
      )}
      {draft && (
        <DraftDialog
          target={target}
          source={draft.source}
          readOnly={contract.readOnly}
          onClose={() => setDraft(undefined)}
          onSaved={saved}
        />
      )}
      {change && (
        <BindingDialog
          target={target}
          {...change}
          onClose={() => setChange(undefined)}
          onSaved={saved}
        />
      )}
    </>
  );
}
