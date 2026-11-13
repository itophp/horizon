import { useContext, useEffect, useId, useState, type ReactNode } from "react";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  Building2,
  Command,
  ListTree,
  Network,
  Plug,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Users,
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useConfiguration } from "@/features/governance/runtime";
import { evaluate } from "../expression";
import {
  PresentationPermissions,
  type ComponentRendererProps,
} from "./component-registry";
import { useWebMcp } from "@/features/webmcp/provider";
import type { Expression } from "./types";

const icons = {
  users: Users,
  building: Building2,
  shield: ShieldCheck,
  network: Network,
};

function useNumberFormat() {
  const { i18n } = useTranslation();
  const config = useConfiguration();
  return new Intl.NumberFormat(i18n.resolvedLanguage, {
    useGrouping: Boolean(config.value("display.grouping", true)),
  });
}

export function MetricValue({ props, dataMap }: ComponentRendererProps) {
  const { t } = useTranslation();
  const number = useNumberFormat();
  const value = props.value;
  if (typeof value !== "number" || !Number.isFinite(value)) return null;
  const Icon = icons[props.icon as keyof typeof icons];
  const args = Object.fromEntries(
    Object.entries((props.detailArgs ?? {}) as Record<string, Expression>).map(
      ([name, expression]) => [name, evaluate(expression, { data: dataMap })],
    ),
  );
  return (
    <div className="protocol-metric">
      <div className="protocol-metric-heading">
        <h2>{t(String(props.titleKey))}</h2>
        {Icon && <Icon size={20} aria-hidden />}
      </div>
      <strong>{number.format(value)}</strong>
      {typeof props.detailKey === "string" && (
        <p>
          <span className="stat-dot" aria-hidden />
          {t(props.detailKey, args)}
        </p>
      )}
    </div>
  );
}

type ChartPoint = { label: string; value: number };

function chartPoints(items: unknown, label: string): ChartPoint[] {
  if (!Array.isArray(items)) return [];
  return items.flatMap((item: unknown) => {
    if (item === null || typeof item !== "object") return [];
    const row = item as Record<string, unknown>;
    const name = row[label];
    const value = row.value;
    return typeof name === "string" &&
      typeof value === "number" &&
      Number.isFinite(value)
      ? [{ label: name, value }]
      : [];
  });
}

export function TimeSeries({ props, data }: ComponentRendererProps) {
  const { t, i18n } = useTranslation();
  const number = useNumberFormat();
  const label = useId();
  const fill = useId();
  const formatDate = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  });
  const series = chartPoints(props.items ?? data, "date").flatMap((point) => {
    const date = new Date(point.label);
    return Number.isNaN(date.valueOf())
      ? []
      : [{ ...point, formatted: formatDate.format(date) }];
  });
  if (!series.length)
    return <p className="gov-empty">{t("runtime:protocol.empty")}</p>;

  const step = Math.max(
    1,
    Math.ceil(Math.max(...series.map((point) => point.value)) / 3),
  );
  const ticks = [3 * step, 2 * step, step, 0];
  const positions = series.map(({ value }, index) => ({
    x: series.length > 1 ? (index / (series.length - 1)) * 660 : 330,
    y: 164 - (value / ticks[0]) * 138,
  }));

  const line = positions.map(({ x, y }) => `${x},${y}`).join(" ");
  const first = positions[0];
  const last = positions[positions.length - 1];

  return (
    <figure className="growth-chart" aria-labelledby={label}>
      <figcaption className="sr-only" id={label}>
        {t(String(props.labelKey))}
      </figcaption>
      <div className="chart-labels" aria-hidden="true">
        {ticks.map((tick) => (
          <span key={tick}>{number.format(tick)}</span>
        ))}
      </div>
      <div className="chart-drawing">
        <svg
          viewBox="0 0 660 180"
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          <defs>
            <linearGradient id={fill} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="currentColor" stopOpacity={0.22} />
              <stop offset="100%" stopColor="currentColor" stopOpacity={0} />
            </linearGradient>
          </defs>
          {[26, 72, 118, 164].map((y) => (
            <line
              key={y}
              x1="0"
              y1={y}
              x2="660"
              y2={y}
              className="chart-gridline"
            />
          ))}
          {positions.length > 1 && (
            <polygon
              points={`${line} ${last.x},164 ${first.x},164`}
              fill={`url(#${fill})`}
            />
          )}
          <polyline
            points={line}
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            vectorEffect="non-scaling-stroke"
          />
          {positions.map(({ x, y }) => (
            <circle
              key={x}
              cx={x}
              cy={y}
              r={3}
              fill="var(--app-panel)"
              stroke="currentColor"
            />
          ))}
        </svg>
        <div className="chart-dates" aria-hidden="true">
          {series.map((point) => (
            <span key={point.label}>{point.formatted}</span>
          ))}
        </div>
      </div>
      <ul className="sr-only">
        {series.map((point) => (
          <li key={point.label}>
            <time dateTime={point.label}>{point.formatted}</time>{" "}
            <data value={point.value}>{number.format(point.value)}</data>
          </li>
        ))}
      </ul>
    </figure>
  );
}

