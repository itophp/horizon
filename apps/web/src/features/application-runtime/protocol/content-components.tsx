import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Fragment,
  useContext,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import {
  Archive,
  ArrowUpRight,
  Building2,
  Play,
  Plus,
  RefreshCw,
  ShieldCheck,
  Square,
} from "lucide-react";
import { ShortDate, SuccessNotice } from "@/features/administration/shared";
import { formatMoment } from "@/lib/capability";
import { evaluate, pathValue } from "../expression";
import {
  PresentationPermissions,
  type ComponentRendererProps,
} from "./component-registry";
import type { Expression } from "./types";

export const contentIcons = {
  building: Building2,
  shield: ShieldCheck,
  archive: Archive,
  play: Play,
  plus: Plus,
  refresh: RefreshCw,
  stop: Square,
};

type FieldArgument =
  | Expression
  | {
      expression: Expression;
      format?: "dateTime";
      labelKey?: string;
      argument?: string;
      skipEmpty?: boolean;
    };
type Field = {
  when?: Expression;
  path?: string;
  value?: Expression;
  labelKey?: string;
  labelPrefix?: string;
  labelSuffix?: string;
  fallbackValue?: Expression;
  args?: Record<string, FieldArgument>;
  fallbackKey?: string;
  format?: "shortDate" | "dateTime" | "code" | "list" | "strong" | "small";
  className?: string;
  classPrefix?: string;
  append?: string;
};
type Action = {
  id: string;
  labelKey: string;
  pendingKey?: string;
  iconOnly?: boolean;
  payloadLabelKey?: string;
  className?: string;
  icon?: keyof typeof contentIcons;
  linkArrow?: boolean;
  permission?: string;
  visible?: Expression;
  href?: Expression;
};
type Column = {
  labelKey: string;
  fields?: Field[];
  actions?: Action[];
  className?: string;
};

export function useContentFields(data: Record<string, unknown> | undefined) {
  const { t, i18n } = useTranslation("runtime");
  const permissions = useContext(PresentationPermissions);
  const scope = (record: unknown) => ({
    record,
    data,
    context: { permissions },
  });
  const check = (condition: Expression | undefined, record: unknown) =>
    !condition?.kind || Boolean(evaluate(condition, scope(record)));

  function field(field: Field, record: unknown): ReactNode {
    if (!check(field.when, record)) return null;
    const value = field.value?.kind
      ? evaluate(field.value, scope(record))
      : field.path
        ? pathValue(record, field.path)
        : undefined;
    const raw = value == null ? "" : String(value);
    const args = Object.fromEntries(
      Object.entries(field.args ?? {}).map(([key, config]) => {
        if (!("expression" in config))
          return [key, evaluate(config, scope(record))];
        const value = evaluate(config.expression, scope(record));
        if (config.skipEmpty && (value == null || value === ""))
          return [key, ""];
        if (config.labelKey)
          return [key, t(config.labelKey, { [config.argument ?? key]: value })];
        return [
          key,
          config.format === "dateTime"
            ? formatMoment(i18n.language, String(value ?? ""))
            : value,
        ];
      }),
    );
    const dynamicLabelKey =
      field.labelPrefix && raw
        ? field.labelPrefix + raw + (field.labelSuffix ?? "")
        : undefined;
    const content = field.labelKey
      ? t(field.labelKey, args)
      : dynamicLabelKey
        ? field.fallbackValue &&
          !i18n.exists(dynamicLabelKey, { ns: "runtime" })
          ? String(evaluate(field.fallbackValue, scope(record)) ?? raw)
          : t(dynamicLabelKey, { defaultValue: raw })
        : field.format === "dateTime" && raw
          ? formatMoment(i18n.language, raw)
          : raw
            ? raw + (field.append ?? "")
            : field.fallbackKey
              ? t(field.fallbackKey)
              : "";
    const className = [
      field.className,
      field.classPrefix && raw ? field.classPrefix + raw : undefined,
    ]
      .filter(Boolean)
      .join(" ");
    if (field.format === "shortDate") return <ShortDate value={raw} />;
    if (field.format === "list" && Array.isArray(value) && value.length)
      return value.map((item, index) => (
        <small className={className} key={index}>
          {String(item)}
        </small>
      ));
    if (field.format === "strong")
      return <strong className={className}>{content}</strong>;
    if (field.format === "small" || field.format === "list")
      return <small className={className}>{content}</small>;
    if (field.format === "code")
      return <code className={className}>{content}</code>;
    return content ? <span className={className}>{content}</span> : null;
  }

  return { t, permissions, scope, field, check };
}

