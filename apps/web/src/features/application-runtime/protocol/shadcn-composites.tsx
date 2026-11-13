import { useId, useState } from "react";
import { useLinkClickHandler, useSearchParams } from "react-router";
import { enUS, zhCN } from "react-day-picker/locale";
import { format, isValid, parse } from "date-fns";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import type {
  ComponentRenderer,
  ComponentRendererProps,
} from "./component-registry";
import { RuntimeChart } from "./shadcn-chart";
import { useBoundValue, useUI } from "./shadcn-props";

type DateRange = { from: string; to?: string };

function parseDate(value: unknown): Date | undefined {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return;
  const date = parse(value, "yyyy-MM-dd", new Date());
  if (isValid(date) && format(date, "yyyy-MM-dd") === value) return date;
}

function DateControl(p: ComponentRendererProps & { picker?: boolean }) {
  const { label, i18n, value, disabled, event, text, string, number } =
    useUI(p);
  const [open, setOpen] = useState(false);
  const range = p.props.mode === "range";
  const multiple = p.props.mode === "multiple";
  const rangeSource = p.props.range ?? value;
  const rangeValue =
    typeof rangeSource === "object" && rangeSource && "from" in rangeSource
      ? (rangeSource as DateRange)
      : undefined;
  const datesSource = p.props.dates ?? value;
  const source = JSON.stringify(
    range
      ? { from: rangeValue?.from, to: rangeValue?.to }
      : multiple
        ? Array.isArray(datesSource)
          ? datesSource.filter((item) => typeof item === "string").slice(0, 366)
          : []
        : typeof value === "string"
          ? value
          : null,
  );
  const [selection, setSelection] = useBoundValue(source);
  const selected: unknown = JSON.parse(selection);
  const date = parseDate(selected);
  const from =
    range && selected && typeof selected === "object" && "from" in selected
      ? parseDate(selected.from)
      : undefined;
  const to =
    range && selected && typeof selected === "object" && "to" in selected
      ? parseDate(selected.to)
      : undefined;
  const dates =
    multiple && Array.isArray(selected)
      ? selected.flatMap((item) => {
          const date = parseDate(item);
          return date ? [date] : [];
        })
      : [];
  const min = parseDate(string("minDate"));
  const max = parseDate(string("maxDate"));
  const shared = {
    "aria-label": label,
    locale: i18n.resolvedLanguage?.startsWith("zh") ? zhCN : enUS,
    defaultMonth: date ?? from ?? dates[0],
    numberOfMonths: Math.max(1, Math.min(3, Math.trunc(number("months") ?? 1))),
    disabled: disabled
      ? true
      : [...(min ? [{ before: min }] : []), ...(max ? [{ after: max }] : [])],
  };
  const dayKey = (day: Date) => format(day, "yyyy-MM-dd");
  function change(next: string | string[] | DateRange | null, close = false) {
    if (disabled) return;
    setSelection(JSON.stringify(next));
    event("change", next);
    if (close) setOpen(false);
  }
  const calendar = range ? (
    <Calendar
      {...shared}
      mode="range"
      resetOnSelect
      excludeDisabled
      selected={from ? { from, to } : undefined}
      onSelect={(next) => {
        if (next?.from)
          change(
            {
              from: dayKey(next.from),
              ...(next.to && { to: dayKey(next.to) }),
            },
            !!next.to,
          );
        else if (p.props.clearable === true) change(null);
      }}
    />
  ) : multiple ? (
    <Calendar
      {...shared}
      mode="multiple"
      selected={dates}
      onSelect={(next) => change((next ?? []).map(dayKey))}
    />
  ) : (
    <Calendar
      {...shared}
      mode="single"
      selected={date}
      onSelect={(next) => {
        if (next) change(dayKey(next), true);
        else if (p.props.clearable === true) change(null, true);
      }}
    />
  );
  if (!p.picker) return calendar;
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
  });
  const display =
    from && to
      ? formatter.formatRange(from, to)
      : (date ?? from)
        ? formatter.format(date ?? from)
        : dates.length
          ? new Intl.ListFormat(i18n.resolvedLanguage).format(
              dates.map((item) => formatter.format(item)),
            )
          : (text(p.props.placeholderKey) ?? label);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        aria-label={label}
        disabled={disabled}
        render={<Button variant="outline" />}
      >
        {display}
      </PopoverTrigger>
      <PopoverContent className="w-auto max-w-[95vw] overflow-auto">
        {calendar}
      </PopoverContent>
    </Popover>
  );
}

