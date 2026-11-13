import { useState } from "react";
import { useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import { runtimeRead, runtimeExecute } from "@/lib/capabilities";
import { runtimeDefinitionListSchema } from "@/api/generated/runtime";
import { useSession } from "@/features/administration/auth";
import {
  PageHeading,
  ErrorBanner,
  Loading,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { TaskPanel } from "./task-panel";
import { DesignVersion } from "./design-version";
import { DesignContracts } from "./design-contracts";
const schema = z.object({ request: z.string().min(1).max(4000) });
export function ApplicationDesignPage() {
  const { t, i18n } = useTranslation("runtime");
  const session = useSession();
  const [params, setParams] = useSearchParams();
  const identity = JSON.stringify([
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.authorizationVersion,
  ]);
  const list = useQuery({
    queryKey: ["runtime-design-list", identity],
    queryFn: async ({ signal }) =>
      runtimeDefinitionListSchema.parse(
        await runtimeRead("application.runtime.definition.list", {}, signal),
      ),
    retry: false,
  });
  const selected =
    params.get("application") ?? list.data?.items[0]?.applicationId;
  const [runId, setRunId] = useState<number>();
  const [error, setError] = useState<unknown>();
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: { request: "" },
  });
  const canDesign =
    session.permissions.includes("applications:design") &&
    session.permissions.includes("assistant:use") &&
    session.permissions.includes("orchestration:run");
  return (
    <>
      <PageHeading title={t("design.heading")} subtitle={t("design.hint")} />
      {list.isPending ? (
        <Loading />
      ) : list.isError ? (
        <ErrorBanner error={list.error} />
      ) : (
        <label>
          {t("design.application")}
          <select
            value={selected ?? ""}
            onChange={(event) => setParams({ application: event.target.value })}
          >
            {[
              ...list.data.items,
              {
                applicationId: "new",
                labelKey: "runtime:design.new",
                labelEn: "",
                labelZhCN: "",
              },
            ].map((app) => (
              <option key={app.applicationId} value={app.applicationId}>
                {(i18n.language.startsWith("zh")
                  ? app.labelZhCN
                  : app.labelEn) || t(app.labelKey)}
              </option>
            ))}
          </select>
        </label>
      )}
      {selected && (
        <>
          <form
            onSubmit={form.handleSubmit(async (values) => {
              setError(undefined);
              try {
                const result = await runtimeExecute(
                  "orchestration.workflow.run",
                  "1.0.0",
                  {
                    flow: "application-runtime-designer",
                    version: "1.0.0",
                    input: { applicationId: selected, request: values.request },
                  },
                  crypto.randomUUID(),
                  crypto.randomUUID(),
                );
                if (result.status !== "succeeded")
                  throw new Error(result.errorCode || "unavailable");
                const accepted = z
                  .object({ runId: z.number().int().positive() })
                  .parse(result.data);
                setRunId(accepted.runId);
              } catch (error) {
                setError(error);
              }
            })}
          >
            <label>
              {t("design.request")}
              <textarea
                {...form.register("request")}
                disabled={!canDesign || form.formState.isSubmitting}
              />
            </label>
            {form.formState.errors.request && (
              <p role="alert">{t("invalid")}</p>
            )}
            <Button disabled={!canDesign || form.formState.isSubmitting}>
              {t("design.generate")}
            </Button>
          </form>
          {error !== undefined && <ErrorBanner error={error} />}
          {selected !== "new" && (
            <DesignVersion
              key={identity + selected}
              applicationId={selected}
              identity={identity}
            />
          )}
        </>
      )}
      {session.permissions.includes("applications:design") && (
        <DesignContracts identity={identity} />
      )}
      {runId && <TaskPanel key={runId} runId={runId} />}
    </>
  );
}
