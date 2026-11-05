import { useId, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EditorDialog } from "@/features/administration/shared";

const schema = z.object({ name: z.string().trim().min(2).max(80) });
type Values = z.infer<typeof schema>;

// Reference only. Supply translated labels and an authorized capability callback.
export function RecordEditor({
  open,
  onOpenChange,
  initialName,
  labels,
  onSave,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialName: string;
  labels: {
    title: string;
    description: string;
    name: string;
    invalidName: string;
    failed: string;
    save: string;
    cancel: string;
    saving: string;
  };
  onSave: (values: Values) => Promise<void>;
}) {
  const [failed, setFailed] = useState(false);
  const errorId = useId();
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: initialName },
  });
  return (
    <EditorDialog
      open={open}
      onOpenChange={(next) => {
        if (!isSubmitting) onOpenChange(next);
      }}
      title={labels.title}
      description={labels.description}
    >
      <form
        className="space-y-4"
        noValidate
        onSubmit={handleSubmit(async (values) => {
          setFailed(false);
          try {
            await onSave(values);
            onOpenChange(false);
          } catch {
            setFailed(true);
          }
        })}
      >
        <label className="block space-y-2">
          <span>{labels.name}</span>
          <Input
            {...register("name")}
            aria-invalid={!!errors.name}
            aria-describedby={errors.name ? errorId : undefined}
          />
        </label>
        {errors.name && (
          <p id={errorId} role="alert">
            {labels.invalidName}
          </p>
        )}
        {failed && <p role="alert">{labels.failed}</p>}
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={isSubmitting}
            onClick={() => onOpenChange(false)}
          >
            {labels.cancel}
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? labels.saving : labels.save}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
