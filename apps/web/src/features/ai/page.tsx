import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  aiConversations,
  aiStatus,
  aiUpdateConversation,
} from "@/api/generated/baseline";
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
  const [active, setActive] = useState<string | null>(null);
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
  const tasks =
    conversations.data?.status === 200 ? conversations.data.data : [];

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
          <span
            className={`ai-badge ${snapshot.enabled ? "tone-1" : "tone-3"}`}
          >
            {snapshot.enabled ? t("ai:runtime.on") : t("ai:runtime.off")}
          </span>
        )}
        {snapshot && (
          <span className="ai-badge">
            {t("ai:runtime.capabilities", { total: snapshot.capabilityCount })}
          </span>
        )}
      </PageHeading>
      <div className="ai-layout">
        <TaskList
          tasks={tasks}
          active={active}
          onNew={() => setActive(null)}
          onOpen={setActive}
          onRename={(sessionId, title) =>
            updateConversation(sessionId, { title })
          }
          onArchive={async (sessionId, archived) => {
            await updateConversation(sessionId, { archived });
            if (archived && active === sessionId) setActive(null);
          }}
        />
        <Workbench
          key={active ?? "new"}
          sessionId={active}
          onTurnComplete={() => {
            void client.invalidateQueries({ queryKey: conversationsKey });
          }}
        />
      </div>
    </>
  );
}
