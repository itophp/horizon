import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { executionPlans } from "@/lib/capabilities";
import { ApiError } from "@/api/client/request";
import { ErrorBanner, Loading } from "@/features/administration/shared";
import { WorkflowViewer } from "@/features/orchestration/viewer";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import { selector, usePlanScope, type Target, type Source } from "./state";
import "@/features/orchestration/orchestration.css";

const when = z.object({
  from: z.string(),
  field: z.string(),
  equals: z.unknown(),
});
const diagramSchema = z.object({
  name: z.string(),
  version: z.string(),
  hash: z.string(),
  input: z
    .object({
      properties: z.record(z.string(), z.unknown()).optional(),
      required: z.array(z.string()).optional(),
    })
    .optional(),
  nodes: z.array(
    z.object({
      id: z.string(),
      kind: z.enum(["start", "end", "step", "agent", "poll"]),
      capability: z.string().optional(),
      version: z.string().optional(),
      compiled: z.boolean(),
      dependsOn: z.array(z.string()).optional(),
      tools: z.array(z.string()).optional(),
      maxAttempts: z.number().optional(),
      compensation: z.string().optional(),
      when: when.optional(),
    }),
  ),
  edges: z.array(
    z.object({
      from: z.string(),
      to: z.string(),
      kind: z.enum(["control", "data", "branch"]),
    }),
  ),
});

export function PlanInspector({
  target,
  source,
  current,
  canValidate,
}: {
  target: Target;
  source: Source;
  current: Source;
  canValidate: boolean;
}) {
  const { t } = useTranslation("capability");
  const scope = usePlanScope(target);
  const [sample, setSample] = useState("{}");
  const info = useQuery({
    queryKey: [...scope, "inspect", source.plan.planId, source.plan.version],
    queryFn: ({ signal }) =>
      executionPlans.inspect(selector(target, source.plan), signal),
    retry: false,
  });
  const preflight = useMutation({
    mutationFn: async () => {
      let input: unknown;
      try {
        input = JSON.parse(sample);
      } catch {
        throw new ApiError("invalid", 400);
      }
      const object = z.record(z.string(), z.unknown()).safeParse(input);
      if (!object.success) throw new ApiError("invalid", 400);
      return executionPlans.preflight({
        ...selector(target, source.plan),
        input: object.data,
      });
    },
  });
  const metadata = info.data?.data;
  const parsed = diagramSchema.safeParse(metadata?.diagram);
  const changes = (
    [
      "version",
      "mode",
      "implementationRef",
      "workflowHash",
      "graphHash",
    ] as const
  ).filter((field) => source.plan[field] !== current.plan[field]);
  return (
    <section
      className="admin-panel p-4 space-y-4"
      aria-label={t("plans.inspect")}
    >
      <h3>
        {t("plans.namedVersion", {
          name: source.name,
          version: source.plan.version,
        })}
      </h3>
      {info.isPending ? (
        <Loading />
      ) : info.error ? (
        <ErrorBanner error={info.error} />
      ) : (
        <>
          {metadata?.diagram && !parsed.success && (
            <ErrorBanner error={new ApiError("invalid", 400)} />
          )}
          {parsed.success && <WorkflowViewer diagram={parsed.data} />}
          <div className="grid gap-3 md:grid-cols-2">
            <div>
              <h4>{t("plans.inputSchema")}</h4>
              <pre className="max-h-64 overflow-auto text-xs">
                {JSON.stringify(metadata?.inputSchema, null, 2)}
              </pre>
            </div>
            <div>
              <h4>{t("plans.outputSchema")}</h4>
              <pre className="max-h-64 overflow-auto text-xs">
                {JSON.stringify(metadata?.outputSchema ?? {}, null, 2)}
              </pre>
            </div>
          </div>
          {metadata?.budget && (
            <div>
              <h4>{t("plans.budget")}</h4>
              <pre className="overflow-auto text-xs">
                {JSON.stringify(metadata.budget, null, 2)}
              </pre>
            </div>
          )}
          {canValidate && (
            <div className="space-y-2">
              <label className="grid gap-1">
                {t("plans.sampleInput")}
                <Textarea
                  value={sample}
                  maxLength={32000}
                  onChange={(event) => {
                    setSample(event.target.value);
                    preflight.reset();
                  }}
                />
              </label>
              <p>{t("plans.preflightHint")}</p>
              <Button
                variant="outline"
                disabled={preflight.isPending}
                onClick={() => preflight.mutate()}
              >
                {t("plans.preflight")}
              </Button>
              {!!preflight.error && <ErrorBanner error={preflight.error} />}{" "}
              {preflight.isSuccess && (
                <p role="status">{t("plans.preflightPassed")}</p>
              )}
            </div>
          )}
        </>
      )}
      <h4>{t("plans.compare")}</h4>
      {!changes.length ? (
        <p>{t("plans.noDifferences")}</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("plans.field")}</TableHead>
              <TableHead>{t("plans.current")}</TableHead>
              <TableHead>{t("plans.selected")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {changes.map((field) => (
              <TableRow key={field}>
                <TableCell>{t(`plans.fields.${field}`)}</TableCell>
                <TableCell className="break-all">
                  {field === "mode"
                    ? t(`plans.modes.${current.plan.mode}`)
                    : String(current.plan[field] ?? t("plans.none"))}
                </TableCell>
                <TableCell className="break-all">
                  {field === "mode"
                    ? t(`plans.modes.${source.plan.mode}`)
                    : String(source.plan[field] ?? t("plans.none"))}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  );
}
