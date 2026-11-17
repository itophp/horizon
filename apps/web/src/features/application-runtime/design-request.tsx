import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { runtimeExecute } from "@/lib/capabilities";
import { ErrorBanner } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";

const schema = z.object({ request: z.string().min(1).max(4000) });

export function DesignRequest({
  applicationId,
  canGenerate,
  onStarted,
}: {
  applicationId: string;
  canGenerate: boolean;
  onStarted: (runId: number) => void;
}) {
  const { t } = useTranslation("runtime");
  const [error, setError] = useState<unknown>();
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: { request: "" },
  });
  return (
    <form
      className="admin-form design-request-form"
      onSubmit={form.handleSubmit(async (values) => {
        setError(undefined);
        try {
          const result = await runtimeExecute(
            "orchestration.workflow.run",
            "1.0.0",
            {
              flow: "application-runtime-designer",
              version: "1.0.0",
              input: { applicationId, request: values.request },
            },
            crypto.randomUUID(),
            crypto.randomUUID(),
          );
          if (result.status !== "succeeded")
            throw new Error(result.errorCode || "unavailable");
          const accepted = z
            .object({ runId: z.number().int().positive() })
            .parse(result.data);
          onStarted(accepted.runId);
        } catch (error) {
          setError(error);
        }
      })}
    >
      <label>
        {t("design.request")}
        <textarea
          className="editor-textarea"
          rows={6}
          placeholder={t("design.requestHint")}
          aria-invalid={!!form.formState.errors.request}
          disabled={!canGenerate || form.formState.isSubmitting}
          {...form.register("request")}
        />
      </label>
      {form.formState.errors.request && <p role="alert">{t("invalid")}</p>}
      {error !== undefined && <ErrorBanner error={error} />}
      <div className="design-request-actions">
        <Button
          type="submit"
          disabled={!canGenerate || form.formState.isSubmitting}
        >
          <Sparkles size={14} />
          {t("design.generate")}
        </Button>
        <small>
          {canGenerate ? t("design.generateHint") : t("design.forbidden")}
        </small>
      </div>
    </form>
  );
}
