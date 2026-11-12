import { CardComposition } from "./card-composition";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { CircleCheck, Command, TriangleAlert, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { changeLanguage, i18n } from "@/i18n";
import { NotificationEditor } from "@/features/governance/notification-editor";
import { Choice, Panel, Status } from "./primitives";
import { patternIds, projects } from "./catalog";

type Toast = { id: number; kind: "success" | "error"; text: string };
type CommandItem = {
  id: string;
  label: string;
  group: "commandPages" | "commandAppearance";
  keywords: string;
  run: () => void;
};
type WorkOrder = {
  id: number;
  title: string;
  qty: number;
  status: "active" | "review" | "done";
};
const initialOrders: WorkOrder[] = [
  { id: 1, title: projects[0], qty: 24, status: "active" },
  { id: 2, title: projects[2], qty: 8, status: "review" },
  { id: 3, title: projects[5], qty: 46, status: "done" },
  { id: 4, title: projects[6], qty: 12, status: "active" },
];
const donutSegments = [
  { key: "donutPlanned", value: 34 },
  { key: "delivered", value: 41 },
  { key: "review", value: 17 },
  { key: "donutBlocked", value: 8 },
];
const areaValues = {
  weekly: [18, 24, 21, 32, 28, 41, 36, 46],
  monthly: [14, 18, 16, 24, 21, 29, 26, 35, 31, 42, 38, 51],
};

function useToasts() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const timers = useRef<number[]>([]);
  useEffect(
    () => () => timers.current.forEach((id) => window.clearTimeout(id)),
    [],
  );
  const dismiss = useCallback((id: number) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);
  const push = useCallback(
    (kind: Toast["kind"], text: string) => {
      const id = Date.now() + Math.random();
      setToasts((current) => [...current.slice(-2), { id, kind, text }]);
      timers.current.push(window.setTimeout(() => dismiss(id), 4000));
    },
    [dismiss],
  );
  return { toasts, push, dismiss };
}

