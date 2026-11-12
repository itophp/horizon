import { useMemo, useState, type ReactNode } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import {
  Building,
  Building2,
  Eye,
  KeyRound,
  LayoutGrid,
  List,
  Network,
  Pencil,
  Plus,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CheckCircle2,
  FileText,
  Play,
  RefreshCw,
  ShieldCheck,
  Trash2,
  Users,
  XCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Controller, useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { EntityAvatar } from "@/components/shared/controls";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  ListSearch,
  RowActions,
  ShortDate,
  StatusBadge,
  EmptyState,
  SuccessNotice,
  ScopeLabel,
  type AdminColumn,
} from "@/features/administration/shared";
import { useSession } from "@/features/administration/auth";
import { useConfiguration } from "@/features/governance/runtime";
import { PermissionMatrix } from "@/features/administration/permission-matrix";
import { Checkbox } from "@/components/ui/checkbox";
import type {
  DepartmentRecord,
  PermissionDefinition,
  PermissionGrant,
} from "@/api/generated/models";
import { runtimeExecute, runtimeRead } from "@/lib/capabilities";
import { ApiError } from "@/api/client/request";
import { RegisteredControl } from "../components";
import { MarkdownDisplay } from "../markdown";
import { evaluate, mapInput, type Json } from "../expression";
import { formSchema } from "../form-schema";
import { FormField } from "@/components/shared/form-field";
import type {
  RuntimeFormV2,
  RuntimeFieldV2,
  RuntimeMapping,
} from "@/api/generated/runtime";
import { pathValue } from "../expression";
import { useProtocolRuntime } from "./protocol-page";
import { coerceProviderInput } from "./data-resolver";
import { ProtocolErrorDialog } from "./error-dialog";
import type { ComponentRendererProps } from "./component-registry";
import type { ProtocolRecord } from "./types";

type Translator = (key: string, values?: Record<string, unknown>) => string;

function useProtocolTranslation(): Translator {
  const { i18n } = useTranslation();
  return (key, values) =>
    i18n.t(key, { defaultValue: key, ...values }) as string;
}

function text(value: unknown): string {
  return value === undefined || value === null ? "" : String(value);
}

function records(value: unknown): ProtocolRecord[] {
  return Array.isArray(value)
    ? value.filter(
        (item): item is ProtocolRecord => !!item && typeof item === "object",
      )
    : [];
}

function dataset(data: unknown, path: string): ProtocolRecord[] {
  return records(path ? pathValue(data, path) : data);
}

function label(
  t: Translator,
  item: ProtocolRecord,
  configuration: { field?: string; fallbackField?: string },
  references: Record<string, ProtocolRecord[]>,
  referenceName?: string,
): string {
  const direct = configuration.field ? item[configuration.field] : undefined;
  if (direct !== undefined && direct !== null && direct !== "")
    return text(direct);
  const id = configuration.fallbackField
    ? item[configuration.fallbackField]
    : undefined;
  const found = referenceName
    ? references[referenceName]?.find((row) => String(row.id) === String(id))
    : undefined;
  return found ? text(found.name ?? found.code) : t("admin:fields.unassigned");
}

const icons = {
  building: Building,
  building2: Building2,
  eye: Eye,
  key: KeyRound,
  grid: LayoutGrid,
  list: List,
  network: Network,
  pencil: Pencil,
  shield: ShieldCheck,
  trash: Trash2,
  plus: Plus,
  users: Users,
  file: FileText,
  check: CheckCircle2,
  reject: XCircle,
  play: Play,
  refresh: RefreshCw,
} as const;

function Icon({ name, size = 14 }: { name?: string; size?: number }) {
  const Component =
    name && name in icons ? icons[name as keyof typeof icons] : undefined;
  return Component ? <Component size={size} /> : null;
}

function RuntimeFormDialog({
  definition,
  mode,
  record,
  optionsData,
  titleKey,
  descriptionKey,
  wide,
  onClose,
}: {
  definition: RuntimeFormV2;
  mode: string;
  record?: ProtocolRecord;
  optionsData?: unknown;
  titleKey: string;
  descriptionKey: string;
  wide?: boolean;
  onClose: () => void;
}) {
  const t = useProtocolTranslation();
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t(titleKey)}
      description={t(descriptionKey)}
      wide={wide}
    >
      <ProtocolRecordForm
        definition={definition}
        mode={mode}
        record={record}
        optionsData={optionsData}
        onClose={onClose}
      />
    </EditorDialog>
  );
}