function PageNumber({
  href,
  page,
  active,
  label,
}: {
  href: string;
  page: string;
  active: boolean;
  label: string;
}) {
  const navigate = useLinkClickHandler<HTMLAnchorElement>(href);
  return (
    <PaginationItem>
      <PaginationLink
        href={href}
        isActive={active}
        aria-label={label}
        onClick={navigate}
      >
        {page}
      </PaginationLink>
    </PaginationItem>
  );
}

export const compositeRenderers = {
  calendar: DateControl,
  "date-picker": (p) => <DateControl {...p} picker />,
  chart: RuntimeChart,
  pagination: function PageNavigation(p) {
    const { label, t, i18n, number, string, disabled } = useUI(p);
    const [params] = useSearchParams();
    const param = string("param") ?? "page";
    const total = Math.max(1, Math.trunc(number("pages") ?? 1));
    const requested = Number(params.get(param));
    const current = Number.isSafeInteger(requested)
      ? Math.min(total, Math.max(1, requested))
      : 1;
    function href(page: number) {
      const next = new URLSearchParams(params);
      next.set(param, String(page));
      return "?" + next.toString();
    }
    const previous = href(current - 1);
    const next = href(current + 1);
    const goPrevious = useLinkClickHandler<HTMLAnchorElement>(previous);
    const goNext = useLinkClickHandler<HTMLAnchorElement>(next);
    const pages = [...new Set([1, current - 1, current, current + 1, total])]
      .filter((page) => page >= 1 && page <= total)
      .sort((a, b) => a - b);
    const formatPage = new Intl.NumberFormat(i18n.resolvedLanguage);
    return (
      <Pagination aria-label={label}>
        <PaginationContent>
          <PaginationItem>
            {current > 1 && !disabled && (
              <PaginationPrevious
                aria-label={t("components.previousPage")}
                href={previous}
                onClick={goPrevious}
                text={t("components.previousPage")}
              />
            )}
          </PaginationItem>
          {p.props.numbered === true && !disabled ? (
            pages.flatMap((page, index) => {
              const link = (
                <PageNumber
                  key={page}
                  href={href(page)}
                  page={formatPage.format(page)}
                  active={page === current}
                  label={t("components.goToPage", { page })}
                />
              );
              return index > 0 && page - pages[index - 1] > 1
                ? [
                    <PaginationItem key={`gap-${page}`}>
                      <PaginationEllipsis />
                    </PaginationItem>,
                    link,
                  ]
                : [link];
            })
          ) : (
            <PaginationItem aria-current="page">
              {t("components.pageOf", { current, total })}
            </PaginationItem>
          )}
          <PaginationItem>
            {current < total && !disabled && (
              <PaginationNext
                aria-label={t("components.nextPage")}
                href={next}
                onClick={goNext}
                text={t("components.nextPage")}
              />
            )}
          </PaginationItem>
        </PaginationContent>
      </Pagination>
    );
  },
  sidebar: function PageSidebar(p) {
    const { title } = useUI(p);
    const id = useId();
    return (
      <SidebarProvider
        defaultOpen={p.props.open === true}
        className="min-h-48 max-h-96"
      >
        <Sidebar variant="inset" id={id} embedded>
          {p.props.sections !== true && (
            <SidebarContent>
              {title && <h2 className="p-3 font-medium">{title}</h2>}
              {p.children}
            </SidebarContent>
          )}
          {p.props.sections === true && p.children}
        </Sidebar>
        <div className="flex-1">
          <SidebarTrigger aria-controls={id} />
        </div>
      </SidebarProvider>
    );
  },
  "sidebar-header": (p) => <SidebarHeader>{p.children}</SidebarHeader>,
  "sidebar-footer": (p) => <SidebarFooter>{p.children}</SidebarFooter>,
  "sidebar-content": (p) => <SidebarContent>{p.children}</SidebarContent>,
  "sidebar-group": function Group(p) {
    const { label } = useUI(p);
    return (
      <SidebarGroup>
        {label && <SidebarGroupLabel>{label}</SidebarGroupLabel>}
        <SidebarGroupContent>{p.children}</SidebarGroupContent>
      </SidebarGroup>
    );
  },
} satisfies Record<string, ComponentRenderer>;