export function InteractionLab() {
  const { t } = useTranslation("showcase");
  const [searchParams, setSearchParams] = useSearchParams();
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const [markdown, setMarkdown] = useState(() => t("richInitial"));
  const [markdownSaved, setMarkdownSaved] = useState(false);
  const [orders, setOrders] = useState(initialOrders);
  const [editing, setEditing] = useState<WorkOrder | null>(null);
  const [inlineError, setInlineError] = useState("");
  const [savedRow, setSavedRow] = useState<number | null>(null);
  const [range, setRange] = useState<"weekly" | "monthly">("weekly");
  const [tags, setTags] = useState(["Design", "Operations"]);
  const [tagDraft, setTagDraft] = useState("");
  const [tagError, setTagError] = useState("");
  const { toasts, push, dismiss } = useToasts();
  const percent = new Intl.NumberFormat(i18n.language, {
    style: "percent",
    maximumFractionDigits: 0,
  });
  const number = new Intl.NumberFormat(i18n.language);

  const commands = useMemo<CommandItem[]>(
    () => [
      ...patternIds.map(
        (id) =>
          ({
            id,
            label: t(id),
            group: "commandPages",
            keywords: id,
            run: () => {
              const next = new URLSearchParams(searchParams);
              next.set("pattern", id);
              setSearchParams(next);
            },
          }) satisfies CommandItem,
      ),
      {
        id: "theme",
        label: t("commandTheme"),
        group: "commandAppearance",
        keywords: "theme dark light contrast",
        run: () => document.documentElement.classList.toggle("dark"),
      },
      {
        id: "language",
        label: t("commandLanguage"),
        group: "commandAppearance",
        keywords: "language english chinese",
        run: () => void changeLanguage(i18n.language === "en" ? "zh-CN" : "en"),
      },
    ],
    [t, searchParams, setSearchParams],
  );
  const filtered = commands.filter((command) =>
    `${command.label} ${command.keywords}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
  const activeCommand = filtered[Math.min(active, filtered.length - 1)];

  useEffect(() => {
    function onKeyDown(event: globalThis.KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setQuery("");
        setActive(0);
        setPaletteOpen((open) => !open);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  function runCommand(command: CommandItem | undefined) {
    if (!command) return;
    command.run();
    setPaletteOpen(false);
    setQuery("");
    setActive(0);
    push("success", t("commandExecuted"));
  }
  function onPaletteKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActive((value) => Math.min(value + 1, filtered.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActive((value) => Math.max(value - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      runCommand(activeCommand);
    }
  }
  function addTag() {
    const value = tagDraft.trim();
    if (!value) return;
    if (tags.includes(value)) setTagError(t("tagsDuplicate"));
    else if (tags.length >= 6) setTagError(t("tagsMax"));
    else {
      setTags((current) => [...current, value]);
      setTagError("");
    }
    setTagDraft("");
  }
  function saveRow() {
    if (!editing) return;
    if (editing.title.trim().length < 2) setInlineError(t("inlineValidTitle"));
    else if (!Number.isFinite(editing.qty) || editing.qty < 1)
      setInlineError(t("inlineValidQty"));
    else {
      setOrders((current) =>
        current.map((order) => (order.id === editing.id ? editing : order)),
      );
      setSavedRow(editing.id);
      setEditing(null);
      setInlineError("");
    }
  }
  const circumference = 2 * Math.PI * 48;
  const donutRender = donutSegments.map((segment, index) => {
    const previous = donutSegments
      .slice(0, index)
      .reduce((sum, item) => sum + (item.value / 100) * circumference, 0);
    const length = (segment.value / 100) * circumference;
    return {
      key: segment.key,
      index,
      length,
      offset: previous,
    };
  });
  const values = areaValues[range];

  return (
    <div className="sc-lab">
      <CardComposition />
      <div className="sc-lab-grid">
        <Panel title={t("commandTitle")}>
          <p className="sc-muted">{t("commandHint")}</p>
          <p className="sc-muted">{t("patternIntro")}</p>
          <Button onClick={() => setPaletteOpen(true)}>
            <Command size={14} />
            {t("commandOpen")}
          </Button>
        </Panel>
        <Panel title={t("toastTitle")}>
          <p className="sc-muted">{t("toastDescription")}</p>
          <div className="sc-lab-actions">
            <Button
              variant="outline"
              onClick={() => push("success", t("toastSuccessText"))}
            >
              {t("toastSuccessAction")}
            </Button>
            <Button
              variant="outline"
              onClick={() => push("error", t("toastErrorText"))}
            >
              {t("toastErrorAction")}
            </Button>
          </div>
        </Panel>
        <Panel title={t("tagsTitle")}>
          <p className="sc-muted">{t("tagsHint")}</p>
          <div className="sc-tag-list">
            {tags.map((tag) => (
              <span className="sc-tag" key={tag}>
                {tag}
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t("tagRemove", { tag })}
                  onClick={() => setTags(tags.filter((item) => item !== tag))}
                >
                  <X size={12} />
                </Button>
              </span>
            ))}
          </div>
          <Input
            aria-label={t("tagsTitle")}
            placeholder={t("tagsPlaceholder")}
            value={tagDraft}
            onChange={(event) => setTagDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                addTag();
              }
            }}
          />
          {tagError && (
            <p role="alert" className="sc-inline-error">
              {tagError}
            </p>
          )}
        </Panel>
      </div>
      <Panel title={t("richTitle")}>
        <p className="sc-muted">{t("richHint")}</p>
        <div className="sc-rich-editor">
          <NotificationEditor
            value={markdown}
            onChange={(value) => {
              setMarkdown(value);
              setMarkdownSaved(false);
            }}
            onBlur={() => setMarkdownSaved(true)}
            ariaLabel={t("richLabel")}
          />
        </div>
        <p className="sc-rich-meta" role="status">
          {t("richChars", { count: markdown.length })}
          {markdownSaved && <Badge variant="secondary">{t("richSaved")}</Badge>}
        </p>
      </Panel>
      <Panel title={t("inlineTitle")}>
        <p className="sc-muted">{t("inlineHint")}</p>
        {inlineError && (
          <p role="alert" className="sc-inline-error">
            {inlineError}
          </p>
        )}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("name")}</TableHead>
              <TableHead>{t("inlineQty")}</TableHead>
              <TableHead>{t("status")}</TableHead>
              <TableHead>{t("detail")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {orders.map((order) => {
              const isEditing = editing?.id === order.id;
              return (
                <TableRow key={order.id}>
                  <TableCell>
                    {isEditing ? (
                      <Input
                        aria-label={t("name")}
                        value={editing.title}
                        onChange={(event) =>
                          setEditing({ ...editing, title: event.target.value })
                        }
                      />
                    ) : (
                      <strong>{order.title}</strong>
                    )}
                  </TableCell>
                  <TableCell>
                    {isEditing ? (
                      <Input
                        aria-label={t("inlineQty")}
                        type="number"
                        min={1}
                        value={editing.qty}
                        onChange={(event) =>
                          setEditing({
                            ...editing,
                            qty: Number(event.target.value),
                          })
                        }
                      />
                    ) : (
                      number.format(order.qty)
                    )}
                  </TableCell>
                  <TableCell>
                    <Status value={order.status} />
                  </TableCell>
                  <TableCell>
                    <div className="sc-inline-actions">
                      {isEditing ? (
                        <>
                          <Button size="sm" onClick={saveRow}>
                            {t("save")}
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setEditing(null);
                              setInlineError("");
                            }}
                          >
                            {t("cancel")}
                          </Button>
                        </>
                      ) : (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setEditing(order);
                            setSavedRow(null);
                            setInlineError("");
                          }}
                        >
                          {t("inlineEdit")}
                        </Button>
                      )}
                    </div>
                    {savedRow === order.id && (
                      <Badge variant="secondary">{t("inlineSaved")}</Badge>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </Panel>
      <div className="sc-lab-charts">
        <Card>
          <CardHeader>
            <CardTitle>{t("donutTitle")}</CardTitle>
            <CardDescription>{t("patternIntro")}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="sc-donut">
              <svg
                viewBox="0 0 120 120"
                role="img"
                aria-label={t("donutTitle")}
              >
                <title>{t("donutTitle")}</title>
                {donutRender.map((segment) => {
                  return (
                    <circle
                      key={segment.key}
                      className={`sc-donut-segment sc-donut-${segment.index}`}
                      cx="60"
                      cy="60"
                      r="48"
                      fill="none"
                      strokeWidth="16"
                      strokeDasharray={`${segment.length} ${circumference - segment.length}`}
                      strokeDashoffset={-segment.offset}
                    />
                  );
                })}
                <text
                  x="60"
                  y="57"
                  textAnchor="middle"
                  className="sc-donut-total"
                >
                  {t("donutTotal")}
                </text>
                <text
                  x="60"
                  y="74"
                  textAnchor="middle"
                  className="sc-donut-count"
                >
                  {percent.format(1)}
                </text>
              </svg>
              <ul className="sc-donut-legend">
                {donutSegments.map((segment, index) => (
                  <li key={segment.key}>
                    <span className={`sc-donut-dot sc-donut-${index}`} />
                    {t(segment.key)}
                    <strong>{percent.format(segment.value / 100)}</strong>
                  </li>
                ))}
              </ul>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <div className="sc-chart-heading">
              <div>
                <CardTitle>{t("areaTitle")}</CardTitle>
                <CardDescription>{t("areaSeries")}</CardDescription>
              </div>
              <Choice
                labelKey="date"
                value={range}
                options={["weekly", "monthly"]}
                onChange={(value) =>
                  setRange(value === "monthly" ? "monthly" : "weekly")
                }
              />
            </div>
          </CardHeader>
          <CardContent>
            <svg
              className="sc-area-chart"
              viewBox="0 0 700 220"
              role="img"
              aria-label={t("areaTitle")}
            >
              <title>{t("areaTitle")}</title>
              <defs>
                <linearGradient id="sc-area-fill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" className="sc-area-top" />
                  <stop offset="100%" className="sc-area-bottom" />
                </linearGradient>
              </defs>
              {[0, 25, 50, 75, 100].map((line) => (
                <line
                  key={line}
                  x1="40"
                  x2="680"
                  y1={190 - line * 1.5}
                  y2={190 - line * 1.5}
                  stroke="var(--app-line)"
                />
              ))}
              {(() => {
                const points = values.map(
                  (value, index) =>
                    `${40 + (index * 640) / (values.length - 1)},${190 - value * 2.4}`,
                );
                return (
                  <>
                    <path
                      d={`M ${points.join(" L ")} L 680,190 L 40,190 Z`}
                      fill="url(#sc-area-fill)"
                    />
                    <polyline
                      points={points.join(" ")}
                      fill="none"
                      stroke="var(--app-green)"
                      strokeWidth="3"
                    />
                    {values.map((value, index) => (
                      <circle
                        key={index}
                        cx={40 + (index * 640) / (values.length - 1)}
                        cy={190 - value * 2.4}
                        r="4"
                        fill="var(--app-green)"
                      />
                    ))}
                  </>
                );
              })()}
            </svg>
          </CardContent>
        </Card>
      </div>
      <div className="sc-toast-stack" aria-live="polite">
        {toasts.map((toast) => (
          <output className={`sc-toast sc-toast-${toast.kind}`} key={toast.id}>
            {toast.kind === "success" ? (
              <CircleCheck size={16} />
            ) : (
              <TriangleAlert size={16} />
            )}
            {toast.text}
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("toastDismiss")}
              onClick={() => dismiss(toast.id)}
            >
              <X size={13} />
            </Button>
          </output>
        ))}
      </div>
      <Dialog open={paletteOpen} onOpenChange={setPaletteOpen}>
        <DialogContent className="sc-command-dialog">
          <DialogHeader>
            <DialogTitle>{t("commandTitle")}</DialogTitle>
            <DialogDescription>{t("commandHint")}</DialogDescription>
          </DialogHeader>
          <Input
            autoFocus
            aria-label={t("commandTitle")}
            placeholder={t("commandPlaceholder")}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setActive(0);
            }}
            onKeyDown={onPaletteKeyDown}
          />
          <ul
            className="sc-command-list"
            role="listbox"
            aria-label={t("commandTitle")}
          >
            {filtered.map((command, index) => (
              <li key={command.id}>
                <button
                  type="button"
                  role="option"
                  aria-selected={index === active}
                  className="sc-command-item"
                  data-active={index === active || undefined}
                  onMouseEnter={() => setActive(index)}
                  onClick={() => runCommand(command)}
                >
                  <span>
                    <small>{t(command.group)}</small>
                    {command.label}
                  </span>
                  {command.group === "commandPages" && (
                    <Badge variant="secondary">{t(command.id)}</Badge>
                  )}
                </button>
              </li>
            ))}
            {filtered.length === 0 && (
              <li className="sc-command-empty">{t("commandEmpty")}</li>
            )}
          </ul>
        </DialogContent>
      </Dialog>
    </div>
  );
}
