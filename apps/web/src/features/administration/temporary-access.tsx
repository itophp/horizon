import { useConfiguration } from "@/features/governance/runtime";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useQueryClient } from "@tanstack/react-query";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Clock3, ShieldCheck } from "lucide-react";
import {
  identityRequestTemporaryAccess,
  identityReviewTemporaryAccess,
  identityCancelTemporaryAccess,
  identityRevokeTemporaryAccess,
  getIdentityTemporaryAccessQueryKey,
  useIdentityTemporaryAccess,
} from "@/api/generated/baseline";
import type {
  TemporaryAccessRecord,
  TemporaryAccessWorkspace,
} from "@/api/generated/models";
import { Button } from "@/components/ui/button";
import { useSession } from "./auth";
import { PermissionMatrix, permissionFunctionLabel } from "./permission-matrix";
import { FormField } from "./forms";
import {
  PageHeading,
  DataTable,
  EditorDialog,
  ErrorBanner,
  Loading,
  ShortDate,
  useRefreshAdmin,
  type AdminColumn,
  SuccessNotice,
} from "./shared";
const durations = [30, 60, 120, 240, 480, 1440, 4320, 10080] as const;
function durationLabel(minutes: number, t: TFunction) {
  return minutes < 60
    ? t("admin:temporary.durationValue", { count: minutes })
    : minutes < 1440
      ? t("admin:temporary.durationHours", { count: minutes / 60 })
      : t("admin:temporary.durationDays", { count: minutes / 1440 });
}
const schema = z.object({
  reason: z.string().trim().min(5).max(500),
  minutes: z.union([
    z.literal(30),
    z.literal(60),
    z.literal(120),
    z.literal(240),
    z.literal(480),
    z.literal(1440),
    z.literal(4320),
    z.literal(10080),
  ]),
  decision: z.enum(["approve", "reject"]),
  note: z.string().max(500, { message: "note" }),
  grants: z
    .array(
      z.object({
        permission: z.string(),
        effect: z.enum(["allow", "deny"]),
        scope: z.enum([
          "ALL",
          "ORGANIZATION",
          "DEPARTMENT",
          "DEPARTMENT_TREE",
          "OWN",
          "CUSTOM",
        ]),
        departmentIds: z.array(z.number()),
      }),
    )
    .refine(
      (grants) =>
        grants.length > 0 &&
        grants.every((g) => g.scope !== "CUSTOM" || g.departmentIds.length > 0),
    ),
});
function RequestEditor({
  workspace,
  record,
  review = false,
  readOnly = false,
  onSaved,
  onCancel,
}: {
  workspace: TemporaryAccessWorkspace;
  record?: TemporaryAccessRecord;
  review?: boolean;
  readOnly?: boolean;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const config = useConfiguration();
  const configured = config.entries.find(
    (e) => e.key === "temporary.allowedMinutes",
  );
  const allowed: unknown = configured
    ? JSON.parse(configured.value)
    : durations;
  const availableDurations = durations.filter(
    (m) =>
      (Array.isArray(allowed) && allowed.includes(m)) ||
      (readOnly && m === (record?.approvedMinutes || record?.requestedMinutes)),
  );
  const reasonMin = Number(config.value("temporary.reasonMin", 5));
  const { t } = useTranslation();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const form = useForm({
    resolver: zodResolver(
      schema.extend({
        reason: z
          .string()
          .trim()
          .min(review || readOnly ? 1 : reasonMin)
          .max(500),
      }),
    ),
    defaultValues: {
      reason: record?.reason ?? "",
      minutes:
        availableDurations.find(
          (value) =>
            value === (record?.approvedMinutes || record?.requestedMinutes),
        ) ??
        availableDurations[0] ??
        30,
      decision: "approve" as "approve" | "reject",
      note: record?.reviewNote ?? "",
      grants: record?.approvedGrants.length
        ? record.approvedGrants
        : (record?.requestedGrants ?? []),
    },
  });
  const grants = useWatch({ control: form.control, name: "grants" });
  const decision = useWatch({ control: form.control, name: "decision" });
  const submit = async () => {
    setError(null);
    // Rejecting records does not require granting permissions, but both paths validate notes and duration.
    if (review && decision === "reject") {
      const valid = await form.trigger(["minutes", "note"]);
      if (!valid) return;
    } else if (!(await form.trigger())) return;
    const values = form.getValues();
    setBusy(true);
    try {
      if (review && record)
        await identityReviewTemporaryAccess(record.id, {
          grants: decision === "reject" ? [] : values.grants,
          minutes: values.minutes,
          decision: values.decision,
          note: values.note,
        });
      else
        await identityRequestTemporaryAccess({
          grants: values.grants,
          minutes: values.minutes,
          reason: values.reason,
        });
      onSaved();
    } catch (cause) {
      setError(cause);
    } finally {
      setBusy(false);
    }
  };
  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      {!!error && <ErrorBanner error={error} />}
      {record && (
        <div className="request-context">
          <strong>{record.applicantName}</strong>
          <span className="muted-text">{record.departmentName}</span>
          <p>{record.reason}</p>
          <p>
            {t("admin:temporary.statusLabel")}
            {t(`admin:temporary.status.${record.status}`)}
          </p>
          <ShortDate value={record.createdAt} />
          {record.expiresAt && (
            <p>
              {t("admin:temporary.expiresAt")}
              <RequestTime value={record.expiresAt} />
            </p>
          )}
          {record.reviewerName && (
            <p>
              {t("admin:temporary.reviewer")}
              {record.reviewerName}
            </p>
          )}
        </div>
      )}
      {record && (
        <details className="request-original">
          <summary>{t("admin:temporary.original")}</summary>
          <p>
            {t("admin:temporary.durationValue", {
              count: record.requestedMinutes,
            })}
          </p>
          <PermissionMatrix
            grants={record.requestedGrants}
            onChange={() => {}}
            departments={workspace.departments.map((department) => ({
              id: department.id,
              name: t("admin:temporary.departmentChoice", {
                company: department.organizationName,
                department: department.name,
              }),
            }))}
            catalog={workspace.catalog}
            request
            disabled
          />
        </details>
      )}
      <fieldset disabled={readOnly || busy}>
        <div className="editor-grid">
          {!record && (
            <FormField
              label={t("admin:temporary.reason")}
              wide
              error={form.formState.errors.reason ? "reason" : undefined}
            >
              <textarea rows={3} {...form.register("reason")} />
            </FormField>
          )}
          {review && (
            <FormField label={t("admin:temporary.decision")}>
              <select {...form.register("decision")}>
                <option value="approve">{t("admin:temporary.approve")}</option>
                <option value="reject">{t("admin:temporary.reject")}</option>
              </select>
            </FormField>
          )}
          <FormField label={t("admin:temporary.duration")}>
            <select {...form.register("minutes", { valueAsNumber: true })}>
              {availableDurations.map((minutes) => (
                <option key={minutes} value={minutes}>
                  {durationLabel(minutes, t)}
                </option>
              ))}
            </select>
          </FormField>
          {(review || readOnly) && (
            <FormField
              label={t("admin:temporary.note")}
              error={form.formState.errors.note?.message}
              wide
            >
              <textarea rows={2} {...form.register("note")} />
            </FormField>
          )}
        </div>
        {(!review || decision === "approve") && (
          <>
            <div className="editor-section-heading">
              <h3>
                {review ||
                (record &&
                  ["approved", "expired", "revoked"].includes(record.status))
                  ? t("admin:temporary.finalGrant")
                  : t("admin:temporary.selectPermissions")}
              </h3>
              <p>{t("admin:temporary.scopeHint")}</p>
            </div>
            <PermissionMatrix
              key={record?.id ?? "new"}
              grants={grants}
              onChange={(value) =>
                form.setValue("grants", value, { shouldDirty: true })
              }
              departments={workspace.departments.map((department) => ({
                id: department.id,
                name: t("admin:temporary.departmentChoice", {
                  company: department.organizationName,
                  department: department.name,
                }),
              }))}
              catalog={workspace.catalog}
              request
              disabled={readOnly || busy}
            />
            {form.formState.errors.grants && (
              <p className="field-error" role="alert">
                {t("admin:temporary.grantRequired")}
              </p>
            )}
          </>
        )}
      </fieldset>
      <p className="form-hint">{t("admin:temporary.policyHint")}</p>
      <div className="editor-footer">
        <Button type="button" variant="outline" onClick={onCancel}>
          {t(readOnly ? "admin:actions.close" : "admin:actions.cancel")}
        </Button>
        {!readOnly && (
          <Button type="submit" disabled={busy}>
            {busy
              ? t("admin:actions.saving")
              : review
                ? t("admin:temporary.submitReview")
                : t("admin:temporary.submit")}
          </Button>
        )}
      </div>
    </form>
  );
}
function RequestTime({ value }: { value: string }) {
  const { i18n } = useTranslation();
  return (
    <time dateTime={value}>
      {new Intl.DateTimeFormat(i18n.resolvedLanguage, {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: "Asia/Shanghai",
      }).format(new Date(value))}
    </time>
  );
}
export function TemporaryAccessPage() {
  const { t } = useTranslation();
  const session = useSession();
  const client = useQueryClient();
  const refresh = useRefreshAdmin();
  const query = useIdentityTemporaryAccess({
    query: {
      retry: false,
      refetchInterval: 15000,
      queryKey: [
        ...getIdentityTemporaryAccessQueryKey(),
        session.tenantId,
        session.account.id,
        session.activeDepartmentId,
      ],
    },
  });
  const workspace = query.data?.status === 200 ? query.data.data : undefined;
  const [tab, setTab] = useState<"mine" | "inbox">("mine");
  const [status, setStatus] = useState("all");
  const [editor, setEditor] = useState<{
    record?: TemporaryAccessRecord;
    review?: boolean;
    readOnly?: boolean;
  } | null>(null);
  const [revoke, setRevoke] = useState<TemporaryAccessRecord | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState("");
  async function reload() {
    await Promise.all([
      client.invalidateQueries({
        queryKey: getIdentityTemporaryAccessQueryKey(),
      }),
      refresh(),
    ]);
  }
  async function saved() {
    setEditor(null);
    setNotice(t("admin:feedback.saved"));
    await reload();
  }
  async function cancel(record: TemporaryAccessRecord) {
    setError(null);
    try {
      await identityCancelTemporaryAccess(record.id);
      await reload();
      setNotice(t("admin:feedback.saved"));
    } catch (cause) {
      setError(cause);
    }
  }
  async function revokeLease() {
    if (!revoke) return;
    setBusy(true);
    setError(null);
    try {
      await identityRevokeTemporaryAccess(revoke.id);
      setRevoke(null);
      await reload();
      setNotice(t("admin:feedback.saved"));
    } catch (cause) {
      setError(cause);
    } finally {
      setBusy(false);
    }
  }
  if (query.isPending) return <Loading />;
  if (!workspace) return <ErrorBanner error={query.error} />;
  const rows = workspace.requests.filter(
    (record) =>
      (tab === "mine"
        ? record.applicantId === session.account.id
        : record.applicantId !== session.account.id) &&
      (status === "all" || record.status === status),
  );
  const columns: AdminColumn<TemporaryAccessRecord>[] = [
    {
      id: "applicant",
      header: t("admin:temporary.applicant"),
      cell: (record) => (
        <div className="entity-name">
          <strong>{record.applicantName}</strong>
          <span className="muted-text">{record.departmentName}</span>
          <small>{record.reason}</small>
        </div>
      ),
    },
    {
      id: "grants",
      header: t("admin:temporary.requested"),
      cell: (record) => (
        <div className="grant-badges">
          {[
            ...new Set(
              record.requestedGrants.map(
                (g) =>
                  workspace.catalog.find((d) => d.code === g.permission)
                    ?.function ?? g.permission,
              ),
            ),
          ].map((fn) => (
            <span className="neutral-badge" key={fn}>
              {permissionFunctionLabel(fn, t)}
            </span>
          ))}
        </div>
      ),
    },
    {
      id: "duration",
      header: t("admin:temporary.duration"),
      cell: (record) =>
        durationLabel(record.approvedMinutes || record.requestedMinutes, t),
    },
    {
      id: "status",
      header: t("admin:fields.status"),
      cell: (record) => (
        <span className={`request-status request-${record.status}`}>
          {t(`admin:temporary.status.${record.status}`)}
        </span>
      ),
    },
    {
      id: "expires",
      header: t("admin:temporary.expiresAt"),
      cell: (record) =>
        record.expiresAt ? (
          <RequestTime value={record.expiresAt} />
        ) : (
          t("admin:temporary.notStarted")
        ),
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      cell: (record) => (
        <div className="request-actions">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setEditor({ record, readOnly: true })}
          >
            {t("admin:actions.details")}
          </Button>
          {record.status === "pending" &&
            record.applicantId === session.account.id && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  void cancel(record);
                }}
              >
                {t("admin:temporary.cancel")}
              </Button>
            )}
          {record.status === "pending" &&
            workspace.canReview &&
            record.applicantId !== session.account.id && (
              <Button
                size="sm"
                onClick={() => setEditor({ record, review: true })}
              >
                {t("admin:temporary.review")}
              </Button>
            )}
          {record.status === "approved" &&
            (workspace.canReview ||
              record.applicantId === session.account.id) && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setRevoke(record)}
              >
                {t("admin:temporary.revoke")}
              </Button>
            )}
        </div>
      ),
    },
  ];
  return (
    <>
      <PageHeading
        title={t("admin:temporary.title")}
        subtitle={t("admin:temporary.subtitle")}
      >
        <Button onClick={() => setEditor({})}>
          <Plus size={15} />
          {t("admin:temporary.create")}
        </Button>
      </PageHeading>
      <div className="temporary-banner">
        <Clock3 size={18} />
        <div>
          <strong>{t("admin:temporary.durationHint")}</strong>
          <p>{t("admin:temporary.scopeHint")}</p>
        </div>
        <ShieldCheck size={20} />
      </div>
      <div className="permission-page-tabs">
        <button
          className={tab === "mine" ? "active" : ""}
          onClick={() => setTab("mine")}
        >
          {t("admin:temporary.mine")}
        </button>
        {workspace.canReview && (
          <button
            className={tab === "inbox" ? "active" : ""}
            onClick={() => setTab("inbox")}
          >
            {t("admin:temporary.inbox")}
          </button>
        )}
      </div>
      {!!error && <ErrorBanner error={error} />}
      <section className="admin-panel">
        <div className="table-toolbar">
          <select
            aria-label={t("admin:fields.status")}
            value={status}
            onChange={(event) => setStatus(event.target.value)}
          >
            <option value="all">{t("admin:status.all")}</option>
            {[
              "pending",
              "approved",
              "expired",
              "rejected",
              "cancelled",
              "revoked",
            ].map((value) => (
              <option value={value} key={value}>
                {t(`admin:temporary.status.${value}`)}
              </option>
            ))}
          </select>
        </div>
        <DataTable rows={rows} columns={columns} />
      </section>
      <EditorDialog
        open={!!editor}
        onOpenChange={(open) => {
          if (!open) setEditor(null);
        }}
        title={
          editor?.readOnly
            ? t("admin:temporary.details")
            : editor?.review
              ? t("admin:temporary.review")
              : t("admin:temporary.create")
        }
        description={t("admin:temporary.policyHint")}
        wide
      >
        {editor && (
          <RequestEditor
            key={`${editor.record?.id ?? "new"}-${editor.review}`}
            workspace={workspace}
            record={editor.record}
            review={editor.review}
            readOnly={editor.readOnly}
            onCancel={() => setEditor(null)}
            onSaved={() => {
              void saved();
            }}
          />
        )}
      </EditorDialog>
      <EditorDialog
        open={!!revoke}
        onOpenChange={(open) => {
          if (!open) setRevoke(null);
        }}
        title={t("admin:temporary.revoke")}
        description={t("admin:temporary.revokeHint")}
      >
        <div className="editor-footer">
          <Button variant="outline" onClick={() => setRevoke(null)}>
            {t("admin:actions.cancel")}
          </Button>
          <Button
            disabled={busy}
            onClick={() => {
              void revokeLease();
            }}
          >
            {t("admin:temporary.revoke")}
          </Button>
        </div>
      </EditorDialog>
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}
