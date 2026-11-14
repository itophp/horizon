import { contentRenderers } from "./shadcn-content";
import { fieldRenderers } from "./shadcn-fields";
import { overlayRenderers } from "./shadcn-overlays";
import { compositeRenderers } from "./shadcn-composites";
import type { ComponentProps, ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link, useSearchParams } from "react-router";
import { EntityAvatar } from "@/components/shared/controls";
import { useSession } from "@/features/administration/auth";
import {
  DataTable,
  ListSearch,
  PageHeading,
  ShortDate,
  StatusBadge,
  type AdminColumn,
} from "@/features/administration/shared";
import {
  ActionControl,
  isInternalHref,
  CardComponent,
  useContentFields,
  ContentFields,
  ContentHeading,
  ContentNotice,
  ContentTable,
  InlineInput,
} from "./content-components";
import {
  DictionaryPanel,
  RemoteMcpPanel,
  SettingsPanel,
  WebMcpPanel,
} from "./platform-panels";
import { AIAnalysisPanel, AIResultComponent } from "./ai-components";
import { ItemListComponent } from "./item-list";
import { ChangeListForm } from "./change-list-form";
import { ActionForm } from "./action-form";
import { BatchImport } from "./batch-import";
import { FileReviewForm } from "./file-review-form";
import { DialogComponent } from "./dialog-component";
import {
  ComponentRegistry,
  type ComponentRendererProps,
} from "./component-registry";
import { CalendarDays } from "lucide-react";
import { useConfiguration } from "@/features/governance/runtime";
import {
  DistributionPie,
  HorizontalBars,
  LayoutTabPanel,
  LayoutTabs,
  MetricValue,
  TimeSeries,
} from "./dashboard-components";
import { MarkdownDisplay } from "../markdown";
import { pathValue } from "../expression";
import type { ProtocolRecord, ProtocolTableColumn } from "./types";
import {
  CardGridPanelComponent,
  DetailPanelComponent,
  MasterDetailPanelComponent,
  PageTabsComponent,
  RecordFormPanelComponent,
  SearchPickerPanelComponent,
  TablePanelComponent,
  TreeTablePanelComponent,
} from "./admin-workbench";

const headingIcons = { plus: Plus } as const;

function HeadingIcon({ name, size = 15 }: { name?: string; size?: number }) {
  const Component = name
    ? headingIcons[name as keyof typeof headingIcons]
    : undefined;
  return Component ? <Component size={size} /> : null;
}

function text(value: unknown): string {
  return value === undefined || value === null ? "" : String(value);
}

function PageHeadingComponent({
  props,
  data,
  dataMap,
}: ComponentRendererProps) {
  const { field } = useContentFields(dataMap);
  const { t, i18n } = useTranslation(["runtime", "admin"]);
  const config = useConfiguration();
  const session = useSession();
  const [, setParams] = useSearchParams();
  const actions =
    (props.actions as {
      labelKey?: string;
      icon?: string;
      permission?: string;
      dialog?: string;
      href?: string;
    }[]) ?? [];
  return (
    <PageHeading
      title={t(text(props.titleKey))}
      subtitle={
        props.subtitleKey
          ? t(
              text(props.subtitleKey),
              props.subtitleAccountName === true
                ? { name: session.account.fullName }
                : {},
            )
          : ""
      }
    >
      {props.showCurrentDate === true && (
        <span className="date-chip">
          <CalendarDays size={14} aria-hidden />
          {new Intl.DateTimeFormat(i18n.resolvedLanguage, {
            year: "numeric",
            month: "short",
            day: "numeric",
            timeZone: String(config.value("display.timezone", "Asia/Shanghai")),
          }).format(new Date())}
        </span>
      )}
      {((props.badges ?? []) as Parameters<typeof field>[0][]).map(
        (badge, index) => (
          <span key={index}>{field(badge, data)}</span>
        ),
      )}
      {actions
        .filter(
          (action) =>
            !action.permission ||
            session.permissions.includes(action.permission),
        )
        .map((action) =>
          action.href && isInternalHref(action.href) ? (
            <Button
              key={action.labelKey}
              variant="outline"
              render={<Link to={action.href} />}
            >
              {t(text(action.labelKey))}
            </Button>
          ) : (
            <Button
              key={action.labelKey}
              type="button"
              onClick={() =>
                setParams((old) => {
                  const next = new URLSearchParams(old);
                  if (action.dialog) next.set("dialog", action.dialog);
                  return next;
                })
              }
            >
              <HeadingIcon name={action.icon} size={15} />
              {t(text(action.labelKey))}
            </Button>
          ),
        )}
    </PageHeading>
  );
}

