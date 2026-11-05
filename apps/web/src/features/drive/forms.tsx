import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EditorDialog, ErrorBanner } from "@/features/administration/shared";
import { driveMemberAdd } from "@/lib/capabilities";
import { DriveSelect } from "./select";
const memberSchema = z.object({
  targetType: z.enum(["account", "department"]),
  targetId: z.number().int().positive(),
  role: z.enum(["admin", "editor", "viewer"]),
});
type Values = z.infer<typeof memberSchema>;

export function MemberForm({
  spaceId,
  onClose,
  onSaved,
}: {
  spaceId: number;
  onClose: () => void;
  onSaved: () => Promise<unknown>;
}) {
  const { t } = useTranslation("drive");
  const form = useForm<Values>({
    resolver: zodResolver(memberSchema),
    defaultValues: { targetType: "account", targetId: 0, role: "viewer" },
  });
  const add = useMutation({
    mutationFn: (value: Values) =>
      driveMemberAdd(spaceId, value.targetType, value.targetId, value.role),
    onSuccess: async () => {
      await onSaved();
      onClose();
    },
  });
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t("addMember")}
      description={t("editDescription")}
    >
      <form onSubmit={form.handleSubmit((value) => add.mutate(value))}>
        <label htmlFor="drive-member-type">{t("targetType")}</label>
        <Controller
          name="targetType"
          control={form.control}
          render={({ field }) => (
            <DriveSelect
              id="drive-member-type"
              label={t("targetType")}
              value={field.value}
              onValueChange={field.onChange}
              onBlur={field.onBlur}
              options={(["account", "department"] as const).map((value) => ({
                value,
                label: t(value),
              }))}
            />
          )}
        />
        <label htmlFor="drive-member-id">{t("targetId")}</label>
        <Input
          id="drive-member-id"
          type="number"
          min={1}
          {...form.register("targetId", { valueAsNumber: true })}
        />
        {form.formState.errors.targetId && <p role="alert">{t("required")}</p>}
        <label htmlFor="drive-member-role">{t("role")}</label>
        <Controller
          name="role"
          control={form.control}
          render={({ field }) => (
            <DriveSelect
              id="drive-member-role"
              label={t("role")}
              value={field.value}
              onValueChange={field.onChange}
              onBlur={field.onBlur}
              options={(["admin", "editor", "viewer"] as const).map(
                (value) => ({ value, label: t(`roles.${value}`) }),
              )}
            />
          )}
        />
        {add.isError && <ErrorBanner error={add.error} />}
        <div className="editor-footer">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button type="submit" disabled={add.isPending}>
            {t("save")}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
