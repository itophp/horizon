import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { useQueryClient, useMutation, useQuery } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import { useSession } from "@/features/administration/auth";
import {
  EditorDialog,
  ErrorBanner,
  ShortDate,
  DataTable,
  type AdminColumn,
} from "@/features/administration/shared";
import type { ChangeRecord, SettingSchedule } from "@/api/types";
import {
  settingsPublish,
  settingsSchedule,
  settingsSchedules,
  settingsCancelSchedule,
  settingsReauthenticate,
  settingsRollback,
  dictionariesPublish,
  dictionariesRollback,
} from "@/lib/capabilities";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { localScheduleTime, scheduleInstant } from "./schedule-time";
import { governanceKey } from "./runtime";
export function useGovTask() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (action: () => Promise<unknown>) => action(),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: [governanceKey] });
    },
  });
}
export function useGovKey(...parts: (string | number)[]) {
  const session = useSession();
  const { i18n } = useTranslation();
  return [
    governanceKey,
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    i18n.resolvedLanguage,
    ...parts,
  ];
}
export function NoAccess() {
  const { t } = useTranslation("governance");
  return <div className="admin-empty">{t("noPermission")}</div>;
}
export function ChangeHistory({
  drafts,
  history,
  domain,
  revision,
  overrideRevision,
}: {
  drafts: ChangeRecord[];
  history: ChangeRecord[];
  domain: "settings" | "dictionaries";
  revision: number;
  overrideRevision?: number;
}) {
  const { t } = useTranslation("governance");
  const session = useSession();
  const task = useGovTask();
  const [action, setAction] = useState<{
    record: ChangeRecord;
    rollback: boolean;
    schedule?: boolean;
  } | null>(null);
  const client = useQueryClient();
  const queue = useQuery({
    queryKey: useGovKey("settings-schedules"),
    queryFn: () => settingsSchedules(),
    enabled:
      domain === "settings" && session.permissions.includes("settings:read"),
    refetchInterval: 5000,
    retry: false,
  });
  const scheduled = queue.data?.status === 200 ? queue.data.data : [];
  const stamp = scheduled.map((job) => `${job.id}:${job.status}`).join(";");
  const previous = useRef("");
  useEffect(() => {
    if (previous.current && previous.current !== stamp) {
      void client.invalidateQueries({ queryKey: [governanceKey] });
    }
    previous.current = stamp;
  }, [stamp, client]);
  const pending = (id: number) =>
    scheduled.some((job) => job.draftId === id && job.status === "pending");
  const scheduleColumns: AdminColumn<SettingSchedule>[] = [
    {
      id: "reason",
      header: t("reason"),
      cell: (r) => <strong>{r.reason}</strong>,
    },
    { id: "version", header: t("scheduledBase"), cell: (r) => r.baseRevision },
    {
      id: "planned",
      header: t("scheduledAt"),
      cell: (r) => <ShortDate value={r.scheduledAt} />,
    },
    {
      id: "status",
      header: t("status"),
      cell: (r) => (
        <span className="scope-badge">{t(`scheduleStatuses.${r.status}`)}</span>
      ),
    },
    {
      id: "finished",
      header: t("scheduleFinished"),
      cell: (r) => (r.finishedAt ? <ShortDate value={r.finishedAt} /> : "—"),
    },
    {
      id: "result",
      header: t("scheduleResult"),
      cell: (r) =>
        r.errorCode
          ? t(`scheduleErrors.${r.errorCode}`, { defaultValue: t("failed") })
          : "—",
    },
    {
      id: "cancel",
      header: t("cancelSchedule"),
      cell: (r) =>
        r.status === "pending" &&
        session.permissions.includes("settings:publish") ? (
          <Button
            variant="outline"
            disabled={task.isPending}
            onClick={() => task.mutate(() => settingsCancelSchedule(r.id))}
          >
            {t("cancelSchedule")}
          </Button>
        ) : null,
    },
  ];
  return (
    <div className="gov-history">
      {task.error && <ErrorBanner error={task.error} />}{" "}
      {[
        { label: t("drafts"), rows: drafts, publish: true },
        { label: t("history"), rows: history, publish: false },
      ].map((group) => (
        <section className="admin-panel entity-card" key={group.label}>
          <header className="gov-card-heading">
            <h2>{group.label}</h2>
          </header>
          {group.rows.length === 0 ? (
            <p className="gov-empty">{t("empty")}</p>
          ) : (
            group.rows.map((record) => (
              <article className="gov-change" key={record.id}>
                <div className="gov-line">
                  <div>
                    <strong>{record.reason}</strong>
                    <p>
                      <ShortDate value={record.createdAt} />
                      <span>
                        {t("revision", {
                          version: record.revision || record.baseRevision,
                        })}
                      </span>
                    </p>
                  </div>
                  {session.permissions.includes(
                    `${domain}:${group.publish ? "publish" : "rollback"}`,
                  ) && (
                    <Button
                      variant="outline"
                      disabled={
                        domain === "settings" &&
                        group.publish &&
                        pending(record.id)
                      }
                      onClick={() =>
                        setAction({ record, rollback: !group.publish })
                      }
                    >
                      {group.publish ? t("publish") : t("rollback")}
                    </Button>
                  )}
                  {domain === "settings" &&
                    group.publish &&
                    session.permissions.includes("settings:publish") && (
                      <Button
                        variant="outline"
                        disabled={pending(record.id)}
                        onClick={() =>
                          setAction({ record, rollback: false, schedule: true })
                        }
                      >
                        {t(
                          pending(record.id)
                            ? "scheduledAlready"
                            : "schedulePublish",
                        )}
                      </Button>
                    )}
                </div>
                <details>
                  <summary>{t("difference")}</summary>
                  {record.changes.length > 0 ? (
                    <div className="gov-change-values">
                      {record.changes.map((c) => (
                        <p key={c.key}>
                          <strong>
                            {t(`labels.${c.key.replaceAll(".", "_")}`, {
                              defaultValue: c.key,
                            })}
                          </strong>
                          <span>{t(c.operation)}</span>
                          <code>{c.value}</code>
                        </p>
                      ))}
                    </div>
                  ) : (
                    <div className="gov-change-values">
                      {record.document?.items.map((i) => (
                        <p key={i.code}>
                          <code>{i.code}</code>
                          <span>{i.labelEn}</span>
                          <span>{i.labelZh}</span>
                          <span>{t(i.status)}</span>
                        </p>
                      ))}
                      {record.overrides?.map((o) => (
                        <p key={o.itemId}>
                          {t("overrideFields", {
                            id: o.itemId,
                            fields: o.fields.join(", "),
                          })}
                        </p>
                      ))}
                    </div>
                  )}
                </details>
              </article>
            ))
          )}
        </section>
      ))}
      {domain === "settings" && (
        <section className="admin-panel entity-card">
          <header className="gov-card-heading">
            <h2>{t("scheduledReleases")}</h2>
            <p>{t("scheduleHint")}</p>
          </header>
          {queue.error ? (
            <ErrorBanner error={queue.error} />
          ) : (
            <DataTable rows={scheduled} columns={scheduleColumns} />
          )}
        </section>
      )}
      {action && (
        <PublishDialog
          record={action.record}
          domain={domain}
          rollback={action.rollback}
          schedule={action.schedule}
          revision={
            action.rollback
              ? action.record.domain === "override"
                ? (overrideRevision ?? 0)
                : revision
              : action.record.baseRevision
          }
          onClose={() => setAction(null)}
        />
      )}
    </div>
  );
}
function PublishDialog({
  record,
  domain,
  rollback,
  schedule = false,
  revision,
  onClose,
}: {
  record: ChangeRecord;
  domain: "settings" | "dictionaries";
  rollback: boolean;
  schedule?: boolean;
  revision: number;
  onClose: () => void;
}) {
  const { t } = useTranslation("governance");
  const task = useGovTask();
  const reauth = rollback || record.sensitive;
  const scheduleZoneId = useId();
  const [defaultScheduledAt] = useState(() =>
    schedule ? localScheduleTime(new Date(Date.now() + 3600000)) : "",
  );
  const form = useForm({
    resolver: zodResolver(
      z.object({
        password: reauth ? z.string().min(1) : z.string(),
        reason: rollback ? z.string().trim().min(5) : z.string(),
        scheduledAt: schedule
          ? z.string().refine((value) => Boolean(scheduleInstant(value)))
          : z.string(),
      }),
    ),
    defaultValues: {
      password: "",
      reason: "",
      scheduledAt: defaultScheduledAt,
    },
  });
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open && !task.isPending) onClose();
      }}
      title={
        schedule ? t("schedulePublish") : reauth ? t("reauth") : t("publish")
      }
      description={
        schedule
          ? t("scheduleHint")
          : reauth
            ? t("reauthHint")
            : t("publishHint")
      }
    >
      <form
        className="admin-form"
        onSubmit={form.handleSubmit((data) =>
          task.mutate(async () => {
            const scheduledAt = schedule
              ? scheduleInstant(data.scheduledAt)
              : undefined;
            if (schedule && !scheduledAt) {
              form.setError("scheduledAt", { type: "validate" });
              return;
            }
            let proof = "";
            if (reauth) {
              const r = await settingsReauthenticate({
                password: data.password,
                operation: rollback
                  ? domain === "settings"
                    ? "settings.rollback"
                    : "dictionaries.rollback"
                  : schedule
                    ? "settings.schedule"
                    : "settings.publish",
                resourceId: record.id,
                revision,
              });
              if (r.status === 200) proof = r.data.proof;
            }
            if (domain === "settings") {
              if (schedule && scheduledAt)
                await settingsSchedule(record.id, {
                  baseRevision: revision,
                  proof,
                  scheduledAt,
                });
              else if (rollback)
                await settingsRollback(record.id, {
                  baseRevision: revision,
                  proof,
                  reason: data.reason,
                });
              else
                await settingsPublish(record.id, {
                  baseRevision: revision,
                  proof,
                });
            } else {
              if (rollback)
                await dictionariesRollback(record.id, {
                  baseRevision: revision,
                  proof,
                  reason: data.reason,
                });
              else
                await dictionariesPublish(record.id, {
                  baseRevision: revision,
                  proof,
                });
            }
            onClose();
          }),
        )}
      >
        {schedule && (
          <label>
            <span>{t("scheduledAt")}</span>
            <Input
              type="datetime-local"
              aria-label={t("scheduledAt")}
              aria-describedby={scheduleZoneId}
              step={60}
              {...form.register("scheduledAt")}
            />
            <small id={scheduleZoneId}>
              {t("scheduleZone", {
                zone: Intl.DateTimeFormat().resolvedOptions().timeZone,
              })}
            </small>
          </label>
        )}
        {reauth && (
          <label>
            <span>{t("password")}</span>
            <Input
              type="password"
              autoComplete="current-password"
              {...form.register("password")}
            />
          </label>
        )}
        {rollback && (
          <label>
            <span>{t("revertReason")}</span>
            <Input {...form.register("reason")} />
          </label>
        )}
        {Object.keys(form.formState.errors).length > 0 && (
          <p role="alert">{t("invalid")}</p>
        )}
        {task.error && <ErrorBanner error={task.error} />}
        <div className="form-actions">
          <Button
            variant="outline"
            type="button"
            disabled={task.isPending}
            onClick={onClose}
          >
            {t("cancel")}
          </Button>
          <Button type="submit" disabled={task.isPending}>
            {task.isPending ? t("busy") : t("confirm")}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
export function downloadContent(
  name: string,
  content: BlobPart,
  type = "application/octet-stream",
) {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export function FormError({ children }: { children: ReactNode }) {
  return (
    <p className="field-error" role="alert">
      {children}
    </p>
  );
}