function SearchComponent({
  props,
  data,
  readOnly,
  emit,
}: {
  props: Record<string, unknown>;
  data?: unknown;
  readOnly: boolean;
  emit: (event: string, payload?: unknown) => void;
}) {
  return (
    <ListSearch
      value={text(data ?? props.value)}
      onChange={(value) => {
        if (!readOnly) emit("change", value);
      }}
    />
  );
}

interface TableComponentProps {
  props: Record<string, unknown>;
  data?: unknown;
  readOnly: boolean;
  emit: (event: string, payload?: unknown) => void;
}

function DataTableComponent({
  props,
  data,
  readOnly,
  emit,
}: TableComponentProps) {
  const { t } = useTranslation(["runtime", "admin"]);
  const source = Array.isArray(props.items)
    ? props.items
    : Array.isArray(data)
      ? data
      : data &&
          typeof data === "object" &&
          Array.isArray((data as { items?: unknown }).items)
        ? ((data as { items: unknown[] }).items as unknown[])
        : [];
  const keyField = text(props.keyField) || "id";
  const columns = (Array.isArray(props.columns) ? props.columns : []) as {
    name: string;
    labelKey?: string;
    display?: string;
    width?: number;
    className?: string;
  }[];
  const rows = source.map((value, index) => {
    const record =
      value && typeof value === "object" ? (value as ProtocolRecord) : {};
    const rawId = record[keyField] ?? record.id ?? index;
    const numericId = Number(rawId);
    return {
      ...record,
      id: Number.isFinite(numericId) ? numericId : index,
    };
  });
  const adminColumns: AdminColumn<ProtocolRecord>[] = columns.map(
    (column: ProtocolTableColumn) => ({
      id: column.name,
      header: column.labelKey ? t(column.labelKey) : column.name,
      cell: (row) => renderCell(column, row, t),
      style: column.width ? { width: column.width } : undefined,
      className: column.className,
    }),
  );
  return (
    <DataTable
      rows={rows}
      columns={adminColumns}
      selectable={props.selectable === true}
      serverPage={props.serverPage === true}
      onRowClick={
        props.rowEvent && !readOnly
          ? (row) => emit(text(props.rowEvent), row)
          : undefined
      }
    />
  );
}

function renderCell(
  column: ProtocolTableColumn,
  row: ProtocolRecord,
  t: (key: string) => string,
): ReactNode {
  const value = row[column.name];
  switch (column.display) {
    case "entity": {
      const name = text(row.fullName ?? row.name ?? value);
      const secondary = text(row.username ?? row.email ?? row.code);
      return (
        <span className="entity-name">
          <EntityAvatar name={name} seed={Number(row.id ?? 0)} />
          <span>
            <strong>{name}</strong>
            {secondary && <small>{secondary}</small>}
          </span>
        </span>
      );
    }
    case "mono":
      return <span className="mono-muted">{text(value)}</span>;
    case "status":
      return <StatusBadge status={text(value)} />;
    case "date":
      return <ShortDate value={text(value)} />;
    case "badges":
      if (Array.isArray(value))
        return (
          <div className="cell-tags">
            {value.map((item, index) => (
              <span className="neutral-badge" key={index}>
                {text(
                  item && typeof item === "object"
                    ? ((item as ProtocolRecord).name ??
                        (item as ProtocolRecord).label)
                    : item,
                )}
              </span>
            ))}
          </div>
        );
      return <span className="muted-text">{t("protocol.empty")}</span>;
    default:
      return <>{text(value)}</>;
  }
}

function TextComponent({ props }: { props: Record<string, unknown> }) {
  const { t } = useTranslation("runtime");
  if (!props.textKey) return null;
  return (
    <span
      className={text(props.className)}
      role={
        props.role === "status" || props.role === "alert"
          ? props.role
          : undefined
      }
    >
      {t(text(props.textKey), props.values as Record<string, unknown>)}
    </span>
  );
}

