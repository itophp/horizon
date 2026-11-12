import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, ArrowRight, Copy, Ellipsis } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Panel, Status } from "./primitives";
import { projects } from "./catalog";

const boardColumns = [
  "boardBacklog",
  "boardDoing",
  "boardReviewStage",
  "boardDone",
] as const;
type BoardColumn = (typeof boardColumns)[number];
type Priority = "erpHigh" | "erpMedium" | "erpLow";
type BoardCard = {
  id: string;
  title: string;
  column: BoardColumn;
  priority: Priority;
  due: Date;
};
const initialCards: BoardCard[] = [
  {
    id: "w-1",
    title: projects[11],
    column: "boardBacklog",
    priority: "erpMedium",
    due: new Date(2026, 10, 18),
  },
  {
    id: "w-2",
    title: projects[7],
    column: "boardBacklog",
    priority: "erpLow",
    due: new Date(2026, 10, 24),
  },
  {
    id: "w-3",
    title: projects[0],
    column: "boardDoing",
    priority: "erpHigh",
    due: new Date(2026, 10, 12),
  },
  {
    id: "w-4",
    title: projects[1],
    column: "boardDoing",
    priority: "erpMedium",
    due: new Date(2026, 10, 20),
  },
  {
    id: "w-5",
    title: projects[2],
    column: "boardReviewStage",
    priority: "erpHigh",
    due: new Date(2026, 10, 11),
  },
  {
    id: "w-6",
    title: projects[4],
    column: "boardDone",
    priority: "erpLow",
    due: new Date(2026, 10, 6),
  },
];
const ganttTasks = [
  { name: projects[0], start: 0, span: 5, progress: 1, status: "done" },
  { name: projects[2], start: 2, span: 6, progress: 0.8, status: "active" },
  { name: projects[1], start: 5, span: 5, progress: 0.55, status: "active" },
  { name: projects[8], start: 7, span: 4, progress: 0.35, status: "review" },
  { name: projects[9], start: 9, span: 4, progress: 0.2, status: "review" },
  { name: projects[3], start: 11, span: 3, progress: 0.1, status: "active" },
].map((task) => ({ ...task, startDay: new Date(2026, 10, 9 + task.start) }));
const calendarDays = Array.from(
  { length: 30 },
  (_, i) => new Date(2026, 10, i + 1),
);
const busyDays = new Set([3, 8, 12, 15, 17, 20, 24, 26]);
const eventTypes = ["eventMeeting", "review", "eventDelivery"] as const;

function dayEvents(day: number) {
  if (!busyDays.has(day)) return [];
  const count = (day % 3) + 1;
  return Array.from({ length: count }, (_, i) => ({
    time: ["09:30", "14:00", "16:30"][(day + i) % 3],
    title: projects[(day * 5 + i * 3) % projects.length],
    type: eventTypes[(day + i) % eventTypes.length],
  }));
}