export function isInternalHref(value: unknown): value is string {
  return (
    typeof value === "string" &&
    /^\/[a-zA-Z0-9]/.test(value) &&
    !value.includes("\\")
  );
}

export function ActionControl({
  action,
  record,
  dataMap,
  readOnly,
  emit,
}: {
  action: Action;
  record?: unknown;
  dataMap?: Record<string, unknown>;
  readOnly: boolean;
  emit: ComponentRendererProps["emit"];
}) {
  const { t, permissions, check, scope } = useContentFields(dataMap);
  const [busy, setBusy] = useState(false);
  const operationKey = useRef<string | null>(null);
  if (
    (action.permission && !permissions.includes(action.permission)) ||
    !check(action.visible, record)
  )
    return null;
  const Icon = action.icon && contentIcons[action.icon];
  if (action.href?.kind) {
    const href = evaluate(action.href, scope(record));
    if (!isInternalHref(href)) return null;
    return (
      <Link className={action.className} to={href}>
        {Icon && (
          <span className="entity-symbol" aria-hidden>
            <Icon size={18} />
          </span>
        )}
        <span>{t(action.labelKey)}</span>
        {action.linkArrow && <ArrowUpRight size={14} aria-hidden />}
      </Link>
    );
  }
  return (
    <button
      type="button"
      className={action.className}
      disabled={readOnly || busy}
      onClick={async () => {
        setBusy(true);
        try {
          operationKey.current ??= crypto.randomUUID();
          const succeeded = await emit("action", {
            action: action.id,
            item: record,
            operationKey: operationKey.current,
            label: action.payloadLabelKey
              ? t(action.payloadLabelKey)
              : undefined,
          });
          if (succeeded !== false) operationKey.current = null;
        } finally {
          setBusy(false);
        }
      }}
    >
      {Icon && <Icon size={14} aria-hidden />}
      <span className={action.iconOnly ? "sr-only" : undefined}>
        {t(busy && action.pendingKey ? action.pendingKey : action.labelKey)}
      </span>
    </button>
  );
}

export function ContentHeading({
  props,
  data,
  dataMap,
  emit,
  readOnly,
}: ComponentRendererProps) {
  const { t } = useContentFields(dataMap);
  const action = props.action as Action | undefined;
  const Title = props.level === 2 ? "h2" : props.level === 3 ? "h3" : "strong";
  const EyebrowIcon = contentIcons[props.icon as keyof typeof contentIcons];
  const Description = props.level === 2 || props.level === 3 ? "p" : "small";
  return (
    <header className={String(props.className ?? "panel-heading")}>
      <div>
        {typeof props.eyebrowKey === "string" && (
          <span className="protocol-content-eyebrow">
            {EyebrowIcon && <EyebrowIcon size={14} aria-hidden />}
            {t(props.eyebrowKey)}
          </span>
        )}
        <Title>{t(String(props.titleKey ?? ""))}</Title>
        {typeof props.descriptionKey === "string" && (
          <Description>
            {t(
              props.descriptionKey,
              props.descriptionArgs as Record<string, unknown>,
            )}
          </Description>
        )}
      </div>
      {action && (
        <ActionControl
          action={action}
          record={data}
          dataMap={dataMap}
          readOnly={readOnly}
          emit={emit}
        />
      )}
    </header>
  );
}

export function ContentFields({
  props,
  data,
  dataMap,
}: ComponentRendererProps) {
  const { field } = useContentFields(dataMap);
  return (
    <div
      className={
        typeof props.className === "string" ? props.className : undefined
      }
    >
      {((props.fields ?? []) as Field[]).map((config, index) => (
        <Fragment key={index}>{field(config, data)}</Fragment>
      ))}
    </div>
  );
}

export function ContentNotice({
  props,
  dataMap,
  emit,
}: ComponentRendererProps) {
  const { t } = useTranslation("runtime");
  const args = Object.fromEntries(
    Object.entries((props.args ?? {}) as Record<string, Expression>).map(
      ([key, expression]) => [key, evaluate(expression, { data: dataMap })],
    ),
  );
  return (
    <SuccessNotice
      message={t(String(props.labelKey ?? ""), args)}
      onClose={() => {
        void emit("dismiss");
      }}
    />
  );
}

