import { useEffect, useRef } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { RuntimePage } from "@/api/generated/runtime";
import { runtimeSave } from "@/lib/capabilities";
import { ApiError } from "@/api/client/request";
import { FormField } from "@/features/administration/forms";
import { ErrorBanner } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { components, Context, useRuntimeContext } from "./registry";
import type { RuntimeRecord } from "./records";

export function RuntimeForm({
  page,
  record,
  onSaved,
  onCancel,
  onBusyChange,
}: {
  page: RuntimePage;
  record?: RuntimeRecord;
  onSaved: () => Promise<void>;
  onCancel: () => void;
  onBusyChange?: (busy: boolean) => void;
}) {
  const { t } = useTranslation("articles");
  const context = useRuntimeContext();
  const shape: Record<string, z.ZodString> = {};
  for (const f of page.fields.filter((f) => !f.readOnly && !f.hidden)) {
    let s = z.string().trim();
    if (f.required) s = s.min(1);
    if (f.maxLength) s = s.max(f.maxLength);
    shape[f.name] = s;
  }
  const form = useForm<Record<string, string>>({
    resolver: zodResolver(z.object(shape)),
    defaultValues: Object.fromEntries(
      page.fields.map((f) => [f.name, String(record?.[f.name] ?? "")]),
    ),
  });
  const attempt = useRef<{ payload: string; key: string } | null>(null);
  const mutation = useMutation({
    mutationFn: async (values: Record<string, string>) => {
      const input: Record<string, unknown> = {};
      for (const f of page.fields)
        if (!f.readOnly && !f.hidden) input[f.name] = values[f.name];
      if (record) {
        input.id = record.id;
        input.revision = record.revision;
      }
      const payload = JSON.stringify(input);
      if (attempt.current?.payload !== payload)
        attempt.current = { payload, key: crypto.randomUUID() };
      const result = await runtimeSave(
        record ? page.updateCapability : page.createCapability,
        input,
        attempt.current.key,
      );
      if (result.status !== "succeeded")
        throw new ApiError("approval_required", 409);
    },
    onSuccess: onSaved,
  });
  useEffect(() => {
    onBusyChange?.(mutation.isPending);
    return () => onBusyChange?.(false);
  }, [mutation.isPending, onBusyChange]);
  useEffect(() => {
    const guard = (event: BeforeUnloadEvent) => {
      if (form.formState.isDirty) {
        event.preventDefault();
        event.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [form.formState.isDirty]);
  const cancel = () => {
    if (!form.formState.isDirty || window.confirm(t("runtime.discard")))
      onCancel();
  };
  return (
    <Context.Provider value={context ? { ...context, pageId: page.id } : null}>
      <form
        className="entity-form"
        noValidate
        onSubmit={form.handleSubmit((v) => {
          if (!mutation.isPending) mutation.mutate(v);
        })}
      >
        {mutation.isError && <ErrorBanner error={mutation.error} />}
        {page.fields
          .filter((f) => !f.hidden)
          .map((f) => {
            const Control = components[f.component as keyof typeof components];
            if (!Control) throw new Error("component_not_registered");
            return (
              <Controller
                key={f.name}
                name={f.name}
                control={form.control}
                render={({ field }) => (
                  <FormField
                    label={t(f.labelKey)}
                    error={
                      form.formState.errors[f.name] ? t("invalid") : undefined
                    }
                    wide
                  >
                    <Control {...field} field={f} />
                  </FormField>
                )}
              />
            );
          })}
        <div className="editor-footer">
          <Button
            type="button"
            variant="outline"
            onClick={cancel}
            disabled={mutation.isPending}
          >
            {t("runtime.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={
              mutation.isPending ||
              !(record ? page.updateCapability : page.createCapability)
            }
          >
            {t("save")}
          </Button>
        </div>
      </form>
    </Context.Provider>
  );
}