export function HorizontalBars({ props, data }: ComponentRendererProps) {
  const { t } = useTranslation();
  const number = useNumberFormat();
  const rows = chartPoints(props.items ?? data, "label");
  if (!rows.length)
    return <p className="gov-empty">{t("runtime:protocol.empty")}</p>;
  const maximum = Math.max(
    1,
    ...rows.map(({ value }) => value),
    typeof props.maximum === "number" && Number.isFinite(props.maximum)
      ? props.maximum
      : 0,
  );
  return (
    <ul
      className="department-bars protocol-bars"
      aria-label={t(String(props.labelKey))}
    >
      {rows.map(({ label, value }, index) => (
        <li key={`${label}-${index}`}>
          <div>
            <span>{label}</span>
            <strong>{number.format(value)}</strong>
          </div>
          <span className="bar-track" aria-hidden="true">
            <i style={{ width: `${Math.max(0, (value / maximum) * 100)}%` }} />
          </span>
        </li>
      ))}
    </ul>
  );
}

export function DistributionPie({ props, data }: ComponentRendererProps) {
  const { t, i18n } = useTranslation();
  const titleId = useId();
  const number = useNumberFormat();
  const percentage = new Intl.NumberFormat(i18n.resolvedLanguage, {
    style: "percent",
    maximumFractionDigits: 1,
  });
  const rows = chartPoints(props.items ?? data, "label").filter(
    ({ value }) => value >= 0,
  );
  const total = rows.reduce((sum, row) => sum + row.value, 0);
  let offset = 0;
  const segments = [];
  for (const [index, row] of rows.entries()) {
    const start = offset;
    offset += row.value;
    segments.push({
      ...row,
      color: `var(--pie-category-${(index % 5) + 1})`,
      start: total ? start / total : 0,
      end: total ? offset / total : 0,
    });
  }
  const stops = segments
    .filter(({ value }) => value > 0)
    .map(({ color, start, end }) => `${color} ${start * 100}% ${end * 100}%`)
    .join(", ");
  return (
    <figure className="protocol-distribution-pie" aria-labelledby={titleId}>
      <figcaption>
        <strong id={titleId}>{t(String(props.labelKey))}</strong>
        {typeof props.descriptionKey === "string" && (
          <small>{t(props.descriptionKey)}</small>
        )}
      </figcaption>
      {total > 0 ? (
        <>
          <div
            className="protocol-pie-disc"
            aria-hidden
            style={{ background: `conic-gradient(${stops})` }}
          />
          <ul className="protocol-pie-legend">
            {segments.map(({ label, value, color }, index) => (
              <li key={`${label}-${index}`}>
                <span
                  className="protocol-pie-swatch"
                  aria-hidden
                  style={{ background: color }}
                />
                <span>{label}</span>
                <span>
                  {`${number.format(value)} · ${percentage.format(value / total)}`}
                </span>
              </li>
            ))}
          </ul>
        </>
      ) : (
        <p className="gov-empty protocol-pie-empty">
          {t("runtime:protocol.empty")}
        </p>
      )}
    </figure>
  );
}

