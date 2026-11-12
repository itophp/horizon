import { useTranslation } from "react-i18next";
import { RuntimeSelect } from "@/features/application-runtime/table-select";
import { Badge } from "@/components/ui/badge";
import type { ReactNode } from "react";
export function Choice({
  labelKey,
  value,
  options,
  onChange,
}: {
  labelKey: string;
  value: string;
  options: readonly string[];
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation("showcase");
  return (
    <RuntimeSelect
      label={t(labelKey)}
      value={value}
      options={options.map((value) => ({ value, label: t(value) }))}
      onChange={onChange}
    />
  );
}
export function Status({ value }: { value: string }) {
  const { t } = useTranslation("showcase");
  return (
    <Badge variant="secondary" className={`sc-status sc-${value}`}>
      {t(value)}
    </Badge>
  );
}
export function Panel({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <section className="sc-panel">
      <h2>{title}</h2>
      {children}
    </section>
  );
}