export function WorkViews() {
  const { t, i18n } = useTranslation("showcase");
  const [cards, setCards] = useState(initialCards);
  const [dragging, setDragging] = useState<string | null>(null);
  const [overColumn, setOverColumn] = useState<BoardColumn | null>(null);
  const [menuCard, setMenuCard] = useState<string | null>(null);
  const [boardAction, setBoardAction] = useState("");
  const [selectedDay, setSelectedDay] = useState(8);
  const monthDay = new Intl.DateTimeFormat(i18n.language, { day: "numeric" });
  const shortDate = new Intl.DateTimeFormat(i18n.language, {
    month: "short",
    day: "numeric",
  });
  const fullDate = new Intl.DateTimeFormat(i18n.language, {
    month: "long",
    day: "numeric",
    weekday: "long",
  });
  const weekday = new Intl.DateTimeFormat(i18n.language, {
    weekday: "short",
  });
  const percent = new Intl.NumberFormat(i18n.language, {
    style: "percent",
    maximumFractionDigits: 0,
  });
  const events = dayEvents(selectedDay);

  function moveCard(id: string, column: BoardColumn) {
    const card = cards.find((item) => item.id === id);
    if (!card || card.column === column) return;
    setCards((current) =>
      current.map((item) => (item.id === id ? { ...item, column } : item)),
    );
    setBoardAction(t("boardMoved", { card: card.title, column: t(column) }));
  }
  function duplicateCard(id: string) {
    const card = cards.find((item) => item.id === id);
    if (!card) return;
    let copyId = `${id}-copy`;
    let suffix = 2;
    while (cards.some((item) => item.id === copyId)) {
      copyId = `${id}-copy-${suffix}`;
      suffix += 1;
    }
    setCards((current) => [...current, { ...card, id: copyId }]);
    setBoardAction(
      t("boardDuplicated", { card: card.title, column: t(card.column) }),
    );
  }
  return (
    <TooltipProvider delay={0}>
      <Tabs defaultValue="board" className="sc-workviews">
        <TabsList variant="line">
          <TabsTrigger value="board">{t("tabBoard")}</TabsTrigger>
          <TabsTrigger value="gantt">{t("ganttTitle")}</TabsTrigger>
          <TabsTrigger value="calendar">{t("month")}</TabsTrigger>
        </TabsList>
        <TabsContent value="board">
          <p className="sc-muted">{t("boardIntro")}</p>
          <div className="sc-board">
            {boardColumns.map((column) => {
              const columnCards = cards.filter(
                (card) => card.column === column,
              );
              return (
                <section
                  key={column}
                  className="sc-board-column"
                  aria-label={t(column)}
                  data-drag-over={overColumn === column || undefined}
                  onDragOver={(event) => {
                    event.preventDefault();
                    setOverColumn(column);
                  }}
                  onDragLeave={() => setOverColumn(null)}
                  onDrop={(event) => {
                    event.preventDefault();
                    if (dragging) moveCard(dragging, column);
                    setDragging(null);
                    setOverColumn(null);
                  }}
                >
                  <header>
                    <h3>{t(column)}</h3>
                    <Badge variant="secondary">{columnCards.length}</Badge>
                  </header>
                  {columnCards.map((card) => (
                    <article
                      key={card.id}
                      className="sc-board-card"
                      draggable
                      data-dragging={dragging === card.id || undefined}
                      onDragStart={() => setDragging(card.id)}
                      onDragEnd={() => {
                        setDragging(null);
                        setOverColumn(null);
                      }}
                      onContextMenu={(event) => {
                        event.preventDefault();
                        setMenuCard(card.id);
                      }}
                    >
                      <div className="sc-board-card-main">
                        <strong>{card.title}</strong>
                        <span>
                          <Status value={card.priority} />
                          <small>{shortDate.format(card.due)}</small>
                        </span>
                      </div>
                      <DropdownMenu
                        open={menuCard === card.id}
                        onOpenChange={(open) =>
                          setMenuCard(open ? card.id : null)
                        }
                      >
                        <DropdownMenuTrigger
                          render={
                            <button
                              type="button"
                              className="sc-board-menu"
                              aria-label={t("boardActionsFor", {
                                card: card.title,
                              })}
                            />
                          }
                        >
                          <Ellipsis size={14} />
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-52">
                          <DropdownMenuGroup>
                            <DropdownMenuLabel>
                              {t("boardActions")}
                            </DropdownMenuLabel>
                            <DropdownMenuItem
                              disabled={column === boardColumns[0]}
                              onClick={() =>
                                moveCard(
                                  card.id,
                                  boardColumns[
                                    Math.max(
                                      0,
                                      boardColumns.indexOf(column) - 1,
                                    )
                                  ],
                                )
                              }
                            >
                              <ArrowLeft size={14} />
                              {t("boardMoveBack")}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              disabled={
                                column === boardColumns[boardColumns.length - 1]
                              }
                              onClick={() =>
                                moveCard(
                                  card.id,
                                  boardColumns[
                                    Math.min(
                                      boardColumns.length - 1,
                                      boardColumns.indexOf(column) + 1,
                                    )
                                  ],
                                )
                              }
                            >
                              <ArrowRight size={14} />
                              {t("boardMoveForward")}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => duplicateCard(card.id)}
                            >
                              <Copy size={14} />
                              {t("boardDuplicate")}
                            </DropdownMenuItem>
                          </DropdownMenuGroup>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </article>
                  ))}
                </section>
              );
            })}
          </div>
          <p className="sc-board-result" role="status">
            {boardAction}
          </p>
        </TabsContent>
        <TabsContent value="gantt">
          <div className="sc-pattern-controls">
            <span>{t("ganttWindow")}</span>
            <span>{t("ganttProgress")}</span>
          </div>
          <div className="sc-gantt">
            <div className="sc-gantt-row sc-gantt-head" aria-hidden="true">
              <span />
              {Array.from({ length: 14 }, (_, i) => (
                <span
                  key={i}
                  data-weekend={
                    new Date(2026, 10, 9 + i).getDay() === 0 || undefined
                  }
                >
                  {monthDay.format(new Date(2026, 10, 9 + i))}
                </span>
              ))}
            </div>
            {ganttTasks.map((task) => (
              <div className="sc-gantt-row" key={task.name}>
                <span className="sc-gantt-label">{task.name}</span>
                <div className="sc-gantt-track">
                  <span className="sc-gantt-today" aria-hidden="true" />
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <button
                          type="button"
                          className={`sc-gantt-bar sc-gantt-${task.status}`}
                          aria-label={task.name}
                          style={{
                            left: `${(task.start / 14) * 100}%`,
                            width: `${(task.span / 14) * 100}%`,
                          }}
                        />
                      }
                    >
                      <span
                        className="sc-gantt-progress"
                        style={{ width: `${task.progress * 100}%` }}
                      />
                    </TooltipTrigger>
                    <TooltipContent>
                      <strong>{task.name}</strong>
                      <br />
                      {t("ganttRange", {
                        start: shortDate.format(task.startDay),
                        end: shortDate.format(
                          new Date(2026, 10, 9 + task.start + task.span - 1),
                        ),
                      })}
                      <br />
                      {t("ganttProgressPercent", {
                        value: percent.format(task.progress),
                      })}
                    </TooltipContent>
                  </Tooltip>
                </div>
              </div>
            ))}
            <p className="sc-gantt-legend">
              <span className="sc-gantt-done">{t("done")}</span>
              <span className="sc-gantt-active">{t("active")}</span>
              <span className="sc-gantt-review">{t("review")}</span>
              <span className="sc-gantt-marker">{t("today")}</span>
            </p>
          </div>
        </TabsContent>
        <TabsContent value="calendar">
          <p className="sc-muted">{t("calendarIntro")}</p>
          <div className="sc-calendar-split">
            <div className="sc-calendar" role="grid" aria-label={t("month")}>
              {Array.from({ length: 7 }, (_, i) => (
                <span className="sc-calendar-weekday" key={i}>
                  {weekday.format(new Date(2026, 10, 1 + i))}
                </span>
              ))}
              {calendarDays.map((day) => {
                const dayEventsList = dayEvents(day.getDate());
                return (
                  <button
                    type="button"
                    key={day.toISOString()}
                    className="sc-calendar-day"
                    aria-pressed={selectedDay === day.getDate()}
                    onClick={() => setSelectedDay(day.getDate())}
                  >
                    <span>{monthDay.format(day)}</span>
                    {dayEventsList.slice(0, 2).map((event, i) => (
                      <small data-event={event.type} key={i}>
                        {event.time} {t(event.type)}
                      </small>
                    ))}
                    {dayEventsList.length > 2 && (
                      <small>
                        {t("calendarMore", { count: dayEventsList.length - 2 })}
                      </small>
                    )}
                  </button>
                );
              })}
            </div>
            <Panel title={t("calendarAgenda")}>
              <p className="sc-calendar-date">
                {fullDate.format(new Date(2026, 10, selectedDay))}
              </p>
              {events.length === 0 ? (
                <p className="sc-empty-note">{t("calendarNoEvents")}</p>
              ) : (
                <ul className="sc-agenda-list">
                  {events.map((event, i) => (
                    <li key={i}>
                      <span>{event.time}</span>
                      <div>
                        <strong>{event.title}</strong>
                        <Badge variant="secondary">{t(event.type)}</Badge>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </Panel>
          </div>
        </TabsContent>
      </Tabs>
    </TooltipProvider>
  );
}