export function TablePanelComponent({
  props,
  data,
  dataMap,
  readOnly,
  emit,
  renderRowActions,
  onOpenRow,
}: ComponentRendererProps & {
  renderRowActions?: (row: ProtocolRecord) => ReactNode;
  onOpenRow?: (row: ProtocolRecord) => void;
}) {
  const t = useProtocolTranslation();
  const session = useSession();
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const route = useParams();
  const [deleting, setDeleting] = useState<ProtocolRecord | null>(null);
  const [loadedRecords, setLoadedRecords] = useState<
    Record<string, ProtocolRecord>
  >({});
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [notice, setNotice] = useState("");
  const [actionError, setActionError] = useState<unknown>(null);
  const rows = dataset(data, text(props.rowsPath));
  const references = useMemo(() => {
    const result: Record<string, ProtocolRecord[]> = {};
    const configured = (props.references ?? {}) as Record<string, string>;
    for (const [name, path] of Object.entries(configured))
      result[name] = [
        ...dataset(data, path),
        ...dataset(dataMap, path).filter(
          (row) =>
            !dataset(data, path).some(
              (existing) => String(existing.id) === String(row.id),
            ),
        ),
      ];
    return result;
  }, [data, dataMap, props.references]);
  const searchParam = text(props.searchParam) || "q";
  const search = params.get(searchParam) ?? "";
  const searchFields = (props.searchFields as string[]) ?? [];
  const tabConfig = props.tabs as
    | {
        param?: string;
        field?: string;
        options?: { value: string; labelKey?: string }[];
      }
    | undefined;
  const tabParam = tabConfig?.param ?? "";
  const defaultTabValue = tabConfig?.options?.some(
    (option) => option.value === "all",
  )
    ? "all"
    : "";
  const tabValue = params.get(tabParam) || defaultTabValue;
  const selectConfig = props.select as
    | {
        param?: string;
        field?: string;
        options?: { value: string; labelKey?: string }[];
      }
    | undefined;
  const selectParam = selectConfig?.param ?? "";
  const selectValue = selectParam ? (params.get(selectParam) ?? "") : "";
  const serverPage = props.serverPage === true;
  const pageParam = text(props.pageParam) || "page";
  const pageSizeParam = text(props.pageSizeParam) || "pageSize";
  const defaultPageSize = Number(props.defaultPageSize ?? 20) || 20;
  const page = Math.max(1, Number(params.get(pageParam) ?? 1) || 1);
  const pageSize = Math.min(
    100,
    Math.max(
      1,
      Number(params.get(pageSizeParam) ?? defaultPageSize) || defaultPageSize,
    ),
  );
  const total = Number(
    pathValue(data, text(props.totalPath) || "total") ?? rows.length,
  );

  function changeParam(key: string, value: string) {
    setParams((old) => {
      const next = new URLSearchParams(old);
      if (value && value !== "all") next.set(key, value);
      else next.delete(key);
      if (serverPage && key !== pageParam) next.delete(pageParam);
      return next;
    });
  }

  function changePage(nextPage: number) {
    if (!serverPage) return;
    setParams((old) => {
      const next = new URLSearchParams(old);
      if (nextPage > 1) next.set(pageParam, String(nextPage));
      else next.delete(pageParam);
      if (pageSize !== defaultPageSize)
        next.set(pageSizeParam, String(pageSize));
      else next.delete(pageSizeParam);
      return next;
    });
  }

  const parentField = text(props.parentField) || "parentId";
  const matches = (row: ProtocolRecord) => {
    const tabOk =
      !tabConfig?.field ||
      !tabValue ||
      tabValue === "all" ||
      text(row[tabConfig.field]) === tabValue;
    const selectOk =
      !selectConfig?.field ||
      !selectValue ||
      selectValue === "all" ||
      text(row[selectConfig.field]) === selectValue;
    const searchOk =
      serverPage && props.serverSearch === true
        ? true
        : !searchFields.length ||
          searchFields.some((field) =>
            text(row[field]).toLowerCase().includes(search.toLowerCase()),
          );
    return tabOk && selectOk && searchOk;
  };
  const flattened = rows.filter(matches);
  const hierarchical: ProtocolRecord[] = [];
  if (props.hierarchical === true) {
    const append = (parent: number, depth: number) => {
      for (const row of rows.filter(
        (item) => Number(item[parentField]) === parent,
      )) {
        const value = { ...row, depth };
        if (matches(row)) hierarchical.push(value);
        if (!collapsed.has(Number(row.id)) || search)
          append(Number(row.id), depth + 1);
      }
    };
    append(0, 0);
    for (const row of rows) {
      if (
        !hierarchical.some((item) => String(item.id) === String(row.id)) &&
        !rows.some((parent) => String(parent.id) === String(row[parentField]))
      ) {
        if (matches(row)) hierarchical.push({ ...row, depth: 0 });
        append(Number(row.id), 1);
      }
    }
  }
  const filtered = props.hierarchical === true ? hierarchical : flattened;
  const keyField = text(props.keyField) || "id";
  const tableRows = filtered.map((row, index) => {
    const value = Number(row[keyField] ?? row.id ?? index);
    return { ...row, id: Number.isFinite(value) ? value : index };
  });
  const columns = (props.columns as WorkbenchColumn[]) ?? [];
  function toggleCollapse(id: number) {
    setCollapsed((old) => {
      const next = new Set(old);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }
  const tableColumns: AdminColumn<ProtocolRecord>[] = columns.map((column) => ({
    id: column.name,
    header: column.labelKey ? t(column.labelKey) : column.name,
    className: column.className,
    style: column.width ? { width: column.width } : undefined,
    cell:
      onOpenRow && column.display === "entity-link"
        ? (row) => (
            <button
              type="button"
              className="entity-link"
              onClick={() => onOpenRow(row)}
            >
              <strong>{text(row[column.name])}</strong>
              {column.subtitleField && (
                <small>{text(row[column.subtitleField])}</small>
              )}
            </button>
          )
        : column.display === "account"
          ? (row) => {
              const subtitleField =
                column.subtitleField ?? column.fallbackField ?? "";
              const name = text(row[column.titleField ?? "name"]);
              return (
                <button
                  type="button"
                  className="account-cell"
                  onClick={() =>
                    setParams((old) => {
                      const next = new URLSearchParams(old);
                      next.set("dialog", "view");
                      next.set("id", String(row[keyField]));
                      return next;
                    })
                  }
                >
                  <EntityAvatar name={name} seed={Number(row.id ?? 0)} />
                  <span>
                    <strong>
                      {name}
                      {column.flagField && row[column.flagField] === true ? (
                        <ShieldCheck size={13} />
                      ) : null}
                    </strong>
                    <small>{text(row[subtitleField])}</small>
                  </span>
                </button>
              );
            }
          : column.display === "tree"
            ? (row) => {
                const hasChildren = rows.some(
                  (item) => String(item[parentField]) === String(row.id),
                );
                return (
                  <div
                    className="department-name"
                    style={{ paddingLeft: Number(row.depth ?? 0) * 19 }}
                  >
                    {hasChildren ? (
                      <button
                        aria-label={
                          collapsed.has(Number(row.id))
                            ? t("admin:actions.expand")
                            : t("admin:actions.collapseDepartment")
                        }
                        onClick={() => toggleCollapse(Number(row.id))}
                      >
                        {collapsed.has(Number(row.id)) ? (
                          <ChevronRight size={13} />
                        ) : (
                          <ChevronDown size={13} />
                        )}
                      </button>
                    ) : (
                      <span className="tree-spacer" />
                    )}
                    <span className="entity-symbol">
                      <Icon name={column.icon || "network"} size={15} />
                    </span>
                    <span>
                      <strong>{text(row[column.name])}</strong>
                      {column.subtitleField ? (
                        <small>{text(row[column.subtitleField])}</small>
                      ) : null}
                    </span>
                  </div>
                );
              }
            : (row) => renderWorkbenchCell(column, row, references, t),
  }));
  const actionConfig = (props.rowActions as WorkbenchAction[]) ?? [];
  const optionsData = useMemo(
    () =>
      data && typeof data === "object" && !Array.isArray(data)
        ? { ...(data as ProtocolRecord), ...dataMap }
        : { ...dataMap },
    [data, dataMap],
  );
  const dialog = params.get("dialog");
  const dialogId = Number(params.get("id")) || 0;
  const selected =
    loadedRecords[String(dialogId)] ??
    rows.find((row) => Number(row[keyField]) === dialogId);
  const forms = (props.forms ?? {}) as Record<string, RuntimeFormV2>;
  const dialogMode =
    dialog && ["create", "edit", "access", "view"].includes(dialog)
      ? dialog
      : undefined;
  const activeDefinition =
    dialogMode && dialogMode !== "create" ? forms[dialogMode] : forms.create;
  const formMeta = (props.dialogTitles ?? {}) as Record<
    string,
    { titleKey: string; descriptionKey: string; wide?: boolean }
  >;
  const can = (value?: string) => !value || session.permissions.includes(value);

  function closeDialog() {
    setParams((old) => {
      const next = new URLSearchParams(old);
      next.delete("dialog");
      next.delete("id");
      return next;
    });
  }

  async function remove() {
    const configuration = props.delete as
      { capability?: string; version?: string; idField?: string } | undefined;
    if (!deleting || !configuration?.capability || readOnly) return;
    try {
      const result = await runtimeExecute(
        configuration.capability,
        configuration.version || "1.0.0",
        {
          [configuration.idField || "id"]:
            deleting[configuration.idField || "id"],
        },
        crypto.randomUUID(),
        crypto.randomUUID(),
      );
      if (
        result.status !== "succeeded" &&
        result.status !== "awaiting_approval"
      )
        throw new ApiError(result.errorCode || "internal", 400);
      await client.invalidateQueries({ queryKey: ["protocol-v3"] });
      setDeleting(null);
      setNotice(t("admin:feedback.deleted"));
    } catch (cause) {
      setActionError(cause);
      setDeleting(null);
    }
  }

  const heading = props.panelHeading as
    { titleKey?: string; icon?: string } | undefined;

  return (
    <section className={text(props.className) || "admin-panel"}>
      {heading && (
        <div className="panel-heading">
          <div>
            <h2>
              {text(props.panelHeadingTitle) || t(heading.titleKey ?? "")}
            </h2>
            <p>{t(text(props.totalKey), { count: filtered.length })}</p>
          </div>
          <Icon name={heading.icon} size={18} />
        </div>
      )}
      {tabConfig?.options && (
        <div className="panel-tabs">
          {tabConfig.options.map((option) => (
            <button
              key={option.value}
              type="button"
              aria-pressed={tabValue === option.value}
              onClick={() => changeParam(tabParam, option.value)}
            >
              {option.labelKey ? t(option.labelKey) : option.value}
              <span>
                {
                  rows.filter(
                    (row) =>
                      option.value === "all" ||
                      (tabConfig.field &&
                        text(row[tabConfig.field]) === option.value),
                  ).length
                }
              </span>
            </button>
          ))}
        </div>
      )}
      <div className="table-toolbar">
        {props.search !== false && (
          <ListSearch
            value={search}
            onChange={(value) => changeParam(searchParam, value)}
          />
        )}
        <div className="toolbar-right">
          {Boolean(props.totalKey) && !heading && (
            <span className="muted-text">
              {t(props.totalKey as string, { count: filtered.length })}
            </span>
          )}
          {selectConfig?.options && (
            <select
              className="toolbar-select"
              aria-label={t("admin:fields.status")}
              value={selectValue}
              onChange={(event) => changeParam(selectParam, event.target.value)}
            >
              {selectConfig.options.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.labelKey ? t(option.labelKey) : option.value}
                </option>
              ))}
            </select>
          )}
          {Boolean(props.createKey) &&
            can(props.createPermission as string) &&
            !readOnly && (
              <Button
                onClick={() =>
                  setParams((old) => {
                    const next = new URLSearchParams(old);
                    next.set("dialog", "create");
                    return next;
                  })
                }
              >
                <Icon name={text(props.createIcon) || "plus"} size={15} />
                {t(props.createKey as string)}
              </Button>
            )}
        </div>
      </div>
      <DataTable
        rows={tableRows}
        columns={[
          ...tableColumns,
          ...(renderRowActions && !readOnly
            ? [
                {
                  id: "custom-actions",
                  header: t("admin:fields.actions"),
                  cell: renderRowActions,
                },
              ]
            : []),
          ...(actionConfig.length
            ? [
                {
                  id: "actions",
                  header: t("admin:fields.actions"),
                  className: "actions-column",
                  cell: (row: ProtocolRecord) => (
                    <RowActions
                      actions={actionConfig
                        .filter((action) =>
                          workbenchActionVisible(action, row, session),
                        )
                        .map((action) => ({
                          label: t(action.labelKey),
                          icon: <Icon name={action.icon} />,
                          danger: action.mode === "delete",
                          onClick: () => {
                            if (action.mode === "delete") {
                              setDeleting(row);
                              return;
                            }
                            if (
                              action.mode === "edit" &&
                              text(props.detailCapability)
                            ) {
                              const detailInput = mapInput(
                                (props.detailInput ?? []) as RuntimeMapping[],
                                {
                                  record: row as Record<string, Json>,
                                  route: route as Record<string, Json>,
                                  query: {},
                                  context: {},
                                },
                              );
                              void runtimeRead(
                                text(props.detailCapability),
                                detailInput,
                                undefined,
                                text(props.detailVersion) || "1.0.0",
                              )
                                .then((detail) => {
                                  if (
                                    detail &&
                                    typeof detail === "object" &&
                                    !Array.isArray(detail)
                                  ) {
                                    const record = detail as ProtocolRecord;
                                    setLoadedRecords((old) => ({
                                      ...old,
                                      [String(row[keyField])]: record,
                                    }));
                                    setParams((old) => {
                                      const next = new URLSearchParams(old);
                                      next.set("dialog", action.mode);
                                      next.set("id", String(row[keyField]));
                                      return next;
                                    });
                                  }
                                })
                                .catch((cause) => setActionError(cause));
                              return;
                            }
                            if (action.mode === "event" && action.event) {
                              emit(action.event, row);
                              return;
                            }
                            setParams((old) => {
                              const next = new URLSearchParams(old);
                              next.set("dialog", action.mode);
                              next.set("id", String(row[keyField]));
                              return next;
                            });
                          },
                        }))}
                    />
                  ),
                },
              ]
            : []),
        ]}
        selectable={props.selectable === true}
        serverPage={serverPage}
      />
      {serverPage && (
        <div className="table-pagination">
          <span>
            {t("admin:feedback.rows", {
              from: total ? (page - 1) * pageSize + 1 : 0,
              to: Math.min(page * pageSize, total),
              total,
            })}
          </span>
          <div>
            <span>
              {t("admin:feedback.page", {
                page,
                total: Math.max(1, Math.ceil(total / pageSize)),
              })}
            </span>
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              disabled={page <= 1}
              aria-label={t("admin:actions.previous")}
              onClick={() => changePage(page - 1)}
            >
              <ChevronLeft size={14} />
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              disabled={page >= Math.ceil(total / pageSize)}
              aria-label={t("admin:actions.next")}
              onClick={() => changePage(page + 1)}
            >
              <ChevronRight size={14} />
            </Button>
          </div>
        </div>
      )}
      {dialogMode && activeDefinition && formMeta[dialogMode] && (
        <RuntimeFormDialog
          definition={
            dialogMode === "access" && forms.access
              ? forms.access
              : activeDefinition
          }
          mode={dialogMode}
          record={dialogMode === "create" ? undefined : selected}
          optionsData={optionsData}
          titleKey={formMeta[dialogMode].titleKey}
          descriptionKey={formMeta[dialogMode].descriptionKey}
          wide={formMeta[dialogMode].wide}
          onClose={closeDialog}
        />
      )}
      {deleting && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setDeleting(null);
          }}
          title={t("admin:feedback.deleteTitle")}
          description={t("admin:feedback.deleteDescription", {
            name: text(
              deleting[props.deleteLabelField as string] ?? deleting.name,
            ),
          })}
        >
          <div className="editor-footer">
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleting(null)}
            >
              {t("admin:actions.cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => void remove()}
            >
              {t("admin:actions.delete")}
            </Button>
          </div>
        </EditorDialog>
      )}
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
      {actionError !== null && (
        <ProtocolErrorDialog
          error={actionError}
          onClose={() => setActionError(null)}
        />
      )}
    </section>
  );
}

