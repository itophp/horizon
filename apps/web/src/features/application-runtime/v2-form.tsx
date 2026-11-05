import { useWorkspaceInstance } from "./workspace";
import { formSchema } from "./form-schema";
import { useEffect, useId, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useQueries } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { RuntimeFormV2 } from "@/api/generated/runtime";
import { Button } from "@/components/ui/button";
import { CommandFields } from "./command-fields";
import { ErrorBanner } from "@/features/administration/shared";
import { evaluate, type Json, type Scope } from "./expression";
import { providerOf, readProvider } from "./data";
import { useRuntime, type RecordData } from "./v2-registry";
import { errorReference, useDispatcher } from "./action";
export function CommandForm({
  definition,
  mode,
  scope,
  record,
  onSaved,
  onGuard,
}: {
  definition: RuntimeFormV2;
  mode: string;
  scope: Scope;
  record?: RecordData;
  onSaved: (reference: string) => void;
  onGuard: (guard: () => boolean) => void;
}) {
  const { t } = useTranslation("runtime");
  const ctx = useRuntime();
  const workspace = useWorkspaceInstance();
  const dispatcher = useDispatcher();
  const id = useId();
  const command = mode === "edit" ? definition.edit : definition.create;
  const defaults: Record<string, Json> = {};
  for (const f of command.fields) {
    const value =
      record?.[f.name] ??
      (f.default.kind ? evaluate(f.default, scope) : undefined);
    defaults[f.name] =
      (value as Json) ??
      (f.type === "array" || f.multiple
        ? []
        : f.type === "boolean"
          ? false
          : f.type === "reference"
            ? 0
            : "");
  }
  const schema = formSchema(command.fields, scope);
  const form = useForm<Record<string, unknown>>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
  });
  const [definitionVersion] = useState(
    `${ctx.application.metadataVersion}:${ctx.application.installationRevision ?? 0}:${ctx.application.applicationRevision ?? 0}`,
  );
  const stale =
    ctx.readOnly ||
    definitionVersion !==
      `${ctx.application.metadataVersion}:${ctx.application.installationRevision ?? 0}:${ctx.application.applicationRevision ?? 0}`;
  const watched = useWatch({ control: form.control }) as RecordData;
  useEffect(() => {
    for (const field of command.fields) {
      if (!field.compute?.kind) continue;
      const next = evaluate(field.compute, { ...scope, form: watched });
      if (JSON.stringify(next) !== JSON.stringify(watched[field.name]))
        form.setValue(field.name, next, { shouldDirty: true });
    }
  }, [watched, command.fields, scope, form]);
  const [searches, setSearches] = useState<Record<string, string>>({});
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();
  const [notice, setNotice] = useState("");
  const [outcomes, setOutcomes] = useState<
    { id: string | number; status: string }[]
  >([]);
  const [retryIds, setRetryIds] = useState<Json[] | null>(null);
  const references = command.fields.filter((f) => f.reference);
  const queries = useQueries({
    queries: references.map((f) => {
      const provider = providerOf(ctx.application, f.reference);
      return {
        queryKey: [
          "runtime-v2",
          ctx.identity,
          ctx.application.id,
          `${ctx.application.metadataVersion}:${ctx.application.installationRevision ?? 0}:${ctx.application.applicationRevision ?? 0}`,
          provider.id,
          scope.route,
          searches[f.name],
        ],
        queryFn: ({ signal }: { signal: AbortSignal }) =>
          readProvider(
            provider,
            {
              ...scope,
              query: {
                ...(scope.query &&
                typeof scope.query === "object" &&
                !Array.isArray(scope.query)
                  ? scope.query
                  : {}),
                ...(f.searchParam
                  ? { [f.searchParam]: searches[f.name] ?? "" }
                  : {}),
              },
            },
            signal,
            ctx.application,
            ctx.page.id,
          ),
        retry: false,
        enabled: !stale,
      };
    }),
  });
  useEffect(() => {
    const guard = () =>
      !pending && (!form.formState.isDirty || window.confirm(t("discard")));
    onGuard(guard);
    const remove = workspace?.setGuard(guard);
    return () => {
      onGuard(() => true);
      remove?.();
    };
  }, [pending, form.formState.isDirty, onGuard, t, workspace]);
  useEffect(() => {
    const prevent = (e: BeforeUnloadEvent) => {
      if (form.formState.isDirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", prevent);
    return () => window.removeEventListener("beforeunload", prevent);
  }, [form.formState.isDirty]);
  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        if (pending || stale) return;
        setPending(true);
        setError(undefined);
        try {
          const active: RecordData = {};
          for (const f of command.fields)
            if (
              !f.hidden &&
              !f.readOnly &&
              evaluate(f.visibleWhen, { ...scope, form: values as RecordData })
            )
              if (
                !f.required &&
                (f.type === "integer" ||
                  f.type === "number" ||
                  f.type === "reference") &&
                (values[f.name] === "" ||
                  values[f.name] === undefined ||
                  values[f.name] === null)
              )
                continue;
              else active[f.name] = values[f.name] as Json;
          const result = await dispatcher.call(
            {
              id: definition.id + mode,
              type: command.actionType,
              capability: command.capability,
              inputSchema: command.inputSchema,
              input: command.input,
              tags: definition.tags,
            },
            {
              ...scope,
              record: record ?? scope.record,
              form: active,
              selection: retryIds ? { ids: retryIds } : scope.selection,
            },
          );
          if (result.status === "succeeded") {
            if (
              Array.isArray(
                (result.data as { items?: unknown[] } | undefined)?.items,
              )
            ) {
              const items = (result.data as { items: { status: string }[] })
                .items;
              setOutcomes(
                z
                  .array(
                    z.object({
                      id: z.union([z.number(), z.string()]),
                      status: z.string(),
                    }),
                  )
                  .parse(items),
              );
              if (items.some((i) => i.status !== "succeeded")) {
                setNotice(
                  t("batchResult", {
                    succeeded: items.filter((i) => i.status === "succeeded")
                      .length,
                    failed: items.filter((i) => i.status !== "succeeded")
                      .length,
                  }),
                );
                return;
              }
            }
            onSaved(result.traceId || result.invocationId || ctx.correlationId);
          } else setNotice(t("approval", { id: result.approvalId }));
        } catch (e) {
          setError(e);
        } finally {
          setPending(false);
        }
      })}
    >
      {stale && <div role="alert">{t("definitionExpired")}</div>}
      {error !== undefined && (
        <>
          <ErrorBanner error={error} />
          <small>
            {t("reference", { id: errorReference(error, ctx.correlationId) })}
          </small>
        </>
      )}{" "}
      {notice && <div role="status">{notice}</div>}
      {outcomes.length > 0 && (
        <ul>
          {outcomes.map((item) => (
            <li key={String(item.id)}>
              {t("itemOutcome", {
                id: item.id,
                outcome: t(item.status === "succeeded" ? "saved" : "failed"),
              })}
            </li>
          ))}
        </ul>
      )}
      {outcomes.some((item) => item.status !== "succeeded") && (
        <Button
          type="button"
          disabled={pending}
          onClick={() => {
            setRetryIds(
              outcomes
                .filter((item) => item.status !== "succeeded")
                .map((item) => item.id),
            );
            setOutcomes([]);
            setNotice("");
          }}
        >
          {t("retryFailed")}
        </Button>
      )}
      {stale ? (
        <textarea
          readOnly
          aria-label={t("draftCopy")}
          value={JSON.stringify(watched, null, 2)}
        />
      ) : (
        <CommandFields
          fields={command.fields}
          groups={definition.groups ?? []}
          control={form.control}
          errors={form.formState.errors}
          disabled={pending || stale}
          scope={scope}
          watched={watched}
          references={references}
          queries={queries}
          id={id}
          searches={searches}
          onSearch={(name, value) =>
            setSearches((old) => ({ ...old, [name]: value }))
          }
        />
      )}
      {queries.some((q) => q.isError) && (
        <ErrorBanner error={queries.find((q) => q.isError)?.error} />
      )}
      <Button
        type="submit"
        disabled={
          pending ||
          stale ||
          !command.capability.name ||
          queries.some((q) => q.isPending || q.isError)
        }
      >
        {t("save")}
      </Button>
    </form>
  );
}
