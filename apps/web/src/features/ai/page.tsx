import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  aiConversations,
  aiStatus,
  aiUpdateConversation,
} from "@/api/generated/baseline";
import type { AIConversation } from "@/api/generated/models";
import { ApiError } from "@/api/client/request";
import { PageHeading } from "@/features/administration/shared";
import { useSession } from "@/features/administration/auth";
import { Workbench } from "./workbench";
import { TaskList } from "./tasks";
import { useAiKey } from "./shared";
import "./ai.css";

// The workbench is one page with two panes: a task list on the left and the
// conversation on the right. A task list entry is a conversation, so opening one
// restores its turns and the next question continues the same thread.
export function AiPage() {
  const { t } = useTranslation(["ai", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const [firstSession] = useState(() => crypto.randomUUID());
  const [active, setActive] = useState<string>(firstSession);
  const [opened, setOpened] = useState<string[]>([firstSession]);
  const [localTasks, setLocalTasks] = useState<AIConversation[]>([]);
  const [running, setRunning] = useState<ReadonlySet<string>>(new Set());
  function open(sessionId: string) {
    setOpened((current) =>
      current.includes(sessionId) ? current : [...current, sessionId],
    );
    setActive(sessionId);
  }
  function create() {
    open(crypto.randomUUID());
  }

  const allowed = session.permissions.includes("assistant:use");
  const status = useQuery({
    queryKey: useAiKey("status"),
    queryFn: () => aiStatus(),
    retry: false,
    enabled: allowed,
    staleTime: 60000,
  });
  const conversationsKey = useAiKey("conversations");
  const conversations = useQuery({
    queryKey: conversationsKey,
    queryFn: () => aiConversations({ limit: 100 }),
    retry: false,
    enabled: allowed,
  });
  const snapshot = status.data?.status === 200 ? status.data.data : undefined;
  const storedTasks =
    conversations.data?.status === 200 ? conversations.data.data : [];
  const tasks = [
    ...storedTasks,
    ...localTasks.filter(
      (local) =>
        !storedTasks.some((stored) => stored.sessionId === local.sessionId),
    ),
  ];
  function runState(sessionId: string, title: string, busy: boolean) {
    setRunning((current) => {
      const next = new Set(current);
      if (busy) next.add(sessionId);
      else next.delete(sessionId);
      return next;
    });
    if (busy)
      setLocalTasks((current) =>
        current.some((item) => item.sessionId === sessionId)
          ? current
          : [
              ...current,
              {
                sessionId,
                title,
                messageCount: 1,
                updatedAt: new Date().toISOString(),
                archived: false,
              },
            ],
      );
  }

  // A rename or an archive is a property of the owner's ticket, not of a turn:
  // write it, then reload the list instead of guessing the new order locally.
  async function updateConversation(
    sessionId: string,
    body: { title?: string; archived?: boolean },
  ) {
    const result = await aiUpdateConversation(sessionId, body);
    if (result.status !== 200) {
      throw new ApiError(
        result.status === 404 ? "not_found" : "unavailable",
        result.status,
      );
    }
    await client.invalidateQueries({ queryKey: conversationsKey });
  }

  if (!allowed) {
    return (
      <section className="admin-page-heading">
        <div>
          <h1>{t("ai:title")}</h1>
          <p>{t("ai:forbidden")}</p>
        </div>
      </section>
    );
  }

  return (
    <>
      <PageHeading title={t("ai:title")} subtitle={t("ai:subtitle")}>
        {snapshot && (
          <Badge variant="outline">
            {snapshot.enabled ? t("ai:runtime.on") : t("ai:runtime.off")}
          </Badge>
        )}
        {snapshot && (
          <Badge variant="secondary">
            {t("ai:runtime.capabilities", { total: snapshot.capabilityCount })}
          </Badge>
        )}
      </PageHeading>
      <div className="ai-layout">
        <TaskList
          tasks={tasks}
          loading={conversations.isPending && !tasks.length}
          loadError={conversations.error}
          onRetry={() => void conversations.refetch()}
          active={
            tasks.some((item) => item.sessionId === active) ? active : null
          }
          runningSessionIds={running}
          onNew={create}
          onOpen={open}
          onRename={(sessionId, title) =>
            updateConversation(sessionId, { title })
          }
          onArchive={async (sessionId, archived) => {
            await updateConversation(sessionId, { archived });
            if (archived && active === sessionId) create();
          }}
        />
        {opened.map((id) => (
          <div key={id} hidden={id !== active} className="min-w-0">
            <Workbench
              sessionId={id}
              onRunState={runState}
              onTurnComplete={() => {
                void client.invalidateQueries({ queryKey: conversationsKey });
              }}
            />
          </div>
        ))}
      </div>
    </>
  );
}