type LayoutTab = {
  id: string;
  labelKey: string;
  permission?: string;
  icon?: keyof typeof tabIcons;
  webMcpCount?: boolean;
};

const tabIcons = {
  command: Command,
  plug: Plug,
  sparkles: Sparkles,
  sliders: SlidersHorizontal,
  tree: ListTree,
};

function WebMcpTabLabel({ label }: { label: string }) {
  const { t } = useTranslation("mcp");
  const { status, toolCount } = useWebMcp();
  if (status !== "connected") return label;
  return (
    <>
      <span aria-hidden>{label}</span>
      <span aria-hidden className="mcp-hub-count">
        {toolCount}
      </span>
      <span className="sr-only">
        {t("hub.pageTabAria", { count: toolCount })}
      </span>
    </>
  );
}

export function LayoutTabs({
  props,
  children,
  readOnly,
}: ComponentRendererProps) {
  const { t } = useTranslation();
  const permissions = useContext(PresentationPermissions);
  const [params, setParams] = useSearchParams();
  const tabs = ((props.tabs ?? []) as LayoutTab[]).filter(
    (tab) => !tab.permission || permissions.includes(tab.permission),
  );
  const param = typeof props.param === "string" ? props.param : "tab";
  const sessionKey =
    typeof props.sessionKey === "string" ? props.sessionKey : undefined;
  const storageKey = readOnly ? undefined : sessionKey;
  const requested = params.get(param);
  const [remembered, setRemembered] = useState(() => {
    if (!storageKey) return null;
    if (tabs.some(({ id }) => id === requested)) return requested;
    try {
      return sessionStorage.getItem(storageKey);
    } catch {
      return null;
    }
  });
  const current =
    tabs.find(({ id }) => id === requested) ??
    tabs.find(({ id }) => id === remembered) ??
    tabs[0];
  const activeId = current?.id;
  useEffect(() => {
    if (!sessionKey || !activeId) return;
    if (requested === activeId) return;
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        next.set(param, activeId);
        return next;
      },
      { replace: true },
    );
  }, [activeId, param, requested, sessionKey, setParams]);

  if (!current) return <p role="alert">{t("runtime:denied")}</p>;
  return (
    <Tabs
      className="protocol-layout-tabs"
      value={current.id}
      onValueChange={(value) => {
        if (!tabs.some(({ id }) => id === value)) return;
        if (storageKey) {
          setRemembered(value);
          try {
            sessionStorage.setItem(storageKey, value);
          } catch {
            // Switching tabs still works when storage is blocked.
          }
        }
        setParams(
          (previous) => {
            const next = new URLSearchParams(previous);
            next.set(param, value);
            return next;
          },
          { replace: Boolean(storageKey) },
        );
      }}
    >
      {(!props.hideSingleTab || tabs.length > 1) && (
        <TabsList
          className="overview-tabs"
          aria-label={props.labelKey ? t(String(props.labelKey)) : undefined}
        >
          {tabs.map((tab) => {
            const Icon = tab.icon && tabIcons[tab.icon];
            const label = t(tab.labelKey);
            return (
              <TabsTrigger key={tab.id} value={tab.id}>
                {Icon && <Icon size={15} aria-hidden />}
                {tab.webMcpCount && !readOnly ? (
                  <WebMcpTabLabel label={label} />
                ) : (
                  label
                )}
              </TabsTrigger>
            );
          })}
        </TabsList>
      )}
      {children}
    </Tabs>
  );
}

export function LayoutTabPanel({
  props,
  children,
}: {
  props: Record<string, unknown>;
  children?: ReactNode;
}) {
  if (typeof props.tab !== "string") return null;
  return <TabsContent value={props.tab}>{children}</TabsContent>;
}