export function SearchPickerPanelComponent({
  props,
  dataMap,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const t = useProtocolTranslation();
  const session = useSession();
  const runtime = useProtocolRuntime();
  const route = useParams();
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [results, setResults] = useState<ProtocolRecord[]>([]);
  const [searched, setSearched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  const providerId = text(props.provider);
  const provider = runtime.application.providers.find(
    (item) => item.id === providerId,
  );
  const existingPath = text(props.existingPath);
  const existingKey = text(props.existingKeyField) || "id";
  const existingIds = new Set(
    dataset(dataMap, existingPath).map((row) => String(row[existingKey])),
  );
  const resultKey = text(props.resultKeyField) || "id";
  const resultTitle = text(props.resultTitleField) || "name";
  const resultSubtitle = text(props.resultSubtitleField) || "email";
  const addEvent = text(props.addEvent);

  async function search() {
    const value = keyword.trim();
    if (!provider || value === "") return;
    setBusy(true);
    setError(undefined);
    try {
      const input = coerceProviderInput(
        mapInput((props.searchInput ?? []) as RuntimeMapping[], {
          route: route as Record<string, Json>,
          event: { keyword: value } as unknown as Record<string, Json>,
          query: {},
          context: {},
        }),
        provider.inputSchema,
      );
      const raw = await runtimeRead(
        provider.capability.name,
        input,
        undefined,
        provider.capability.version,
      );
      const path = provider.resultPath || "items";
      setResults(records(provider.resultPath ? pathValue(raw, path) : raw));
      setSearched(true);
    } catch (cause) {
      setError(cause);
    } finally {
      setBusy(false);
    }
  }

  const tableProps = (props.table ?? {}) as Record<string, unknown>;
  return (
    <>
      <div className="table-toolbar">
        <p className="muted-text">{t(text(props.descriptionKey))}</p>
        <div className="toolbar-right">
          {text(props.addLabelKey) &&
            !readOnly &&
            (!text(props.addPermission) ||
              session.permissions.includes(text(props.addPermission))) && (
              <Button type="button" onClick={() => setOpen(true)}>
                <Icon name={text(props.addIcon) || "plus"} size={15} />
                {t(text(props.addLabelKey))}
              </Button>
            )}
        </div>
      </div>
      <TablePanelComponent
        props={tableProps}
        data={dataMap}
        dataMap={dataMap}
        readOnly={readOnly}
        emit={emit}
      />
      {open && (
        <EditorDialog
          open
          onOpenChange={(next) => {
            if (!next) {
              setOpen(false);
              setResults([]);
              setSearched(false);
              setKeyword("");
              setError(undefined);
            }
          }}
          title={t(text(props.addLabelKey))}
          description={t(text(props.descriptionKey))}
        >
          <div className="entity-form">
            {error !== undefined && <ErrorBanner error={error} />}
            <form
              className="editor-grid"
              onSubmit={(event) => {
                event.preventDefault();
                void search();
              }}
            >
              <FormField label={t(text(props.searchLabelKey))}>
                <div className="member-search">
                  <Input
                    aria-label={t(text(props.searchLabelKey))}
                    value={keyword}
                    onChange={(event) => setKeyword(event.target.value)}
                  />
                  <Button
                    type="submit"
                    variant="outline"
                    disabled={busy || keyword.trim() === ""}
                  >
                    {busy
                      ? t("admin:actions.saving")
                      : t(text(props.searchButtonKey))}
                  </Button>
                </div>
              </FormField>
            </form>
            <div>
              {results.map((row) => {
                const id = String(row[resultKey]);
                const already = existingIds.has(id);
                return (
                  <div className="member-result" key={id}>
                    <span>
                      <strong>{text(row[resultTitle])}</strong>
                      <small>{text(row[resultSubtitle])}</small>
                    </span>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={already || readOnly || !addEvent}
                      onClick={() => {
                        emit(addEvent, row);
                        setOpen(false);
                        setResults([]);
                        setSearched(false);
                        setKeyword("");
                      }}
                    >
                      {t(
                        already
                          ? text(props.alreadyLabelKey)
                          : text(props.addLabelKey),
                      )}
                    </Button>
                  </div>
                );
              })}
              {searched && results.length === 0 && (
                <p className="muted-text">{t(text(props.emptyKey))}</p>
              )}
            </div>
            <div className="editor-footer">
              <Button
                type="button"
                variant="outline"
                onClick={() => setOpen(false)}
              >
                {t("admin:actions.close")}
              </Button>
            </div>
          </div>
        </EditorDialog>
      )}
    </>
  );
}

type WorkbenchCondition = RuntimeFormV2["create"]["when"];

type DetailField = {
  name: string;
  labelKey?: string;
  display?: string;
  emptyKey?: string;
  valueLabelPrefix?: string;
};

type DetailSection = {
  titleKey?: string;
  fields?: DetailField[];
  className?: string;
};

type DetailAction = {
  labelKey: string;
  icon?: string;
  variant?: "default" | "outline" | "ghost" | "destructive";
  permission?: string;
  when?: WorkbenchCondition;
  mode?: "event" | "dialog";
  dialog?: string;
  event?: string;
  confirmKey?: string;
};

function renderDetailValue(
  field: DetailField,
  record: ProtocolRecord,
  t: Translator,
): ReactNode {
  const value = record[field.name];
  switch (field.display) {
    case "status":
      return <StatusBadge status={text(value)} />;
    case "date":
      return value ? (
        <ShortDate value={text(value)} />
      ) : (
        <>{t(field.emptyKey || "protocol.empty")}</>
      );
    case "mono":
      return <span className="mono-muted">{text(value)}</span>;
    case "enumBadge":
      return (
        <span className="neutral-badge">
          {t((field.valueLabelPrefix ?? "") + text(value))}
        </span>
      );
    case "markdown":
      return value ? (
        <MarkdownDisplay value={value} />
      ) : (
        <span className="muted-text">
          {t(field.emptyKey || "protocol.empty")}
        </span>
      );
    case "badges":
      return (
        <div className="cell-tags">
          {records(value).map((item, index) => (
            <span className="neutral-badge" key={index}>
              {text(item.name ?? item.label ?? item)}
            </span>
          ))}
        </div>
      );
    default:
      return value === undefined || value === null || value === ""
        ? t(field.emptyKey || "protocol.empty")
        : text(value);
  }
}

export function DetailPanelComponent({
  props,
  data,
  dataMap,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const t = useProtocolTranslation();
  const session = useSession();
  const [params, setParams] = useSearchParams();
  const [confirming, setConfirming] = useState<DetailAction | null>(null);
  const rawRecord = text(props.recordPath)
    ? pathValue(data, text(props.recordPath))
    : data;
  const record =
    rawRecord && typeof rawRecord === "object" && !Array.isArray(rawRecord)
      ? (rawRecord as ProtocolRecord)
      : undefined;
  if (!record)
    return (
      <section className="admin-panel">
        <EmptyState />
      </section>
    );

  const actions = (props.actions as DetailAction[] | undefined) ?? [];
  const visibleActions = actions.filter((action) => {
    if (action.permission && !session.permissions.includes(action.permission))
      return false;
    if (!action.when || action.when.kind === "") return true;
    return Boolean(
      evaluate(action.when, {
        record: record as Record<string, Json>,
        context: { accountId: session.account.id },
      }),
    );
  });
  function execute(action: DetailAction) {
    if (action.mode === "dialog" && action.dialog) {
      setParams((old) => {
        const next = new URLSearchParams(old);
        next.set("dialog", action.dialog ?? "");
        return next;
      });
      return;
    }
    if (!action.event || readOnly) return;
    if (action.confirmKey) {
      setConfirming(action);
      return;
    }
    emit(action.event, record);
  }

  const sections = (props.sections as DetailSection[] | undefined) ?? [];
  const forms = (props.forms ?? {}) as Record<string, RuntimeFormV2>;
  const dialogMode = ["create", "edit", "view", "access"].includes(
    params.get("dialog") ?? "",
  )
    ? (params.get("dialog") as "create" | "edit" | "view" | "access")
    : undefined;
  const activeForm = dialogMode ? forms[dialogMode] : undefined;
  const formMeta = (props.dialogTitles ?? {}) as Record<
    string,
    { titleKey: string; descriptionKey: string; wide?: boolean }
  >;
  const optionsData =
    data && typeof data === "object" && !Array.isArray(data)
      ? { ...(data as ProtocolRecord), ...dataMap }
      : { ...dataMap };
  return (
    <section className={text(props.className) || "detail-panel admin-panel"}>
      <div className="detail-heading">
        <div>
          <h2>{text(record[text(props.titleField) || "title"])}</h2>
          {text(props.subtitleField) && (
            <p>{text(record[text(props.subtitleField)])}</p>
          )}
        </div>
        {text(props.statusField) &&
          (text(props.statusValuePrefix) ? (
            <span className="neutral-badge">
              {t(
                text(props.statusValuePrefix) +
                  text(record[text(props.statusField)]),
              )}
            </span>
          ) : (
            <StatusBadge status={text(record[text(props.statusField)])} />
          ))}
      </div>
      {visibleActions.length > 0 && (
        <div className="detail-actions">
          {visibleActions.map((action) => (
            <Button
              key={action.labelKey}
              type="button"
              variant={action.variant ?? "outline"}
              disabled={readOnly}
              onClick={() => execute(action)}
            >
              <Icon name={action.icon} size={15} />
              {t(action.labelKey)}
            </Button>
          ))}
        </div>
      )}
      {text(props.bodyField) && (
        <div className="detail-body">
          {text(props.bodyTitleKey) && <h3>{t(text(props.bodyTitleKey))}</h3>}
          <MarkdownDisplay value={record[text(props.bodyField)]} />
        </div>
      )}
      {sections.length > 0 && (
        <div className="detail-sections">
          {sections.map((section, sectionIndex) => (
            <section className={section.className} key={sectionIndex}>
              {section.titleKey && <h3>{t(section.titleKey)}</h3>}
              <dl>
                {(section.fields ?? []).map((field) => (
                  <div key={field.name}>
                    <dt>{t(field.labelKey ?? field.name)}</dt>
                    <dd>{renderDetailValue(field, record, t)}</dd>
                  </div>
                ))}
              </dl>
            </section>
          ))}
        </div>
      )}
      {confirming?.confirmKey && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setConfirming(null);
          }}
          title={t("admin:feedback.deleteTitle")}
          description={t(confirming.confirmKey)}
        >
          <div className="editor-footer">
            <Button
              type="button"
              variant="outline"
              onClick={() => setConfirming(null)}
            >
              {t("admin:actions.cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => {
                if (confirming.event) emit(confirming.event, record);
                setConfirming(null);
              }}
            >
              {t("runtime:confirm")}
            </Button>
          </div>
        </EditorDialog>
      )}
      {dialogMode && activeForm && formMeta[dialogMode] && (
        <RuntimeFormDialog
          definition={activeForm}
          mode={dialogMode}
          record={record}
          optionsData={optionsData}
          titleKey={formMeta[dialogMode].titleKey}
          descriptionKey={formMeta[dialogMode].descriptionKey}
          wide={formMeta[dialogMode].wide}
          onClose={() =>
            setParams((old) => {
              const next = new URLSearchParams(old);
              next.delete("dialog");
              return next;
            })
          }
        />
      )}
    </section>
  );
}

