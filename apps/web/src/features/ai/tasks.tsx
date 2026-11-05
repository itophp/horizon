import { TaskControls, type TaskSort } from "./task-controls";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Loader2 } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { Empty, EmptyDescription, EmptyHeader } from "@/components/ui/empty";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Archive, ArchiveRestore, Check, Pencil, X } from "lucide-react";
import type { AIConversation } from "@/api/generated/models";
import { ErrorBanner } from "@/features/administration/shared";
import { formatMoment } from "@/lib/capability";

// Task list orders. The server returns conversations newest first; the list
// keeps that as the default and offers the two orders an owner actually needs:
// the oldest thread and an alphabetical scan of renamed titles.
export type { TaskSort } from "./task-controls";

export function sortTasks(
  tasks: AIConversation[],
  sort: TaskSort,
  locale: string,
): AIConversation[] {
  const ordered = [...tasks];
  if (sort === "title") {
    return ordered.sort((left, right) =>
      left.title.localeCompare(right.title, locale),
    );
  }
  return ordered.sort((left, right) => {
    const diff = Date.parse(left.updatedAt) - Date.parse(right.updatedAt);
    return sort === "recent" ? -diff : diff;
  });
}

// TaskList is the left pane of the workbench. A conversation carries its own
// title and archive flag, so renaming and archiving stay local to the owner and
// never touch the recorded turns.
export function TaskList({
  tasks,
  active,
  onNew,
  onOpen,
  onRename,
  onArchive,
  loading = false,
  loadError,
  onRetry,
  runningSessionIds = new Set<string>(),
}: {
  loading?: boolean;
  loadError?: unknown;
  onRetry?: () => void;
  runningSessionIds?: ReadonlySet<string>;
  tasks: AIConversation[];
  active: string | null;
  onNew: () => void;
  onOpen: (sessionId: string) => void;
  onRename: (sessionId: string, title: string) => Promise<void>;
  onArchive: (sessionId: string, archived: boolean) => Promise<void>;
}) {
  const { t, i18n } = useTranslation("ai");
  const [view, setView] = useState<"active" | "archived">("active");
  const [sort, setSort] = useState<TaskSort>("recent");
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);

  const activeCount = tasks.filter((task) => !task.archived).length;
  const archivedCount = tasks.length - activeCount;
  const visible = useMemo(
    () =>
      sortTasks(
        tasks.filter((task) => task.archived === (view === "archived")),
        sort,
        i18n.language,
      ),
    [tasks, view, sort, i18n.language],
  );

  function startRename(task: AIConversation) {
    setError(null);
    setEditing(task.sessionId);
    setDraft(task.title);
  }

  function stopRename() {
    setEditing(null);
    setDraft("");
  }

  async function commit(sessionId: string) {
    const title = draft.trim();
    if (!title || pending) return;
    setPending(sessionId);
    setError(null);
    try {
      await onRename(sessionId, title);
      stopRename();
    } catch (caught) {
      setError(caught);
    } finally {
      setPending(null);
    }
  }

  async function toggleArchive(task: AIConversation) {
    if (pending) return;
    setPending(task.sessionId);
    setError(null);
    try {
      await onArchive(task.sessionId, !task.archived);
    } catch (caught) {
      setError(caught);
    } finally {
      setPending(null);
    }
  }

  return (
    <aside className="ai-tasks" aria-label={t("tasks.title")}>
      <div className="ai-tasks-heading">
        <strong>{t("tasks.title")}</strong>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onNew()}
          aria-pressed={active === null}
        >
          {t("tasks.new")}
        </Button>
      </div>
      <TaskControls
        view={view}
        setView={setView}
        sort={sort}
        setSort={setSort}
        activeCount={activeCount}
        archivedCount={archivedCount}
      />
      {!!error && <ErrorBanner error={error} />}
      {loadError ? (
        <ErrorBanner error={loadError} onRetry={onRetry} />
      ) : loading ? (
        <Skeleton className="h-24 w-full" />
      ) : !visible.length ? (
        <Empty className="border-0 p-4">
          <EmptyHeader>
            <EmptyDescription>
              {view === "archived"
                ? t("tasks.archivedEmpty")
                : t("tasks.empty")}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <ul className="ai-task-list">
          {visible.map((task) => (
            <li
              key={task.sessionId}
              className={active === task.sessionId ? "is-active" : undefined}
            >
              {editing === task.sessionId ? (
                <form
                  className="ai-task-rename"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void commit(task.sessionId);
                  }}
                >
                  <Input
                    autoFocus
                    value={draft}
                    maxLength={200}
                    aria-label={t("tasks.renameLabel")}
                    onChange={(event) => setDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Escape") stopRename();
                    }}
                  />
                  <Button
                    type="submit"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={t("tasks.renameSave")}
                    title={t("tasks.renameSave")}
                    disabled={!draft.trim() || pending === task.sessionId}
                  >
                    <Check size={13} aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={t("tasks.renameCancel")}
                    title={t("tasks.renameCancel")}
                    onClick={stopRename}
                  >
                    <X size={13} aria-hidden="true" />
                  </Button>
                </form>
              ) : (
                <>
                  <Button
                    type="button"
                    variant="ghost"
                    className="ai-task-open h-auto items-start whitespace-normal"
                    aria-current={
                      active === task.sessionId ? "true" : undefined
                    }
                    onClick={() => onOpen(task.sessionId)}
                  >
                    <strong>{task.title}</strong>
                    {runningSessionIds.has(task.sessionId) && (
                      <span className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Loader2
                          className="size-3 animate-spin motion-reduce:animate-none"
                          aria-hidden="true"
                        />
                        {t("workbench.running")}
                      </span>
                    )}
                    <small>
                      {t("tasks.meta", {
                        count: task.messageCount,
                        time: formatMoment(i18n.language, task.updatedAt),
                      })}
                    </small>
                  </Button>
                  <div className="ai-task-actions">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      aria-label={t("tasks.renameTask", { title: task.title })}
                      title={t("tasks.rename")}
                      disabled={pending === task.sessionId}
                      onClick={() => startRename(task)}
                    >
                      <Pencil size={13} aria-hidden="true" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      aria-label={
                        task.archived
                          ? t("tasks.restoreTask", { title: task.title })
                          : t("tasks.archiveTask", { title: task.title })
                      }
                      title={
                        task.archived ? t("tasks.restore") : t("tasks.archive")
                      }
                      disabled={pending === task.sessionId}
                      onClick={() => void toggleArchive(task)}
                    >
                      {task.archived ? (
                        <ArchiveRestore size={13} aria-hidden="true" />
                      ) : (
                        <Archive size={13} aria-hidden="true" />
                      )}
                    </Button>
                  </div>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </aside>
  );
}
