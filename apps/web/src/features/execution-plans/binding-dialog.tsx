import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { executionPlans } from "@/lib/capabilities";
import { EditorDialog, ErrorBanner } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  selector,
  useOperationKey,
  type Target,
  type Source,
  type Binding,
} from "./state";
const reasonSchema = z.object({
  reason: z.string().trim().min(1, "plans.required").max(160, "plans.required"),
});
export function BindingDialog({
  target,
  source,
  binding,
  rollback,
  onClose,
  onSaved,
}: {
  target: Target;
  source: Source;
  binding: Binding;
  rollback: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation("capability");
  const keys = useOperationKey();
  const form = useForm<z.infer<typeof reasonSchema>>({
    resolver: zodResolver(reasonSchema),
    defaultValues: { reason: "" },
  });
  const change = useMutation({
    mutationFn: ({ reason }: { reason: string }) => {
      const input = {
        ...selector(target, source.plan),
        reason,
        expectedRevision: binding.revision,
      };
      return (rollback ? executionPlans.rollback : executionPlans.bind)(
        input,
        keys.get(input),
      );
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
        if (!open && !change.isPending) onClose();
      }}
      title={t(rollback ? "plans.rollback" : "plans.bind")}
      description={t("plans.bindingHint", {
        name: source.name,
        version: source.plan.version,
        revision: binding.revision,
      })}
    >
      <form
        className="space-y-4"
        onSubmit={form.handleSubmit((value) => change.mutate(value))}
      >
        {!!change.error && <ErrorBanner error={change.error} />}
        <label className="grid gap-1">
          {t("plans.reason")}
          <Textarea {...form.register("reason")} disabled={change.isPending} />
        </label>
        {form.formState.errors.reason && (
          <p role="alert">{t("plans.required")}</p>
        )}
        <footer className="editor-footer">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={change.isPending}
          >
            {t("plans.cancel")}
          </Button>
          <Button type="submit" disabled={change.isPending}>
            {t(rollback ? "plans.rollback" : "plans.bind")}
          </Button>
        </footer>
      </form>
    </EditorDialog>
  );
}