export function RecordFormPanelComponent({
  props,
  data,
  dataMap,
  readOnly,
}: ComponentRendererProps) {
  const t = useProtocolTranslation();
  const navigate = useNavigate();
  const rawRecord = text(props.recordPath)
    ? pathValue(data, text(props.recordPath))
    : data;
  const record =
    rawRecord && typeof rawRecord === "object" && !Array.isArray(rawRecord)
      ? (rawRecord as ProtocolRecord)
      : undefined;
  const form = props.form as RuntimeFormV2 | undefined;
  if (!form)
    return (
      <section className="admin-panel">
        <EmptyState />
      </section>
    );
  const optionsData =
    data && typeof data === "object" && !Array.isArray(data)
      ? { ...(data as ProtocolRecord), ...dataMap }
      : { ...dataMap };
  const target = text(props.successTarget);
  const cancelTarget = text(props.cancelTarget);
  function toTarget(route: string) {
    return route.replace(/:([a-zA-Z][a-zA-Z0-9]*)/g, (_match, key: string) =>
      encodeURIComponent(text(record?.[key] ?? "")),
    );
  }
  return (
    <section className={text(props.className) || "admin-panel form-panel"}>
      <div className="panel-heading">
        <div>
          <h2>{t(text(props.titleKey))}</h2>
          {text(props.descriptionKey) && <p>{t(text(props.descriptionKey))}</p>}
        </div>
      </div>
      <ProtocolRecordForm
        definition={form}
        mode={text(props.mode) || "create"}
        record={record}
        optionsData={optionsData}
        readOnly={readOnly}
        onCancel={() => cancelTarget && navigate(toTarget(cancelTarget))}
        onSaved={() => target && navigate(toTarget(target))}
      />
    </section>
  );
}

type WorkbenchAction = {
  labelKey: string;
  icon?: string;
  mode: "view" | "edit" | "access" | "delete" | "event";
  event?: string;
  permission?: string;
  requirePermissions?: string[];
  notFlagField?: string;
  notFlagUnlessSelf?: boolean;
  notSelf?: boolean;
  when?: WorkbenchCondition;
};

function workbenchActionVisible(
  action: WorkbenchAction,
  row: ProtocolRecord,
  session: { account: { id: number }; permissions: string[] },
) {
  if (action.permission && !session.permissions.includes(action.permission))
    return false;
  if (
    action.when?.kind &&
    !evaluate(action.when, { record: row as Record<string, Json> })
  )
    return false;
  if (
    action.requirePermissions?.some(
      (value) => !session.permissions.includes(value),
    )
  )
    return false;
  if (action.notFlagField && row[action.notFlagField] === true) return false;
  if (action.notSelf && String(row.id) === String(session.account.id))
    return false;
  return true;
}

type WorkbenchColumn = {
  name: string;
  labelKey?: string;
  display?: string;
  className?: string;
  width?: number;
  titleField?: string;
  subtitleField?: string;
  flagField?: string;
  icon?: string;
  reference?: string;
  field?: string;
  fallbackField?: string;
  itemsField?: string;
  itemTextField?: string;
  itemFallbackField?: string;
  templateKey?: string;
  countFields?: string[];
  emptyKey?: string;
  flagLabelKey?: string;
  relationField?: string;
  relationReference?: string;
  prefixKey?: string;
};