export function ContentTable({
  props,
  data,
  dataMap,
  emit,
  readOnly,
}: ComponentRendererProps) {
  const { t, field } = useContentFields(dataMap);
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState("");
  const searchFields = (props.searchFields ?? []) as string[];
  const filterField = String(props.filterField ?? "");
  const filters = (props.filters ?? []) as {
    value: string;
    labelKey: string;
  }[];
  const columns = (props.columns ?? []) as Column[];
  const rows = Array.isArray(data)
    ? data.filter(
        (row: unknown) =>
          (!filter || pathValue(row, filterField) === filter) &&
          (!search.trim() ||
            searchFields.some((path) =>
              String(pathValue(row, path) ?? "")
                .toLowerCase()
                .includes(search.trim().toLowerCase()),
            )),
      )
    : [];
  const searchKey = typeof props.searchKey === "string" ? props.searchKey : "";
  return (
    <>
      {searchKey && (
        <header className="ai-toolbar">
          <label className="ai-search">
            <span className="sr-only">{t(searchKey)}</span>
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t(searchKey)}
            />
          </label>
          {filters.length > 0 && (
            <div
              className="ai-filter"
              role="group"
              aria-label={t(String(props.filterLabelKey ?? searchKey))}
            >
              {filters.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  aria-pressed={filter === option.value}
                  onClick={() => setFilter(option.value)}
                >
                  {t(option.labelKey)}
                </button>
              ))}
            </div>
          )}
          {typeof props.countKey === "string" && (
            <span className="ai-count">
              {t(props.countKey, { total: rows.length })}
            </span>
          )}
        </header>
      )}
      {rows.length ? (
        <div className="admin-table-scroll">
          <Table>
            <TableHeader>
              <TableRow>
                {columns.map((column) => (
                  <TableHead key={column.labelKey}>
                    {t(column.labelKey)}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row, index) => (
                <TableRow
                  key={JSON.stringify(
                    Array.isArray(props.keyFields)
                      ? props.keyFields.map((name) =>
                          pathValue(row, String(name)),
                        )
                      : (pathValue(row, String(props.keyField ?? "id")) ??
                          index),
                  )}
                >
                  {columns.map((column) => (
                    <TableCell key={column.labelKey}>
                      <div className={column.className}>
                        {column.fields?.map((config, position) => (
                          <Fragment key={position}>
                            {field(config, row)}
                          </Fragment>
                        ))}
                        {column.actions?.map((action) => (
                          <ActionControl
                            key={action.id}
                            action={action}
                            record={row}
                            dataMap={dataMap}
                            emit={emit}
                            readOnly={readOnly}
                          />
                        ))}
                      </div>
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : (
        typeof props.emptyKey === "string" && (
          <p className="gov-empty">{t(props.emptyKey)}</p>
        )
      )}
    </>
  );
}

export function InlineInput({ props, emit, readOnly }: ComponentRendererProps) {
  const { t } = useTranslation("runtime");
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (readOnly || busy || !value.trim()) return;
    setBusy(true);
    try {
      const result = await emit("submit", { value: value.trim() });
      if (result !== false) setValue("");
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className={String(props.className ?? "")} onSubmit={submit}>
      <label htmlFor={String(props.inputId ?? "runtime-input")}>
        {t(String(props.labelKey ?? ""))}
      </label>
      <div>
        <input
          id={String(props.inputId ?? "runtime-input")}
          value={value}
          disabled={readOnly}
          maxLength={Number(props.maxLength ?? 200)}
          onChange={(event) => setValue(event.target.value)}
          placeholder={t(String(props.placeholderKey ?? ""))}
        />
        <button
          type="submit"
          className={String(props.buttonClassName ?? "")}
          disabled={readOnly || busy || !value.trim()}
        >
          <Plus size={14} aria-hidden />
          {t(String(busy ? props.pendingKey : props.submitKey))}
        </button>
      </div>
    </form>
  );
}

export function CardComponent({ props, children }: ComponentRendererProps) {
  const { t } = useTranslation("runtime");
  const titleKey = typeof props.titleKey === "string" ? props.titleKey : "";
  const descriptionKey =
    typeof props.descriptionKey === "string" ? props.descriptionKey : "";
  return (
    <section
      className={[
        "admin-panel",
        typeof props.className === "string" ? props.className : "",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      {(titleKey || descriptionKey) && (
        <header className="panel-heading">
          <div>
            {titleKey && <h2>{t(titleKey)}</h2>}
            {descriptionKey && <p>{t(descriptionKey)}</p>}
          </div>
        </header>
      )}
      {children}
    </section>
  );
}
