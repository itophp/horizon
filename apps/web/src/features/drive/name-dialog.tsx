import { useMutation } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EditorDialog, ErrorBanner } from "@/features/administration/shared";

const schema = z.object({ name: z.string().trim().min(1).max(200) });
type Values = z.infer<typeof schema>;

export function NameDialog({
  title,
  initial,
  maxLength = 200,
  closeOnSuccess = true,
  onClose,
  onSave,
}: {
  title: string;
  initial?: string;
  maxLength?: number;
  closeOnSuccess?: boolean;
  onClose: () => void;
  onSave: (name: string) => Promise<unknown>;
}) {
  const { t } = useTranslation("drive");
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: initial ?? "" },
  });
  const mutation = useMutation({
    mutationFn: onSave,
    onSuccess: () => {
      if (closeOnSuccess) onClose();
    },
  });
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={title}
      description={t("editDescription")}
    >
      <form
        onSubmit={form.handleSubmit((value) => mutation.mutate(value.name))}
      >
        <label htmlFor="drive-name">{t("name")}</label>
        <Input
          id="drive-name"
          maxLength={maxLength}
          {...form.register("name")}
          aria-invalid={Boolean(form.formState.errors.name)}
        />
        {form.formState.errors.name && <p role="alert">{t("required")}</p>}
        {mutation.isError && <ErrorBanner error={mutation.error} />}
        <div className="editor-footer">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? t("pending") : t("save")}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