function renderWorkbenchCell(
  column: WorkbenchColumn,
  row: ProtocolRecord,
  references: Record<string, ProtocolRecord[]>,
  t: Translator,
): ReactNode {
  const value = row[column.name];
  switch (column.display) {
    case "avatar": {
      const flagField = column.flagField;
      const subtitleField = column.subtitleField ?? column.field ?? "";
      const name = text(row[column.titleField ?? "name"]);
      return (
        <span className="entity-name">
          <EntityAvatar name={name} seed={Number(row.id ?? 0)} />
          <span>
            <strong>
              {name}
              {flagField && row[flagField] === true ? (
                <ShieldCheck size={13} />
              ) : null}
            </strong>
            {subtitleField ? <small>{text(row[subtitleField])}</small> : null}
          </span>
        </span>
      );
    }
    case "icon": {
      const subtitleField = column.subtitleField;
      const iconField = column.flagField;
      return (
        <div className="entity-name">
          <span className="entity-symbol">
            <Icon
              name={
                iconField
                  ? text(row[iconField]) === "company"
                    ? "building"
                    : "building2"
                  : column.icon
              }
              size={17}
            />
          </span>
          <div>
            <strong>{text(value)}</strong>
            {subtitleField ? <small>{text(row[subtitleField])}</small> : null}
          </div>
        </div>
      );
    }
    case "lookup": {
      const direct = column.field ? row[column.field] : undefined;
      const found = column.reference
        ? references[column.reference]?.find(
            (item) =>
              String(item.id) ===
              String(row[column.fallbackField ?? column.name]),
          )
        : undefined;
      if (found) return <>{text(found.name ?? found.code ?? found.id)}</>;
      if (direct !== undefined && direct !== null && direct !== "")
        return <>{text(direct)}</>;
      return <>{t(column.emptyKey || "admin:fields.unassigned")}</>;
    }
    case "enumBadge":
      return (
        <span className="neutral-badge">
          {t((column.prefixKey ?? "") + text(value))}
        </span>
      );
    case "yesno":
      return <>{value === true ? t("runtime:yes") : t("runtime:no")}</>;
    case "fallback":
      return (
        <>{value ? text(value) : t(column.emptyKey || "protocol.empty")}</>
      );
    case "status":
      return <StatusBadge status={text(value)} />;
    case "date":
      return <ShortDate value={text(value)} />;
    case "mono":
      return <span className="mono-muted">{text(value)}</span>;
    case "scope":
      return (
        <div className="cell-tags">
          {Array.isArray(value)
            ? [
                ...new Set(
                  value.map((item) =>
                    item && typeof item === "object"
                      ? text((item as ProtocolRecord).scope)
                      : text(item),
                  ),
                ),
              ].map((scope) => <ScopeLabel key={scope} scope={scope} />)
            : null}
        </div>
      );
    case "relatedBadges": {
      if (column.flagField && row[column.flagField] === true)
        return (
          <div className="cell-tags">
            <span className="neutral-badge">
              <ShieldCheck size={11} />
              {t(column.flagLabelKey || "protocol.empty")}
            </span>
          </div>
        );
      const groups = records(row[column.itemsField ?? ""]);
      const relationField = column.relationField ?? "";
      const values: string[] = [];
      for (const group of groups) {
        const department = label(
          t,
          group,
          { fallbackField: column.itemFallbackField },
          references,
          column.reference,
        );
        for (const id of records(group[relationField]).length
          ? []
          : Array.isArray(group[relationField])
            ? group[relationField]
            : []) {
          const role = references[column.relationReference ?? ""]?.find(
            (item) => String(item.id) === String(id),
          );
          values.push(`${department}:${role ? text(role.name) : ""}`);
        }
      }
      if (!values.length)
        return (
          <span className="muted-text">
            {t(column.emptyKey || "protocol.empty")}
          </span>
        );
      return (
        <div className="cell-tags">
          {values.map((value) => (
            <span className="neutral-badge" key={value}>
              {value.replace(":", " · ")}
            </span>
          ))}
        </div>
      );
    }
    case "badges": {
      const values = records(row[column.itemsField ?? column.name]);
      if (!values.length)
        return (
          <span className="muted-text">
            {t(column.emptyKey || "protocol.empty")}
          </span>
        );
      return (
        <div className="cell-tags">
          {values.map((item, index) => (
            <span className="neutral-badge" key={index}>
              {column.reference
                ? label(
                    t,
                    item,
                    { fallbackField: column.itemFallbackField },
                    references,
                    column.reference,
                  )
                : column.templateKey
                  ? t(column.templateKey, {
                      value: text(item[column.itemTextField ?? ""]),
                    })
                  : text(
                      item[
                        column.itemTextField ??
                          column.itemFallbackField ??
                          "name"
                      ],
                    )}
            </span>
          ))}
        </div>
      );
    }
    case "membershipCount": {
      const accounts = references[column.reference ?? ""];
      const count = (accounts ?? []).filter((account) =>
        records(account.memberships).some(
          (membership) =>
            String(membership[column.field ?? "departmentId"]) ===
            String(row.id),
        ),
      ).length;
      return (
        <span className="member-count">
          <Users size={13} />
          {count}
        </span>
      );
    }
    case "countText":
      return (
        <>
          {t(column.templateKey ?? "", {
            count: Number(row[column.field ?? "count"] ?? 0),
          })}
        </>
      );
    case "progress":
      return (
        <>
          {t(column.templateKey ?? "", {
            done: Number(row.taskDoneCount ?? 0),
            total: Number(row[column.field ?? "taskCount"] ?? 0),
          })}
        </>
      );
    case "count":
      return (
        <span className="member-count">
          <Users size={13} />
          {(column.countFields ?? []).reduce(
            (count, field) => count + records(row[field]).length,
            0,
          )}
        </span>
      );
    default:
      return <>{text(value)}</>;
  }
}

function treeCount(
  item: ProtocolRecord,
  data: unknown,
  tree: { countPath?: string; countField?: string; countMode?: string },
) {
  const values = dataset(data, tree.countPath ?? "");
  if (tree.countMode === "memberships")
    return values.filter((account) =>
      records(account.memberships).some(
        (membership) =>
          String(membership[tree.countField ?? "departmentId"]) ===
          String(item.id),
      ),
    ).length;
  return values.filter(
    (row) => String(row[tree.countField ?? "id"]) === String(item.id),
  ).length;
}

