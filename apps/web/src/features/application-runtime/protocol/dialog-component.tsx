import { useTranslation } from "react-i18next";
import { EditorDialog } from "@/features/administration/shared";
import type { ComponentRendererProps } from "./component-registry";

export function DialogComponent({
  props,
  children,
  emit,
  readOnly,
}: ComponentRendererProps) {
  const { t } = useTranslation();
  if (readOnly || props.open !== true || typeof props.titleKey !== "string")
    return null;

  return (
    <EditorDialog
      open
      title={t(props.titleKey, {
        replace: props.titleArgs as Record<string, unknown> | undefined,
      })}
      description={
        typeof props.descriptionKey === "string"
          ? t(props.descriptionKey, {
              replace: props.descriptionArgs as
                Record<string, unknown> | undefined,
            })
          : ""
      }
      wide={props.wide === true}
      onOpenChange={(open) => {
        if (!open) {
          void emit("close");
          void emit("openChange", false);
        }
      }}
    >
      {children}
    </EditorDialog>
  );
}
