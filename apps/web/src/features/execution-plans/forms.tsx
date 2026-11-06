import { useDeferredValue, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { executionPlans } from "@/lib/capabilities";
import { EditorDialog, ErrorBanner } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useOperationKey,
  usePlanScope,
  type Source,
  type Target,
} from "./state";

const draftSchema = z
  .object({
    planId: z.string().regex(/^[a-z][a-z0-9._-]{0,119}$/, "plans.idInvalid"),
    name: z.string().trim().min(1, "plans.required").max(160, "plans.required"),
    mode: z.enum(["direct", "workflow", "graph"]),
    ref: z.string(),
  })
  .superRefine((value, ctx) => {
    if (value.mode !== "direct" && !value.ref)
      ctx.addIssue({
        code: "custom",
        path: ["ref"],
        message: "plans.refRequired",
      });
  });
type Draft = z.infer<typeof draftSchema>;
export function DraftDialog({
  target,
  source,
  readOnly,
  onClose,
  onSaved,
}: {
  target: Target;
  source?: Source;
  readOnly: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation("capability");
  const keys = useOperationKey();
  const scope = usePlanScope(target);
  const form = useForm<Draft>({
    resolver: zodResolver(draftSchema),
    defaultValues: {
      planId:
        source?.scopeType === "tenant"
          ? source.plan.planId
          : `plan-${target.capability.replaceAll(".", "-")}`.slice(0, 120),
      name: source?.name ?? "",
      mode: source?.plan.mode ?? "direct",
      ref:
        source?.plan.mode !== "direct"
          ? (source?.plan.implementationRef ?? "")
          : "",
    },
  });
  const mode = useWatch({ control: form.control, name: "mode" });
  const ref = useWatch({ control: form.control, name: "ref" });
  const [search, setSearch] = useState(
    source?.plan.mode !== "direct"
      ? (source?.plan.implementationRef ?? "")
      : "",
  );
  const query = useDeferredValue(search);
  const references = useQuery({
    queryKey: [...scope, "references", mode, query],
    queryFn: ({ signal }) =>
      executionPlans.references(
        { ...target, mode: mode === "graph" ? "graph" : "workflow", query },
        signal,
      ),
    enabled: mode !== "direct",
    retry: false,
  });
  const options = references.data?.data.references ?? [];
  const chosen = options.find((item) => item.ref === ref);
  const save = useMutation({
    mutationFn: (draft: Draft) => {
      const input = {
        ...target,
        planId: draft.planId,
        name: draft.name,
        mode: draft.mode,
        ...(draft.mode === "workflow"
          ? { workflowRef: draft.ref }
          : draft.mode === "graph"
            ? { graphRef: draft.ref }
            : {}),
      };
      return executionPlans.create(input, keys.get(input));
    },
    onSuccess: () => {
      keys.clear();
      onSaved();
      onClose();
    },
  });
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) onClose();
      }}
      title={t(source ? "plans.copy" : "plans.create")}
      description={t("plans.draftHint")}
    >
      <form
        onSubmit={form.handleSubmit((value) => save.mutate(value))}
        className="space-y-4"
      >
        {!!save.error && <ErrorBanner error={save.error} />}
        <label className="grid gap-1">
          {t("plans.planName")}
          <Input {...form.register("name")} disabled={save.isPending} />
        </label>
        <label className="grid gap-1">
          {t("plans.planId")}
          <Input {...form.register("planId")} disabled={save.isPending} />
        </label>
        {Object.values(form.formState.errors).map((error, index) => (
          <p role="alert" key={index}>
            {t(error.message ?? "plans.required")}
          </p>
        ))}
        <div className="grid gap-1">
          <label id="plan-mode-label">{t("plans.mode")}</label>
          <Select
            value={mode}
            disabled={save.isPending}
            onValueChange={(value) => {
              if (
                value === "direct" ||
                value === "workflow" ||
                value === "graph"
              ) {
                form.setValue("mode", value);
                form.setValue("ref", "");
                setSearch("");
              }
            }}
          >
            <SelectTrigger aria-labelledby="plan-mode-label">
              <SelectValue>{t(`plans.modes.${mode}`)}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              {(["direct", "workflow", "graph"] as const).map((value) => (
                <SelectItem
                  key={value}
                  value={value}
                  disabled={value === "graph" && !readOnly}
                >
                  {t(`plans.modes.${value}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        {mode !== "direct" && (
          <>
            <label className="grid gap-1">
              {t("plans.searchReference")}
              <Input
                value={search}
                disabled={save.isPending}
                onChange={(event) => {
                  setSearch(event.target.value);
                  form.setValue("ref", "");
                }}
              />
            </label>
            {!!references.error && <ErrorBanner error={references.error} />}
            <div className="grid gap-1">
              <label id="plan-reference-label">{t("plans.reference")}</label>
              <Select
                value={ref}
                disabled={save.isPending || references.isFetching}
                onValueChange={(value) => form.setValue("ref", value ?? "")}
              >
                <SelectTrigger aria-labelledby="plan-reference-label">
                  <SelectValue placeholder={t("plans.chooseReference")}>
                    {chosen
                      ? t("plans.namedVersion", {
                          name: chosen.name,
                          version: chosen.version,
                        })
                      : t("plans.chooseReference")}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {options.map((item) => (
                    <SelectItem key={item.ref} value={item.ref}>
                      {t("plans.namedVersion", {
                        name: item.name,
                        version: item.version,
                      })}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {!references.isFetching && !options.length && !references.error && (
              <p role="status">{t("plans.noReferences")}</p>
            )}
          </>
        )}
        <footer className="editor-footer">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={save.isPending}
          >
            {t("plans.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={save.isPending || (mode !== "direct" && !chosen)}
          >
            {t("plans.saveDraft")}
          </Button>
        </footer>
      </form>
    </EditorDialog>
  );
}
