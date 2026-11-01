import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Archive,
  ArchiveRestore,
  Check,
  ListFilter,
  Pencil,
  X,
} from "lucide-react";
import type { AIConversation } from "@/api/generated/models";
import { ErrorBanner } from "@/features/administration/shared";
import { formatMoment } from "@/lib/capability";

// Task list orders. The server returns conversations newest first; the list
// keeps that as the default and offers the two orders an owner actually needs:
// the oldest thread and an alphabetical scan of renamed titles.
export type TaskSort = "recent" | "oldest" | "title";

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
}: {
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
        <button
          type="button"
          className="ai-task-new"
          onClick={() => onNew()}
          aria-pressed={active === null}
        >
          {t("tasks.new")}
        </button>
      </div>
      <div className="ai-tasks-tools">
        <div className="ai-task-views">
          <button
            type="button"
            aria-pressed={view === "active"}
            onClick={() => setView("active")}
          >
            {t("tasks.views.active")}
            <small>{activeCount}</small>
          </button>
          <button
            type="button"
            aria-pressed={view === "archived"}
            onClick={() => setView("archived")}
          >
            {t("tasks.views.archived")}
            <small>{archivedCount}</small>
          </button>
        </div>
        <label className="ai-task-sort">
          <ListFilter size={13} aria-hidden="true" />
          <span className="sr-only">{t("tasks.sort.label")}</span>
          <select
            aria-label={t("tasks.sort.label")}
            value={sort}
            onChange={(event) => setSort(event.target.value as TaskSort)}
          >
            <option value="recent">{t("tasks.sort.recent")}</option>
            <option value="oldest">{t("tasks.sort.oldest")}</option>
            <option value="title">{t("tasks.sort.title")}</option>
          </select>
        </label>
      </div>
      {!!error && <ErrorBanner error={error} />}
      {!visible.length ? (
        <p className="gov-empty">
          {view === "archived" ? t("tasks.archivedEmpty") : t("tasks.empty")}
        </p>
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
                  <input
                    autoFocus
                    value={draft}
                    maxLength={200}
                    aria-label={t("tasks.renameLabel")}
                    onChange={(event) => setDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Escape") stopRename();
                    }}
                  />
                  <button
                    type="submit"
                    className="ai-task-icon"
                    aria-label={t("tasks.renameSave")}
                    title={t("tasks.renameSave")}
                    disabled={!draft.trim() || pending === task.sessionId}
                  >
                    <Check size={13} aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    className="ai-task-icon"
                    aria-label={t("tasks.renameCancel")}
                    title={t("tasks.renameCancel")}
                    onClick={stopRename}
                  >
                    <X size={13} aria-hidden="true" />
                  </button>
                </form>
              ) : (
                <>
                  <button
                    type="button"
                    className="ai-task-open"
                    aria-current={
                      active === task.sessionId ? "true" : undefined
                    }
                    onClick={() => onOpen(task.sessionId)}
                  >
                    <strong>{task.title}</strong>
                    <small>
                      {t("tasks.meta", {
                        count: task.messageCount,
                        time: formatMoment(i18n.language, task.updatedAt),
                      })}
                    </small>
                  </button>
                  <div className="ai-task-actions">
                    <button
                      type="button"
                      className="ai-task-icon"
                      aria-label={t("tasks.renameTask", { title: task.title })}
                      title={t("tasks.rename")}
                      disabled={pending === task.sessionId}
                      onClick={() => startRename(task)}
                    >
                      <Pencil size={13} aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      className="ai-task-icon"
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
                    </button>
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