export function TreeTablePanelComponent({
  props,
  data,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const t = useProtocolTranslation();
  const [params, setParams] = useSearchParams();
  const tree = (props.tree ?? {}) as {
    titleKey?: string;
    allLabelKey?: string;
    itemsPath?: string;
    countPath?: string;
    parentField?: string;
    selectedParam?: string;
    footnoteKey?: string;
    filterField?: string;
    filterValue?: unknown;
    countField?: string;
    countMode?: string;
    chevron?: boolean;
  };
  const treeItems = dataset(data, tree.itemsPath ?? "");
  const filteredTree = tree.filterField
    ? treeItems.filter(
        (item) => text(item[tree.filterField ?? ""]) === text(tree.filterValue),
      )
    : treeItems;
  const selectedParam = tree.selectedParam ?? "selected";
  const selected = Number(params.get(selectedParam)) || 0;
  const parentField = tree.parentField ?? "parentId";
  const roots = filteredTree.filter(
    (item) =>
      Number(item[parentField]) === 0 ||
      !filteredTree.some(
        (parent) => String(parent.id) === String(item[parentField]),
      ),
  );
  const rows = dataset(data, text(props.rowsPath));
  const rowParentField = text(props.treeRowParentField) || parentField;
  const visibleRows = selected
    ? rows.filter((row) => {
        const value = Number(row[rowParentField]);
        return (
          value === selected ||
          rows.some(
            (row2) =>
              Number(row2.id) === value &&
              Number(row2[rowParentField]) === selected,
          )
        );
      })
    : rows;
  const selectedRecord = filteredTree.find(
    (item) => Number(item.id) === selected,
  );
  const scopedData = {
    ...(data as ProtocolRecord),
    protocolScopedRows: visibleRows,
    protocolSelectedTitle: selectedRecord?.name,
  };
  return (
    <div className="management-split">
      <aside className="admin-panel tree-panel">
        <div className="tree-title">
          <Icon name={text(props.treeIcon) || "network"} size={16} />
          {t(tree.titleKey ?? "")}
        </div>
        <button
          className={`tree-node ${selected === 0 ? "selected" : ""}`}
          onClick={() =>
            setParams((old) => {
              const next = new URLSearchParams(old);
              next.delete(selectedParam);
              return next;
            })
          }
        >
          {tree.chevron ? <Icon name="chevron" size={12} /> : null}
          <Icon name="grid" size={15} />
          <span>{t(tree.allLabelKey ?? "")}</span>
          <small>{rows.length}</small>
        </button>
        <div className="organization-tree">
          {roots.map((item) => (
            <div key={String(item.id)}>
              <button
                className={`tree-node ${selected === Number(item.id) ? "selected" : ""}`}
                onClick={() =>
                  setParams((old) => {
                    const next = new URLSearchParams(old);
                    next.set(selectedParam, String(item.id));
                    return next;
                  })
                }
              >
                {tree.chevron ? <Icon name="chevron" size={12} /> : null}
                <Icon
                  name={text(props.treeItemIcon) || "building2"}
                  size={15}
                />
                <span>{text(item.name)}</span>
                {tree.countPath ? (
                  <small>{treeCount(item, data, tree)}</small>
                ) : null}
              </button>
              {filteredTree
                .filter(
                  (child) => String(child[parentField]) === String(item.id),
                )
                .map((child) => (
                  <button
                    key={String(child.id)}
                    className={`tree-node tree-child ${selected === Number(child.id) ? "selected" : ""}`}
                    onClick={() =>
                      setParams((old) => {
                        const next = new URLSearchParams(old);
                        next.set(selectedParam, String(child.id));
                        return next;
                      })
                    }
                  >
                    <Icon
                      name={text(props.treeChildIcon) || "building"}
                      size={14}
                    />
                    <span>{text(child.name)}</span>
                  </button>
                ))}
            </div>
          ))}
        </div>
        <div className="tree-footnote">
          <Icon name={text(props.treeFootnoteIcon) || "network"} size={15} />
          {t(tree.footnoteKey ?? "")}
        </div>
      </aside>
      <TablePanelComponent
        props={{
          ...props,
          rowsPath: "protocolScopedRows",
          search: props.search,
        }}
        data={scopedData}
        readOnly={readOnly}
        emit={emit}
      />
    </div>
  );
}

export function CardGridPanelComponent({
  props,
  data,
}: ComponentRendererProps) {
  const t = useProtocolTranslation();
  const session = useSession();
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [view, setView] = useState<"grid" | "list">("grid");
  const [deleting, setDeleting] = useState<ProtocolRecord | null>(null);
  const [notice, setNotice] = useState("");
  const [actionError, setActionError] = useState<unknown>(null);
  const rows = dataset(data, text(props.rowsPath));
  const search = params.get(text(props.searchParam) || "q") ?? "";
  const fields = (props.searchFields as string[]) ?? [];
  const filtered = rows.filter(
    (row) =>
      !fields.length ||
      fields.some((field) =>
        text(row[field]).toLowerCase().includes(search.toLowerCase()),
      ),
  );
  const actionConfig = (props.rowActions as WorkbenchAction[]) ?? [];
  const dialog = params.get("dialog");
  const dialogMode =
    dialog && ["create", "edit", "access", "view"].includes(dialog)
      ? dialog
      : undefined;
  const dialogId = Number(params.get("id")) || 0;
  const selected = rows.find((row) => Number(row.id) === dialogId);
  const forms = (props.forms ?? {}) as Record<string, RuntimeFormV2>;
  const formMeta = (props.dialogTitles ?? {}) as Record<
    string,
    { titleKey: string; descriptionKey: string; wide?: boolean }
  >;
  const columns = (props.columns as WorkbenchColumn[]) ?? [];
  const keyField = text(props.keyField) || "id";

  function closeDialog() {
    setParams((old) => {
      const next = new URLSearchParams(old);
      next.delete("dialog");
      next.delete("id");
      return next;
    });
  }

  async function remove() {
    const configuration = props.delete as
      { capability?: string; version?: string } | undefined;
    if (!deleting || !configuration?.capability) return;
    try {
      const result = await runtimeExecute(
        configuration.capability,
        configuration.version || "1.0.0",
        { id: deleting.id },
        crypto.randomUUID(),
        crypto.randomUUID(),
      );
      if (
        result.status !== "succeeded" &&
        result.status !== "awaiting_approval"
      )
        throw new ApiError(result.errorCode || "internal", 400);
      await client.invalidateQueries({ queryKey: ["protocol-v3"] });
      setDeleting(null);
      setNotice(t("admin:feedback.deleted"));
    } catch (cause) {
      setActionError(cause);
      setDeleting(null);
    }
  }

  return (
    <>
      <div className="standalone-toolbar">
        <ListSearch
          value={search}
          onChange={(value) =>
            setParams((old) => {
              const next = new URLSearchParams(old);
              if (value) next.set("q", value);
              else next.delete("q");
              return next;
            })
          }
        />
        <div className="toolbar-right">
          <span className="muted-text">
            {t(text(props.totalKey), { count: filtered.length })}
          </span>
          <div className="view-toggle">
            <button
              className={view === "grid" ? "active" : ""}
              aria-pressed={view === "grid"}
              aria-label={t("admin:actions.grid")}
              onClick={() => setView("grid")}
            >
              <LayoutGrid size={15} />
            </button>
            <button
              className={view === "list" ? "active" : ""}
              aria-pressed={view === "list"}
              aria-label={t("admin:actions.list")}
              onClick={() => setView("list")}
            >
              <List size={15} />
            </button>
          </div>
          {Boolean(props.createKey) &&
            (!props.createPermission ||
              session.permissions.includes(text(props.createPermission))) && (
              <Button
                onClick={() =>
                  setParams((old) => {
                    const next = new URLSearchParams(old);
                    next.set("dialog", "create");
                    return next;
                  })
                }
              >
                <Icon name={text(props.createIcon) || "plus"} size={15} />
                {t(text(props.createKey))}
              </Button>
            )}
        </div>
      </div>
      {view === "grid" ? (
        <div className="role-card-grid">
          {filtered.map((row) => (
            <article className="admin-panel role-card" key={String(row.id)}>
              <div className="role-card-top">
                <span className={`role-symbol tone-${Number(row.id) % 5}`}>
                  <ShieldCheck size={22} />
                </span>
                <RowActions
                  actions={actionConfig
                    .filter((action) =>
                      workbenchActionVisible(action, row, session),
                    )
                    .map((action) => ({
                      label: t(action.labelKey),
                      icon: <Icon name={action.icon} />,
                      danger: action.mode === "delete",
                      onClick: () => {
                        if (action.mode === "delete") {
                          setDeleting(row);
                          return;
                        }
                        setParams((old) => {
                          const next = new URLSearchParams(old);
                          next.set("dialog", action.mode);
                          next.set("id", String(row[keyField]));
                          return next;
                        });
                      },
                    }))}
                />
              </div>
              <h2>{text(row.name)}</h2>
              <p>{text(row.description)}</p>
              <div className="role-card-scopes">
                {records(row.grants)
                  .map((grant) => text(grant.scope))
                  .filter(
                    (scope, index, values) => values.indexOf(scope) === index,
                  )
                  .map((scope) => (
                    <ScopeLabel key={scope} scope={scope} />
                  ))}
              </div>
              <div className="role-card-meta">
                <span>
                  <Users size={14} />
                  {t("admin:roles.members", {
                    count: Number(row.memberCount ?? 0),
                  })}
                </span>
                <span>
                  <KeyRound size={14} />
                  {t("admin:roles.permissions", {
                    count: records(row.grants).length,
                  })}
                </span>
              </div>
              <div className="role-card-footer">
                <StatusBadge status={text(row.status)} />
                <Link
                  to={text(props.detailHref).replace(":id", String(row.id))}
                >
                  {t("admin:actions.manageAccess")}
                </Link>
              </div>
            </article>
          ))}
        </div>
      ) : (
        <section className="admin-panel">
          <DataTable
            rows={filtered.map((row, index) => {
              const value = Number(row[keyField] ?? row.id ?? index);
              return { ...row, id: Number.isFinite(value) ? value : index };
            })}
            columns={[
              ...columns.map((column) => ({
                id: column.name,
                header: column.labelKey ? t(column.labelKey) : column.name,
                className: column.className,
                cell: (row: ProtocolRecord) =>
                  renderWorkbenchCell(column, row, {}, t),
              })),
              {
                id: "actions",
                header: t("admin:fields.actions"),
                className: "actions-column",
                cell: (row: ProtocolRecord) => (
                  <RowActions
                    actions={actionConfig
                      .filter((action) =>
                        workbenchActionVisible(action, row, session),
                      )
                      .map((action) => ({
                        label: t(action.labelKey),
                        icon: <Icon name={action.icon} />,
                        danger: action.mode === "delete",
                        onClick: () => {
                          if (action.mode === "delete") {
                            setDeleting(row);
                            return;
                          }
                          setParams((old) => {
                            const next = new URLSearchParams(old);
                            next.set("dialog", action.mode);
                            next.set("id", String(row[keyField]));
                            return next;
                          });
                        },
                      }))}
                  />
                ),
              },
            ]}
          />
        </section>
      )}
      {dialogMode && forms[dialogMode] && formMeta[dialogMode] && (
        <RuntimeFormDialog
          definition={forms[dialogMode]}
          mode={dialogMode}
          record={dialogMode === "create" ? undefined : selected}
          optionsData={data}
          titleKey={formMeta[dialogMode].titleKey}
          descriptionKey={formMeta[dialogMode].descriptionKey}
          wide={formMeta[dialogMode].wide}
          onClose={closeDialog}
        />
      )}
      {deleting && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setDeleting(null);
          }}
          title={t("admin:feedback.deleteTitle")}
          description={t("admin:feedback.deleteDescription", {
            name: text(
              deleting[props.deleteLabelField as string] ?? deleting.name,
            ),
          })}
        >
          <div className="editor-footer">
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleting(null)}
            >
              {t("admin:actions.cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => void remove()}
            >
              {t("admin:actions.delete")}
            </Button>
          </div>
        </EditorDialog>
      )}
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
      {actionError !== null && (
        <ProtocolErrorDialog
          error={actionError}
          onClose={() => setActionError(null)}
        />
      )}
    </>
  );
}

type WorkbenchPanel = {
  id: string;
  labelKey: string;
  icon?: string;
  permission?: string;
  type:
    | "table"
    | "tree-table"
    | "cards"
    | "master-detail"
    | "detail"
    | "search-picker";
  props: Record<string, unknown>;
};

