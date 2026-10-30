import { useState } from "react";
import { NotificationMarkdown } from "./markdown";
import { NotificationComposer } from "./notification-composer";
import { Link, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Bell } from "lucide-react";
import {
  notificationsMessage,
  notificationsSent,
  notificationsInbox,
  notificationsRead,
  notificationsDeliveries,
  notificationsRetry,
  notificationsTest,
} from "@/api/generated/baseline";
import { useSession } from "@/features/administration/auth";
import {
  PageHeading,
  DataTable,
  ShortDate,
  ErrorBanner,
  Loading,
  type AdminColumn,
} from "@/features/administration/shared";
import type { DeliveryRecord } from "@/api/generated/models";
import { Button } from "@/components/ui/button";
import { useGovKey, useGovTask } from "./shared";
function useInbox() {
  return useQuery({
    queryKey: useGovKey("inbox"),
    queryFn: () => notificationsInbox(),
    refetchInterval: 15000,
    retry: false,
  });
}
export function NotificationBell() {
  const { t } = useTranslation("governance");
  const q = useInbox();
  const count =
    q.data?.status === 200 ? q.data.data.filter((n) => !n.read).length : 0;
  return (
    <Link
      to="/notifications"
      className="console-icon-button gov-bell"
      aria-label={t("messages")}
    >
      <Bell size={18} />
      {count > 0 && <span>{count}</span>}
    </Link>
  );
}
export function NotificationsPage() {
  const { t } = useTranslation("governance");
  const session = useSession();
  const [tab, setTab] = useState("inbox");
  const tabs = [
    "inbox",
    ...(session.permissions.includes("notifications:send")
      ? ["compose", "sent"]
      : []),
    ...(session.permissions.includes("notifications:read") ||
    session.permissions.includes("notifications:test")
      ? ["deliveries"]
      : []),
  ];
  return (
    <>
      <PageHeading
        title={t("notificationCenter")}
        subtitle={t("notificationCenterHint")}
      />
      <div
        className="gov-tabs"
        role="tablist"
        aria-label={t("notificationCenter")}
      >
        {tabs.map((name) => (
          <button
            key={name}
            type="button"
            role="tab"
            aria-selected={tab === name}
            onClick={() => setTab(name)}
          >
            {t(`notificationTabs.${name}`)}
          </button>
        ))}
      </div>
      <div role="tabpanel">
        {tab === "inbox" ? (
          <InboxPanel />
        ) : tab === "compose" ? (
          <NotificationComposer onSent={() => setTab("sent")} />
        ) : tab === "sent" ? (
          <SentPanel />
        ) : (
          <DeliveryPanel />
        )}
      </div>
    </>
  );
}
function SentPanel() {
  const { t } = useTranslation("governance");
  const q = useQuery({
    queryKey: useGovKey("sent-notifications"),
    queryFn: () => notificationsSent(),
    retry: false,
  });
  return q.isPending ? (
    <Loading />
  ) : q.error ? (
    <ErrorBanner error={q.error} />
  ) : (
    <section className="admin-panel entity-card">
      {q.data?.status === 200 && q.data.data.length > 0 ? (
        q.data.data.map((n) => (
          <article className="gov-message" key={n.id}>
            <div>
              <h2>{n.title}</h2>
              <p>{t("recipientCount", { count: n.recipientCount })}</p>
              <ShortDate value={n.createdAt} />
              <NotificationMarkdown
                markdown={n.markdown}
                attachments={n.attachments}
              />
            </div>
          </article>
        ))
      ) : (
        <p className="gov-empty">{t("empty")}</p>
      )}
    </section>
  );
}
function InboxPanel() {
  const [params] = useSearchParams();
  const messageId = Number(params.get("message"));
  const selected = Number.isSafeInteger(messageId) && messageId > 0;
  const detail = useQuery({
    queryKey: useGovKey("message", messageId || 0),
    queryFn: () => notificationsMessage(messageId),
    enabled: selected,
    retry: false,
  });
  const { t } = useTranslation("governance");
  const q = useInbox();
  const task = useGovTask();
  const rows = selected
    ? detail.data?.status === 200
      ? [detail.data.data]
      : []
    : q.data?.status === 200
      ? q.data.data
      : [];
  const active = selected ? detail : q;
  return (
    <>
      {selected && (
        <Link className="notification-file-link" to="/notifications">
          {t("backToInbox")}
        </Link>
      )}
      {active.isPending ? (
        <Loading />
      ) : active.error ? (
        <ErrorBanner error={active.error} />
      ) : (
        <section className="admin-panel entity-card">
          {rows.length === 0 ? (
            <p className="gov-empty">{t("empty")}</p>
          ) : (
            rows.map((n) => (
              <article
                key={n.id}
                className={`gov-message ${n.read ? "" : "is-unread"}`}
              >
                <Bell size={18} />
                <div>
                  <strong>
                    {n.event === "announcement" ? n.title : t(n.event)}
                  </strong>
                  {n.event === "announcement" ? (
                    <NotificationMarkdown
                      markdown={n.markdown}
                      attachments={n.attachments}
                      messageId={n.id}
                    />
                  ) : (
                    <p>
                      {t("messageText", {
                        name: n.applicantName,
                        department: n.departmentName,
                        id: n.requestId,
                      })}
                    </p>
                  )}
                  <ShortDate value={n.createdAt} />
                  {n.event !== "announcement" && (
                    <Link to="/temporary-access">{t("goRequest")}</Link>
                  )}
                </div>
                <Button
                  variant="outline"
                  disabled={n.read || task.isPending}
                  onClick={() => task.mutate(() => notificationsRead(n.id))}
                >
                  {t(n.read ? "read" : "markRead")}
                </Button>
              </article>
            ))
          )}
        </section>
      )}
      {task.error && <ErrorBanner error={task.error} />}
    </>
  );
}
export function DeliveryPanel() {
  const { t } = useTranslation("governance");
  const session = useSession();
  const canRead = session.permissions.includes("notifications:read");
  const task = useGovTask();
  const q = useQuery({
    queryKey: useGovKey("deliveries"),
    queryFn: () => notificationsDeliveries(),
    enabled: canRead,
    retry: false,
    refetchInterval: 15000,
  });
  const columns: AdminColumn<DeliveryRecord>[] = [
    {
      id: "title",
      header: t("notificationTitle"),
      cell: (r) => r.title || t("systemNotification"),
    },
    { id: "recipient", header: t("recipients"), cell: (r) => r.recipientName },
    {
      id: "channel",
      header: t("channel"),
      cell: (r) => t(`channels.${r.channel}`),
    },
    {
      id: "created",
      header: t("history"),
      cell: (r) => <ShortDate value={r.createdAt} />,
    },
    {
      id: "status",
      header: t("status"),
      cell: (r) => <span className="scope-badge">{t(r.status)}</span>,
    },
    { id: "attempts", header: t("attempts"), cell: (r) => r.attempts },
    {
      id: "error",
      header: t("errorCode"),
      cell: (r) =>
        r.errorCode
          ? t(`deliveryErrors.${r.errorCode}`, { defaultValue: t("failed") })
          : "—",
    },
    {
      id: "actions",
      header: t("retry"),
      cell: (r) =>
        session.permissions.includes("notifications:retry") &&
        r.channel === "smtp" &&
        ["failed", "skipped"].includes(r.status) ? (
          <Button
            variant="outline"
            disabled={task.isPending}
            onClick={() => task.mutate(() => notificationsRetry(r.id))}
          >
            {t("retry")}
          </Button>
        ) : null,
    },
  ];
  if (!canRead && !session.permissions.includes("notifications:test"))
    return null;
  return (
    <section className="admin-panel entity-card gov-deliveries">
      <header className="gov-card-heading">
        <h2>{t("deliveries")}</h2>
        <p>{t("smtpHint")}</p>
        {session.permissions.includes("notifications:test") && (
          <div className="heading-actions">
            <Button
              variant="outline"
              disabled={task.isPending}
              onClick={() =>
                task.mutate(() => notificationsTest({ send: false }))
              }
            >
              {t("smtpTest")}
            </Button>
            <Button
              variant="outline"
              disabled={task.isPending}
              onClick={() =>
                task.mutate(() => notificationsTest({ send: true }))
              }
            >
              {t("smtpSend")}
            </Button>
          </div>
        )}
      </header>
      {task.error && <ErrorBanner error={task.error} />}{" "}
      {task.isSuccess && (
        <p role="status" className="gov-empty">
          {t("testSent")}
        </p>
      )}
      {canRead &&
        (q.error ? (
          <ErrorBanner error={q.error} />
        ) : (
          <DataTable
            rows={q.data?.status === 200 ? q.data.data : []}
            columns={columns}
          />
        ))}
    </section>
  );
}
