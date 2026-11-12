import { useId, cloneElement, type ReactElement } from "react";
import { useTranslation } from "react-i18next";
import { useConfiguration } from "@/features/governance/runtime";
import { validationText } from "@/features/administration/shared";

export function FormField({
  label,
  error,
  children,
  wide = false,
}: {
  label: string;
  error?: string;
  children: ReactElement<{
    id?: string;
    "aria-invalid"?: boolean;
    "aria-describedby"?: string;
  }>;
  wide?: boolean;
}) {
  const { t } = useTranslation();
  const config = useConfiguration();
  const generated = useId();
  const id = children.props.id ?? generated;
  return (
    <div className={`editor-field ${wide ? "field-wide" : ""}`}>
      <label htmlFor={id}>{label}</label>
      {cloneElement(children, {
        id,
        "aria-invalid": !!error,
        "aria-describedby": error ? `${id}-error` : undefined,
      })}
      {error && (
        <small id={`${id}-error`} className="field-error" role="alert">
          {error === "passwordPolicy" || error === "password"
            ? t("governance:passwordPolicy", {
                count: Number(config.value("security.passwordMin", 12)),
              })
            : validationText(error, t)}
        </small>
      )}
    </div>
  );
}