export function PageTabsComponent({
  props,
  data,
  dataMap,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const namedData = Object.fromEntries(Object.entries(dataMap ?? {}));
  const scopedData = { ...namedData, primary: data };
  const t = useProtocolTranslation();
  const session = useSession();
  const [params, setParams] = useSearchParams();
  const panels = (props.panels as WorkbenchPanel[]) ?? [];
  const allowed = panels.filter(
    (panel) =>
      !panel.permission || session.permissions.includes(panel.permission),
  );
  const param = text(props.param);
  const active =
    allowed.find((panel) => panel.id === params.get(param)) ?? allowed[0];
  function renderPanel(panel: WorkbenchPanel) {
    const renderer = {
      props: (panel.props ?? panel) as Record<string, unknown>,
      data: scopedData,
      dataMap: scopedData,
      readOnly,
      emit,
    };
    if (panel.type === "detail")
      return <DetailPanelComponent {...renderer} data={data} />;
    if (panel.type === "search-picker")
      return <SearchPickerPanelComponent {...renderer} />;
    if (panel.type === "tree-table")
      return <TreeTablePanelComponent {...renderer} />;
    if (panel.type === "cards") return <CardGridPanelComponent {...renderer} />;
    if (panel.type === "master-detail")
      return <MasterDetailPanelComponent {...renderer} />;
    return <TablePanelComponent {...renderer} />;
  }
  return (
    <>
      {allowed.length > 1 && (
        <div className="admin-page-tabs">
          {allowed.map((panel) => (
            <button
              key={panel.id}
              className={active?.id === panel.id ? "active" : ""}
              onClick={() =>
                setParams((old) => {
                  const next = new URLSearchParams(old);
                  next.set(param, panel.id);
                  return next;
                })
              }
            >
              <Icon name={panel.icon} size={15} />
              {t(panel.labelKey)}
            </button>
          ))}
        </div>
      )}
      {active && renderPanel(active)}
    </>
  );
}

function InlineRuntimeForm({
  definition,
  record,
  mode,
  optionsData,
  onSaved,
}: {
  definition: RuntimeFormV2;
  record: ProtocolRecord;
  mode: string;
  optionsData?: unknown;
  onSaved?: () => void;
}) {
  return (
    <ProtocolRecordForm
      definition={definition}
      mode={mode}
      record={record}
      optionsData={optionsData}
      onSaved={onSaved}
    />
  );
}

export function MasterDetailPanelComponent({
  props,
  data,
}: ComponentRendererProps) {
  const t = useProtocolTranslation();
  const [params, setParams] = useSearchParams();
  const [notice, setNotice] = useState("");
  const rows = dataset(data, text(props.rowsPath));
  const keyField = text(props.keyField) || "id";
  const selectedParam = text(props.selectedParam) || "id";
  const selected =
    Number(params.get(selectedParam)) || Number(rows[0]?.[keyField]) || 0;
  const record = rows.find((row) => Number(row[keyField]) === selected);
  const titleField = text(props.titleField) || "name";
  const subtitleField = text(props.subtitleField) || "code";
  const form = props.form as RuntimeFormV2 | undefined;
  const mode = text(props.mode) || "edit";
  return (
    <div className="management-split permissions-split">
      <aside className="admin-panel permission-subjects">
        <div className="tree-title">{t(text(props.titleKey))}</div>
        {rows.map((row) => (
          <button
            key={String(row[keyField])}
            className={`subject-item ${selected === Number(row[keyField]) ? "selected" : ""}`}
            onClick={() =>
              setParams((old) => {
                const next = new URLSearchParams(old);
                next.set(selectedParam, String(row[keyField]));
                return next;
              })
            }
          >
            <span className="subject-icon">
              <Icon name={text(props.itemIcon)} size={17} />
            </span>
            <span>
              <strong>{text(row[titleField])}</strong>
              <small>{text(row[subtitleField])}</small>
            </span>
            <Icon name="chevron" size={14} />
          </button>
        ))}
      </aside>
      <section className="admin-panel permission-detail">
        {record && form ? (
          <>
            <div className="permission-detail-heading">
              <div>
                <h2>{text(record[titleField])}</h2>
                <p>{text(record[subtitleField])}</p>
              </div>
              <StatusBadge status={text(record.status)} />
            </div>
            <InlineRuntimeForm
              definition={form}
              record={record}
              mode={mode}
              optionsData={data}
              onSaved={() => setNotice(t("admin:feedback.saved"))}
            />
          </>
        ) : (
          <EmptyState />
        )}
      </section>
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </div>
  );
}

function ProtocolRecordForm({
  definition,
  mode,
  record,
  optionsData,
  onClose,
  onCancel,
  readOnly = false,
  onSaved,
}: {
  definition: RuntimeFormV2;
  mode: string;
  record?: ProtocolRecord;
  optionsData?: unknown;
  onClose?: () => void;
  onCancel?: () => void;
  readOnly?: boolean;
  onSaved?: () => void;
}) {
  const { t } = useTranslation(["runtime", "admin"]);
  const runtime = useProtocolRuntime();
  const client = useQueryClient();
  const session = useSession();
  const route = useParams();
  const [params] = useSearchParams();
  const command = mode === "create" ? definition.create : definition.edit;
  const config = useConfiguration();
  const passwordMin = Number(config.value("security.passwordMin", 12));
  const { referencePaths: formPaths = {}, referenceFilters = {} } =
    definition as RuntimeFormV2 & {
      referencePaths?: Record<string, string>;
      referenceFilters?: Record<
        string,
        { optionField?: string; formField?: string; value?: string }[]
      >;
    };
  const [activeGroup, setActiveGroup] = useState(
    () =>
      (definition.groups ?? []).find((group) => group.type === "tabs")?.id ??
      "",
  );
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const defaults = useMemo(() => {
    const values: Record<string, unknown> = {};
    for (const field of command.fields) {
      let value = record?.[field.name];
      if (field.component === "membership-editor" && record?.memberships) {
        value = records(record.memberships)
          .filter(
            (membership) =>
              String(membership.departmentId) !== String(record.departmentId),
          )
          .map((membership) => ({
            organizationId: membership.organizationId,
            departmentId: membership.departmentId,
            roleIds: membership.roleIds ?? [],
            overrides: membership.overrides ?? [],
          }));
      }
      values[field.name] =
        value ??
        (field.default.kind ? evaluate(field.default, {}) : undefined) ??
        (field.type === "array" || field.multiple
          ? []
          : field.type === "boolean"
            ? false
            : field.type === "reference" ||
                field.type === "integer" ||
                field.type === "number"
              ? 0
              : "");
    }
    return values;
  }, [command.fields, record]);
  const form = useForm<Record<string, unknown>>({
    resolver: zodResolver(
      formSchema(command.fields, {}).superRefine((values, context) => {
        if (command.capability.name !== "identity.account.create") return;
        const password = values.password;
        if (
          typeof password === "string" &&
          [...password].length < passwordMin
        ) {
          context.addIssue({
            code: "custom",
            path: ["password"],
            message: "passwordPolicy",
          });
        }
      }),
    ),
    defaultValues: defaults,
  });
  const watched = useWatch({ control: form.control }) as Record<
    string,
    unknown
  >;
  const tabGroups = (definition.groups ?? []).filter(
    (group) => group.type === "tabs",
  );
  const membershipField = command.fields.find(
    (field) => field.component === "membership-editor",
  );
  const visibleFields = command.fields.filter(
    (field) =>
      !field.hidden &&
      evaluate(field.visibleWhen, {
        record: (record ?? {}) as Record<string, Json>,
        form: form.getValues() as Record<string, Json>,
      }),
  );
  const fields = (
    activeGroup
      ? visibleFields.filter((field) => (field.group ?? "") === activeGroup)
      : visibleFields
  ).filter((field) => field.component !== "membership-editor");
  const isFieldDisabled = (field: RuntimeFieldV2) =>
    busy ||
    readOnly ||
    mode === "view" ||
    field.readOnly ||
    (field.disabledWhen?.kind
      ? Boolean(
          evaluate(field.disabledWhen, {
            record: (record ?? {}) as Record<string, Json>,
            form: watched as Record<string, Json>,
          }),
        )
      : false);
  const referenceOptions = (
    field: RuntimeFieldV2,
  ): { value: Json; label: string }[] => {
    if (field.reference) {
      const provider = runtime.application.providers.find(
        (item) => item.id === field.reference,
      );
      const path = formPaths[field.reference];
      let rows = path ? dataset(optionsData, path) : [];
      for (const filter of referenceFilters[field.name] ?? []) {
        const optionField = filter.optionField;
        if (!optionField) continue;
        const formField = filter.formField;
        const expected = filter.value;
        rows = rows.filter((row) =>
          formField
            ? String(row[optionField]) === String(watched[formField])
            : expected === undefined || String(row[optionField]) === expected,
        );
      }
      return [
        { value: 0, label: t("runtime:none") },
        ...rows.map((row) => ({
          value: row[provider?.keyField ?? "id"] as Json,
          label: text(
            row[provider?.labelField ?? "name"] ??
              row[provider?.keyField ?? "id"],
          ),
        })),
      ];
    }
    return (field.enum ?? []).map((value) => ({
      value,
      label: field.valueLabelPrefix ? t(field.valueLabelPrefix + value) : value,
    }));
  };

  async function submit(values: Record<string, unknown>) {
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      const active: Record<string, unknown> = {};
      for (const field of command.fields) {
        const value = values[field.name];
        if (
          value === undefined ||
          ((field.type === "integer" ||
            field.type === "number" ||
            field.type === "reference") &&
            (value === "" || value === null))
        )
          continue;
        active[field.name] =
          field.component === "membership-editor"
            ? { memberships: Array.isArray(value) ? value : [] }
            : value;
      }
      const input = mapInput(command.input, {
        record: (record ?? {}) as Record<string, Json>,
        form: active as Record<string, Json>,
        route: route as Record<string, Json>,
        query: Object.fromEntries(params.entries()) as Record<string, Json>,
        selection: { ids: [] },
        context: { accountId: session.account.id },
      });
      const result = await runtimeExecute(
        command.capability.name,
        command.capability.version,
        input,
        crypto.randomUUID(),
        runtime.correlationId,
        runtime.invocation,
      );
      if (result.status === "awaiting_approval") {
        setNotice(t("runtime:approvalPrefix"));
        return;
      }
      if (result.status !== "succeeded")
        throw new ApiError(result.errorCode || "internal", 400);
      await client.invalidateQueries({ queryKey: ["protocol-v3"] });
      onSaved?.();
      onClose?.();
    } catch (cause) {
      setError(cause);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit(submit)}
    >
      {error !== null && (
        <ProtocolErrorDialog error={error} onClose={() => setError(null)} />
      )}
      {notice && <div role="status">{notice}</div>}
      {tabGroups.length > 0 && (
        <div className="editor-tabs">
          {tabGroups.map((group) => (
            <button
              key={group.id}
              type="button"
              className={activeGroup === group.id ? "active" : ""}
              aria-selected={activeGroup === group.id}
              data-active={activeGroup === group.id}
              onClick={() => setActiveGroup(group.id)}
            >
              {t(group.labelKey)}
            </button>
          ))}
        </div>
      )}
      {membershipField && (
        <Controller
          control={form.control}
          name={membershipField.name}
          render={({ field: controlled }) => (
            <MembershipEditor
              value={records(controlled.value)}
              onChange={(next) =>
                controlled.onChange(next as unknown as Json[])
              }
              form={form}
              optionsData={optionsData}
              record={record}
              disabled={busy || mode === "view"}
              onSelectMembership={() => setActiveGroup("access")}
              showAccess={tabGroups.length === 0 || activeGroup === "access"}
            />
          )}
        />
      )}
      <div className="editor-grid">
        {fields.map((field) => (
          <Controller
            key={field.name}
            control={form.control}
            name={field.name}
            render={({ field: controlled }) => (
              <>
                {field.component === "permission-matrix" ? (
                  <div className="field-wide">
                    <div className="editor-section-heading">
                      <h3>{t("admin:permissions.matrix")}</h3>
                    </div>
                    <PermissionMatrix
                      catalog={
                        records(
                          pathValue(
                            optionsData,
                            formPaths.catalog ?? "catalog",
                          ),
                        ) as unknown as PermissionDefinition[]
                      }
                      departments={
                        records(
                          pathValue(
                            optionsData,
                            formPaths.departments ?? "departments",
                          ),
                        ) as unknown as DepartmentRecord[]
                      }
                      grants={(controlled.value as PermissionGrant[]) ?? []}
                      disabled={isFieldDisabled(field)}
                      onChange={controlled.onChange}
                    />
                  </div>
                ) : (
                  <FormField
                    label={t(field.labelKey)}
                    error={form.formState.errors[field.name]?.message}
                    wide={
                      field.component === "textarea" ||
                      field.component === "subtable"
                    }
                  >
                    <RegisteredControl
                      definition={field}
                      id={`${definition.id}-${field.name}`}
                      name={field.name}
                      value={controlled.value as Json | undefined}
                      onChange={(value) => {
                        controlled.onChange(value);
                        if (controlled.value === value) return;
                        for (const [name, filters] of Object.entries(
                          referenceFilters,
                        )) {
                          if (
                            filters.some(
                              (filter) => filter.formField === field.name,
                            )
                          )
                            form.setValue(name, 0, { shouldDirty: true });
                        }
                      }}
                      onBlur={controlled.onBlur}
                      disabled={isFieldDisabled(field)}
                      options={referenceOptions(field)}
                    />
                  </FormField>
                )}
              </>
            )}
          />
        ))}
      </div>
      <div className="editor-footer">
        {onCancel && (
          <Button
            type="button"
            variant="outline"
            onClick={onCancel}
            disabled={busy}
          >
            {t("admin:actions.cancel")}
          </Button>
        )}
        {onClose && (
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={busy}
          >
            {t("admin:actions.cancel")}
          </Button>
        )}
        <Button type="submit" disabled={busy || readOnly || mode === "view"}>
          {busy ? t("admin:actions.saving") : t("admin:actions.save")}
        </Button>
      </div>
    </form>
  );
}

