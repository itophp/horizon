import { NotificationMarkdown } from "./markdown";
import { Link, Navigate, useParams, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Bell, PenLine, ArrowLeft } from "lucide-react";
import {
  notificationsMessage,
  notificationsSent,
  notificationsInbox,
  notificationsRead,
  notificationsDeliveries,
  notificationsRetry,
  notificationsTest,
} from "@/lib/capabilities";
import { useSession } from "@/features/administration/auth";
import {
  PageHeading,
  DataTable,
  ShortDate,
  ErrorBanner,
  Loading,
  type AdminColumn,
} from "@/features/administration/shared";
import type { DeliveryRecord } from "@/api/types";
import { Button, buttonVariants } from "@/components/ui/button";
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
  const [params, setParams] = useSearchParams();
  const canSend = session.permissions.includes("notifications:send");
  const canReadDeliveries =
    session.permissions.includes("notifications:read") ||
    session.permissions.includes("notifications:test");
  const tabs = [
    "inbox",
    ...(canSend ? ["sent"] : []),
    ...(canReadDeliveries ? ["deliveries"] : []),
  ];
  const requested = params.get("tab");
  const tab = requested && tabs.includes(requested) ? requested : "inbox";
  const select = (name: string) =>
    setParams(name === "inbox" ? {} : { tab: name }, { replace: true });
  // 邮件中的历史链接仍是 ?message=<id>，统一跳转到独立的通知详情页。
  const legacyMessage = Number(params.get("message"));
  if (Number.isSafeInteger(legacyMessage) && legacyMessage > 0)
    return <Navigate to={`/notifications/${legacyMessage}`} replace />;
  return (
    <>
      <PageHeading
        title={t("notificationCenter")}
        subtitle={t("notificationCenterHint")}
      >
        {canSend && (
          <Link
            to="/notifications/compose"
            className={buttonVariants()}
            data-slot="button"
          >
            <PenLine size={16} aria-hidden="true" />
            {t("composeNotification")}
          </Link>
        )}
      </PageHeading>
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
            onClick={() => select(name)}
          >
            {t(`notificationTabs.${name}`)}
          </button>
        ))}
      </div>
      <div role="tabpanel">
        {tab === "inbox" ? (
          <InboxPanel />
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
          <article className="gov-message gov-sent" key={n.id}>
            <div>
              <div className="gov-message-summary">
                <strong>{n.title}</strong>
                <p>{t("recipientCount", { count: n.recipientCount })}</p>
                <ShortDate value={n.createdAt} />
              </div>
              <details className="gov-message-content">
                <summary>{t("viewContent")}</summary>
                <NotificationMarkdown
                  markdown={n.markdown}
                  attachments={n.attachments}
                />
              </details>
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
  const { t } = useTranslation("governance");
  const q = useInbox();
  const task = useGovTask();
  const rows = q.data?.status === 200 ? q.data.data : [];
  return (
    <section className="admin-panel entity-card">
      {q.isPending ? (
        <Loading />
      ) : q.error ? (
        <ErrorBanner error={q.error} />
      ) : rows.length === 0 ? (
        <p className="gov-empty">{t("empty")}</p>
      ) : (
        rows.map((n) => (
          <article
            key={n.id}
            className={`gov-message ${n.read ? "" : "is-unread"}`}
          >
            <Bell size={18} />
            <div>
              <Link
                className="gov-message-summary"
                to={`/notifications/${n.id}`}
              >
                <strong>
                  {n.event === "announcement" ? n.title : t(n.event)}
                </strong>
                <ShortDate value={n.createdAt} />
              </Link>
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
      {task.error && <ErrorBanner error={task.error} />}
    </section>
  );
}
export function NotificationDetailPage() {
  const { t } = useTranslation("governance");
  const { id } = useParams();
  const messageId = Number(id);
  const valid = Number.isSafeInteger(messageId) && messageId > 0;
  const task = useGovTask();
  const q = useQuery({
    queryKey: useGovKey("message", valid ? messageId : 0),
    queryFn: () => notificationsMessage(messageId),
    enabled: valid,
    retry: false,
  });
  const message = q.data?.status === 200 ? q.data.data : undefined;
  const subject = message
    ? message.event === "announcement"
      ? message.title
      : t(message.event)
    : t("notificationDetail");
  return (
    <>
      <PageHeading title={subject} subtitle={t("notificationDetailHint")}>
        <Link
          to="/notifications"
          className={buttonVariants({ variant: "outline" })}
          data-slot="button"
        >
          <ArrowLeft size={16} aria-hidden="true" />
          {t("backToInbox")}
        </Link>
        {message && (
          <Button
            variant="outline"
            disabled={message.read || task.isPending}
            onClick={() => task.mutate(() => notificationsRead(message.id))}
          >
            {t(message.read ? "read" : "markRead")}
          </Button>
        )}
      </PageHeading>
      {!valid ? (
        <p className="gov-empty">{t("empty")}</p>
      ) : q.isPending ? (
        <Loading />
      ) : q.error ? (
        <ErrorBanner error={q.error} />
      ) : !message ? (
        <p className="gov-empty">{t("empty")}</p>
      ) : (
        <section className="admin-panel entity-card gov-message-detail">
          <article className={`gov-message ${message.read ? "" : "is-unread"}`}>
            <Bell size={18} />
            <div>
              <p className="gov-message-meta">
                <span className="scope-badge">{t(message.event)}</span>
                <ShortDate value={message.createdAt} />
              </p>
              {message.event === "announcement" ? (
                <NotificationMarkdown
                  markdown={message.markdown}
                  attachments={message.attachments}
                  messageId={message.id}
                />
              ) : (
                <p>
                  {t("messageText", {
                    name: message.applicantName,
                    department: message.departmentName,
                    id: message.requestId,
                  })}
                </p>
              )}
              {message.event !== "announcement" && (
                <Link to="/temporary-access">{t("goRequest")}</Link>
              )}
            </div>
          </article>
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