function MarkdownComponent({
  props,
  data,
}: {
  props: Record<string, unknown>;
  data?: unknown;
}) {
  const { t } = useTranslation();
  const path = text(props.valuePath);
  return (
    <MarkdownDisplay
      value={
        typeof props.value === "string"
          ? props.value
          : props.textKey
            ? props.code === true
              ? "```\n" + t(text(props.textKey)) + "\n```"
              : t(text(props.textKey))
            : path
              ? pathValue(data, path)
              : data
      }
    />
  );
}

function ButtonComponent({
  props,
  data,
  dataMap,
  readOnly,
  emit,
}: {
  props: Record<string, unknown>;
  data?: unknown;
  dataMap?: Record<string, unknown>;
  readOnly: boolean;
  emit: (event: string, payload?: unknown) => void;
}) {
  const { t } = useTranslation(["runtime", "admin"]);
  if (props.action)
    return (
      <ActionControl
        action={props.action as Parameters<typeof ActionControl>[0]["action"]}
        record={data}
        dataMap={dataMap}
        readOnly={readOnly || props.disabled === true}
        emit={emit}
      />
    );
  if (readOnly && props.hideInPreview === true) return null;
  return (
    <Button
      type="button"
      variant={
        (text(props.variant) || "default") as ComponentProps<
          typeof Button
        >["variant"]
      }
      size={
        (text(props.size) || "default") as ComponentProps<typeof Button>["size"]
      }
      disabled={readOnly || props.disabled === true}
      onClick={() => emit("click", props.payload)}
    >
      {t(text(props.labelKey), {
        replace: props.labelArgs as Record<string, unknown> | undefined,
      })}
    </Button>
  );
}

export function createAdminRegistry(): ComponentRegistry {
  const registry = new ComponentRegistry();
  registry.register("page-heading", "1.0", PageHeadingComponent);
  registry.register("search", "1.0", SearchComponent);
  registry.register("data-table", "1.0", DataTableComponent);
  registry.register("text", "1.0", TextComponent);
  registry.register("ai-analysis-panel", "1.0", AIAnalysisPanel);
  registry.register("ai-result", "1.0", AIResultComponent);
  registry.register("table-panel", "1.0", TablePanelComponent);
  registry.register("tree-table-panel", "1.0", TreeTablePanelComponent);
  registry.register("card-grid-panel", "1.0", CardGridPanelComponent);
  registry.register("master-detail-panel", "1.0", MasterDetailPanelComponent);
  registry.register("page-tabs", "1.0", PageTabsComponent);
  registry.register("button", "1.0", ButtonComponent);
  registry.register("markdown", "1.0", MarkdownComponent);
  registry.register("detail-panel", "1.0", DetailPanelComponent);
  registry.register("record-form-panel", "1.0", RecordFormPanelComponent);
  registry.register("search-picker-panel", "1.0", SearchPickerPanelComponent);
  registry.register("item-list", "1.0", ItemListComponent);
  registry.register("card", "1.0", CardComponent);
  registry.register("dialog", "1.0", DialogComponent);
  registry.register("action-form", "1.0", ActionForm);
  registry.register("batch-import", "1.0", BatchImport);
  registry.register("file-review-form", "1.0", FileReviewForm);
  registry.register("change-list-form", "1.0", ChangeListForm);
  registry.register("metric-value", "1.0", MetricValue);
  registry.register("time-series", "1.0", TimeSeries);
  registry.register("horizontal-bars", "1.0", HorizontalBars);
  registry.register("distribution-pie", "1.0", DistributionPie);
  registry.register("settings-panel", "1.0", SettingsPanel);
  registry.register("dictionary-panel", "1.0", DictionaryPanel);
  registry.register("remote-mcp-panel", "1.0", RemoteMcpPanel);
  registry.register("webmcp-panel", "1.0", WebMcpPanel);
  registry.register("layout-tabs", "1.0", LayoutTabs);
  registry.register("layout-tab-panel", "1.0", LayoutTabPanel);
  registry.register("content-heading", "1.0", ContentHeading);
  registry.register("content-fields", "1.0", ContentFields);
  registry.register("content-notice", "1.0", ContentNotice);
  registry.register("content-table", "1.0", ContentTable);
  registry.register("inline-input", "1.0", InlineInput);
  for (const [name, renderer] of Object.entries({
    ...contentRenderers,
    ...fieldRenderers,
    ...overlayRenderers,
    ...compositeRenderers,
  })) {
    registry.register(name, "1.0", renderer);
  }
  return registry;
}