type MembershipValue = ProtocolRecord;

function MembershipEditor({
  value,
  onChange,
  form,
  optionsData,
  record,
  disabled,
  onSelectMembership,
  showAccess,
}: {
  value: MembershipValue[];
  onChange: (value: MembershipValue[]) => void;
  form: ReturnType<typeof useForm<Record<string, unknown>>>;
  optionsData?: unknown;
  record?: ProtocolRecord;
  disabled: boolean;
  onSelectMembership: () => void;
  showAccess: boolean;
}) {
  const { t } = useTranslation(["runtime", "admin"]);
  const session = useSession();
  const snapshot =
    (pathValue(optionsData, "_accounts") as ProtocolRecord | undefined) ??
    (optionsData as ProtocolRecord | undefined);
  const departments = records(snapshot?.departments);
  const organizations = records(snapshot?.organizations);
  const roles = records(snapshot?.roles);
  const catalog = records(
    snapshot?.catalog,
  ) as unknown as PermissionDefinition[];
  const matrixDepartments = records(
    snapshot?.departments,
  ) as unknown as DepartmentRecord[];
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const [additionalDepartment, setAdditionalDepartment] = useState(0);
  const primaryDepartment = Number(form.getValues("departmentId") ?? 0);
  const selectedMembership = value[selectedIndex];
  const canManage =
    !disabled &&
    session.permissions.includes("access:manage") &&
    (!record?.isAdmin || record.id === session.account.id);
  const canGrant =
    canManage && (!record?.isAdmin || Boolean(selectedMembership));
  const roleIds = (selectedMembership?.roleIds ??
    form.getValues("roleIds")) as Json[];
  const overrides = (selectedMembership?.overrides ??
    form.getValues("overrides")) as PermissionGrant[];
  function departmentName(id: unknown): string {
    const department = departments.find(
      (item) => String(item.id) === String(id),
    );
    return department ? text(department.name) : t("admin:fields.unassigned");
  }

  function setRoles(next: Json[]) {
    if (!selectedMembership) {
      form.setValue("roleIds", next, {
        shouldDirty: true,
        shouldValidate: true,
      });
      return;
    }
    onChange(
      value.map((membership, index) =>
        index === selectedIndex ? { ...membership, roleIds: next } : membership,
      ),
    );
  }

  function setGrants(next: PermissionGrant[]) {
    if (!selectedMembership) {
      form.setValue("overrides", next, {
        shouldDirty: true,
        shouldValidate: true,
      });
      return;
    }
    onChange(
      value.map((membership, index) =>
        index === selectedIndex
          ? { ...membership, overrides: next }
          : membership,
      ),
    );
  }

  function addMembership() {
    const department = departments.find(
      (item) => String(item.id) === String(additionalDepartment),
    );
    if (!department || value.length >= 49) return;
    onChange([
      ...value,
      {
        organizationId: department.organizationId,
        departmentId: department.id,
        roleIds: [],
        overrides: [],
      },
    ]);
    setSelectedIndex(value.length);
    setAdditionalDepartment(0);
    onSelectMembership();
  }

  return (
    <section className="membership-editor">
      <div className="editor-section-heading">
        <h3>{t("admin:memberships.title")}</h3>
        <p>{t("admin:memberships.hint")}</p>
      </div>
      <div className="cell-tags">
        <button
          type="button"
          className="neutral-badge"
          aria-pressed={selectedIndex === -1}
          onClick={() => {
            setSelectedIndex(-1);
            onSelectMembership();
          }}
        >
          {t("admin:memberships.primaryLabel", {
            department: departmentName(primaryDepartment),
          })}
        </button>
        {value.map((membership, index) => (
          <button
            type="button"
            className="neutral-badge"
            key={String(membership.departmentId)}
            aria-pressed={selectedIndex === index}
            onClick={() => {
              setSelectedIndex(index);
              onSelectMembership();
            }}
          >
            {departmentName(membership.departmentId)}
          </button>
        ))}
      </div>
      {canManage && (
        <div className="membership-actions">
          <FormField label={t("admin:memberships.additional")}>
            <select
              value={additionalDepartment}
              onChange={(event) =>
                setAdditionalDepartment(Number(event.target.value))
              }
            >
              <option value={0}>{t("admin:fields.select")}</option>
              {departments
                .filter(
                  (department) =>
                    department.status === "active" &&
                    String(department.id) !== String(primaryDepartment) &&
                    !value.some(
                      (membership) =>
                        String(membership.departmentId) ===
                        String(department.id),
                    ),
                )
                .map((department) => (
                  <option
                    key={String(department.id)}
                    value={Number(department.id)}
                  >
                    {t("admin:memberships.option", {
                      organization:
                        organizations.find(
                          (organization) =>
                            String(organization.id) ===
                            String(department.organizationId),
                        )?.name ?? t("admin:fields.unassigned"),
                      department: String(department.name),
                    })}
                  </option>
                ))}
            </select>
          </FormField>
          <Button
            type="button"
            variant="outline"
            disabled={!additionalDepartment || value.length >= 49}
            onClick={addMembership}
          >
            {t("admin:memberships.add")}
          </Button>
          {selectedMembership && (
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                onChange(value.filter((_, index) => index !== selectedIndex));
                setSelectedIndex(-1);
              }}
            >
              {t("admin:memberships.remove")}
            </Button>
          )}
        </div>
      )}
      {!disabled && !canManage && (
        <p className="form-hint">{t("admin:memberships.restricted")}</p>
      )}
      {showAccess && (
        <>
          <div className="editor-section-heading">
            <h3>{t("admin:accounts.roles")}</h3>
            <p>{t("admin:accounts.roleHint")}</p>
          </div>
          <div className="role-options">
            {roles.map((role) => {
              const id = role.id as Json;
              const grants = records(role.grants);
              return (
                <label key={String(role.id)}>
                  <Checkbox
                    disabled={!canGrant}
                    checked={roleIds.includes(id)}
                    onCheckedChange={(checked) =>
                      setRoles(
                        checked
                          ? [...roleIds, id]
                          : roleIds.filter((value) => value !== id),
                      )
                    }
                  />
                  <span className="role-option-content">
                    <strong>{String(role.name)}</strong>
                    <small>
                      {t("admin:roles.permissions", { count: grants.length })}
                    </small>
                  </span>
                  <ScopeLabel
                    scope={String(grants[0]?.scope ?? "DEPARTMENT")}
                  />
                </label>
              );
            })}
          </div>
          <div className="editor-section-heading">
            <h3>{t("admin:accounts.overrides")}</h3>
            <p>{t("admin:accounts.overrideHint")}</p>
          </div>
          {Boolean(record?.isAdmin) && !selectedMembership && (
            <p className="form-hint admin-protection">
              {t("admin:memberships.adminHint")}
            </p>
          )}
          <PermissionMatrix
            key={String(selectedMembership?.departmentId ?? primaryDepartment)}
            catalog={catalog}
            departments={matrixDepartments}
            grants={overrides}
            account
            disabled={!canGrant}
            onChange={setGrants}
          />
        </>
      )}
    </section>
  );
}
