import { ShieldCheck } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { EntityAvatar } from "@/components/shared/controls";
import { componentRegistry } from "./components";
import type { RuntimeFieldV2 } from "@/api/generated/runtime";
import type { Json } from "./expression";
import { useTranslation } from "react-i18next";

// Contract-referenced display adapters. The Go contract declares columns with
// component:"person"/"badge.list"/"status"; these are the only frontend
// implementations, registered once and resolved by the Runtime table.

function PersonDisplay({ value }: { value: Json | undefined }) {
  if (!value || typeof value !== "object") return <>{String(value ?? "")}</>;
  const record = value as Record<string, unknown>;
  return (
    <span className="entity-name">
      <EntityAvatar
        name={String(record.fullName ?? record.name ?? "")}
        seed={Number(record.id ?? 0)}
      />
      <span>
        <strong>{String(record.fullName ?? record.name ?? "")}</strong>
        {record.isAdmin === true && <ShieldCheck size={13} />}
        <small>
          {String(record.username ?? record.email ?? record.code ?? "")}
        </small>
      </span>
    </span>
  );
}

function BadgeListDisplay({
  value,
}: {
  definition: RuntimeFieldV2;
  value: Json | undefined;
}) {
  const { t } = useTranslation();
  if (!Array.isArray(value) || value.length === 0)
    return <span className="muted-text">{t("runtime:empty")}</span>;
  return (
    <span className="cell-tags">
      {value.map((item, i) => {
        if (typeof item === "string")
          return (
            <Badge variant="secondary" key={i}>
              {item}
            </Badge>
          );
        if (item && typeof item === "object") {
          const rec = item as Record<string, unknown>;
          return (
            <Badge variant="secondary" key={i}>
              {String(
                rec.name ??
                  rec.label ??
                  rec.departmentName ??
                  rec.fullName ??
                  "",
              )}
            </Badge>
          );
        }
        return null;
      })}
    </span>
  );
}

function StatusDisplay({
  value,
}: {
  definition: RuntimeFieldV2;
  value: Json | undefined;
}) {
  const { t } = useTranslation();
  const status = String(value ?? "");
  return (
    <Badge variant="secondary" className={`sc-status sc-${status}`}>
      {t(`admin:status.${status}`, { defaultValue: status })}
    </Badge>
  );
}

function TimeDisplay({
  value,
}: {
  definition: RuntimeFieldV2;
  value: Json | undefined;
}) {
  const { t } = useTranslation();
  if (!value) return <span className="muted-text">{t("runtime:empty")}</span>;
  const date = new Date(String(value));
  if (isNaN(date.getTime())) return <>{String(value)}</>;
  return <time>{date.toLocaleDateString()}</time>;
}

// Register display-only components (source "admin", referenced by Go contracts)
const displaySchema = { parse: () => true } as never;
for (const [name, display] of Object.entries({
  person: PersonDisplay,
  "badge.list": BadgeListDisplay,
  "status.display": StatusDisplay,
  time: TimeDisplay,
})) {
  try {
    componentRegistry.register({
      name,
      version: "2.0",
      source: "system.identity",
      schema: displaySchema,
      render: () => null,
      display,
    });
  } catch {
    // Skip duplicate registration during HMR re-execution
  }
}
