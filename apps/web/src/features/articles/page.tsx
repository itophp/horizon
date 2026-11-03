import { useMemo, useState } from "react";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Check, Eye, Pencil, Plus, Send, X } from "lucide-react";
import { useSession } from "@/features/administration/auth";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  ListSearch,
  Loading,
  PageHeading,
  RowActions,
  SuccessNotice,
  type AdminColumn,
} from "@/features/administration/shared";
import { FormField } from "@/features/administration/forms";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  articleGet,
  articleDecide,
  articlePublish,
  articleSave,
  articlesList,
  type Article,
} from "@/lib/capabilities";
import { StateBadge, stateTone } from "@/features/administration/shared";

const articleStates = [
  "draft",
  "pending_ai",
  "pending_review",
  "published",
  "rejected",
] as const;

const schema = z.object({
  title: z.string().trim().min(1, { message: "required" }).max(200),
  body: z.string().trim().min(1, { message: "required" }).max(20000),
});
type Values = z.infer<typeof schema>;

type DialogState =
  | { kind: "create" }
  | { kind: "edit"; article: Article }
  | { kind: "detail"; article: Article }
  | null;

/**
 * ArticlesPage follows the account page: a toolbar over one table, row commands
 * in an overflow menu, and a single dialog that is either detail or editor.
 */
export function ArticlesPage() {
  const { t } = useTranslation(["articles", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const status = params.get("status") ?? "all";
  const [dialog, setDialog] = useState<DialogState>(null);
  const [notice, setNotice] = useState("");

  const canWrite = session.permissions.includes("articles:write");
  const canReview = session.permissions.includes("articles:review");
  const canRead = session.permissions.includes("articles:read");

  const query = useQuery({
    queryKey: ["articles"],
    queryFn: articlesList,
    enabled: canRead,
    retry: false,
    // The review flow advances in the background, so the list refreshes itself.
    refetchInterval: 5000,
  });

  function filter(key: string, value: string) {
    setParams(
      (old) => {
        const next = new URLSearchParams(old);
        if (value && value !== "all") next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
  }

  const items = query.data?.data.items ?? [];
  const rows = useMemo(
    () =>
      items.filter(
        (article) =>
          (status === "all" || article.status === status) &&
          article.title.toLowerCase().includes(search.toLowerCase()),
      ),
    [items, status, search],
  );

  const refresh = () => client.invalidateQueries({ queryKey: ["articles"] });

  const review = useMutation({
    mutationFn: (article: Article) =>
      articlePublish(article.id, article.revision),
    onSuccess: async () => {
      await refresh();
      setNotice(t("admin:feedback.saved"));
    },
  });

  const decide = useMutation({
    mutationFn: ({
      article,
      approve,
    }: {
      article: Article;
      approve: boolean;
    }) => articleDecide(article.id, article.revision, approve),
    onSuccess: async () => {
      await refresh();
      setNotice(t("admin:feedback.saved"));
    },
  });

  const actions = (article: Article) => [
    {
      label: t("article.details"),
      icon: <Eye size={14} />,
      onClick: () => openDetail(article),
    },
    ...(canWrite &&
    (article.status === "draft" || article.status === "rejected")
      ? [
          {
            label: t("article.edit"),
            icon: <Pencil size={14} />,
            onClick: () => openEdit(article),
          },
        ]
      : []),
    ...(canWrite &&
    (article.status === "draft" ||
      article.status === "rejected" ||
      article.status === "pending_ai")
      ? [
          {
            label:
              article.status === "pending_ai" ? t("retryReview") : t("submit"),
            icon: <Send size={14} />,
            onClick: () => review.mutate(article),
          },
        ]
      : []),
    ...(canReview && article.status === "pending_review"
      ? [
          {
            label: t("approve"),
            icon: <Check size={14} />,
            onClick: () => decide.mutate({ article, approve: true }),
          },
          {
            label: t("reject"),
            icon: <X size={14} />,
            danger: true,
            onClick: () => decide.mutate({ article, approve: false }),
          },
        ]
      : []),
  ];

  // Detail and edit both need the body, which the list projection omits.
  const openDetail = (article: Article) => {
    void articleGet(article.id).then((response) =>
      setDialog({ kind: "detail", article: response.data }),
    );
  };
  const openEdit = (article: Article) => {
    void articleGet(article.id).then((response) =>
      setDialog({ kind: "edit", article: response.data }),
    );
  };

  const columns: AdminColumn<Article>[] = [
    {
      id: "title",
      header: t("title"),
      cell: (article) => (
        <button className="account-cell" onClick={() => openDetail(article)}>
          <span>
            <strong>{article.title}</strong>
            <small>
              {article.reviewCode
                ? t(`codes.${article.reviewCode}`)
                : t("article.noReview")}
            </small>
          </span>
        </button>
      ),
    },
    {
      id: "status",
      header: t("status"),
      cell: (article) => (
        <StateBadge
          label={t(`states.${article.status}`)}
          tone={stateTone(article.status)}
        />
      ),
    },
    {
      id: "reason",
      header: t("reason"),
      cell: (article) =>
        article.reviewCode ? t(`codes.${article.reviewCode}`) : t("dash"),
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (article) => <RowActions actions={actions(article)} />,
    },
  ];

  if (!canRead) {
    return (
      <div className="admin-empty">
        <strong>{t("denied")}</strong>
      </div>
    );
  }
  if (query.isPending) return <Loading />;

  return (
    <>
      <PageHeading title={t("heading")} subtitle={t("description")} />
      {(query.isError || review.isError || decide.isError) && (
        <ErrorBanner
          error={query.error ?? review.error ?? decide.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      )}
      <div className="standalone-toolbar">
        <ListSearch value={search} onChange={(value) => filter("q", value)} />
        <div className="toolbar-right">
          <select
            className="toolbar-select"
            aria-label={t("status")}
            value={status}
            onChange={(event) => filter("status", event.target.value)}
          >
            <option value="all">{t("admin:actions.all")}</option>
            {articleStates.map((value) => (
              <option key={value} value={value}>
                {t(`states.${value}`)}
              </option>
            ))}
          </select>
          <span className="muted-text">
            {t("article.total", { count: rows.length })}
          </span>
          {canWrite && (
            <Button onClick={() => setDialog({ kind: "create" })}>
              <Plus size={15} />
              {t("new")}
            </Button>
          )}
        </div>
      </div>
      <section className="admin-panel">
        <DataTable rows={rows} columns={columns} />
      </section>

      <EditorDialog
        open={dialog !== null}
        onOpenChange={(open) => {
          if (!open) setDialog(null);
        }}
        title={
          dialog?.kind === "create"
            ? t("new")
            : dialog?.kind === "edit"
              ? t("article.edit")
              : t("article.details")
        }
        description={t("article.dialogHint")}
        wide
      >
        {dialog?.kind === "detail" ? (
          <ArticleDetail
            article={dialog.article}
            canWrite={canWrite}
            canReview={canReview}
            onEdit={() => setDialog({ kind: "edit", article: dialog.article })}
            onSubmit={() => {
              review.mutate(dialog.article);
              setDialog(null);
            }}
            onDecide={(approve) => {
              decide.mutate({ article: dialog.article, approve });
              setDialog(null);
            }}
            onClose={() => setDialog(null)}
          />
        ) : (
          <ArticleForm
            key={dialog?.kind === "edit" ? dialog.article.id : "new"}
            article={dialog?.kind === "edit" ? dialog.article : undefined}
            onCancel={() => setDialog(null)}
            onSaved={async (message) => {
              setDialog(null);
              setNotice(message);
              await refresh();
            }}
          />
        )}
      </EditorDialog>
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}

function ArticleForm({
  article,
  onCancel,
  onSaved,
}: {
  article?: Article;
  onCancel: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  const { t } = useTranslation(["articles", "admin"]);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: article?.title ?? "",
      body: article?.body ?? "",
    },
  });
  const mutation = useMutation({
    mutationFn: (values: Values) =>
      articleSave({
        id: article?.id,
        revision: article?.revision,
        title: values.title,
        body: values.body,
      }),
    onSuccess: () => onSaved(t("admin:feedback.saved")),
  });

  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
    >
      {mutation.isError && <ErrorBanner error={mutation.error} />}
      <div className="editor-grid">
        <FormField
          label={t("title")}
          error={form.formState.errors.title?.message}
          wide
        >
          <Input {...form.register("title")} />
        </FormField>
        <FormField
          label={t("body")}
          error={form.formState.errors.body?.message}
          wide
        >
          <textarea className="editor-textarea" {...form.register("body")} />
        </FormField>
      </div>
      <p className="form-hint">{t("article.markdownHint")}</p>
      <div className="editor-footer">
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("admin:actions.cancel")}
        </Button>
        <Button type="submit" disabled={mutation.isPending}>
          {mutation.isPending ? t("admin:actions.saving") : t("save")}
        </Button>
      </div>
    </form>
  );
}

function ArticleDetail({
  article,
  canWrite,
  canReview,
  onEdit,
  onSubmit,
  onDecide,
  onClose,
}: {
  article: Article;
  canWrite: boolean;
  canReview: boolean;
  onEdit: () => void;
  onSubmit: () => void;
  onDecide: (approve: boolean) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation(["articles", "admin"]);
  const editable =
    canWrite && (article.status === "draft" || article.status === "rejected");
  const reviewable = canReview && article.status === "pending_review";
  const submittable =
    canWrite &&
    (article.status === "draft" ||
      article.status === "rejected" ||
      article.status === "pending_ai");

  return (
    <div className="entity-form">
      <div className="detail-body">
        <dl className="detail-grid">
          <div>
            <dt>{t("status")}</dt>
            <dd>
              <StateBadge
                label={t(`states.${article.status}`)}
                tone={stateTone(article.status)}
              />
            </dd>
          </div>
          <div>
            <dt>{t("reason")}</dt>
            <dd>
              {article.reviewCode
                ? t(`codes.${article.reviewCode}`)
                : t("dash")}
            </dd>
          </div>
          <div>
            <dt>{t("article.revision")}</dt>
            <dd>{article.revision}</dd>
          </div>
        </dl>
      </div>
      <div className="brief-block">
        <strong>{t("body")}</strong>
        <p>{article.body || t("dash")}</p>
      </div>
      <p className="form-hint">{t("policy")}</p>
      <div className="editor-footer">
        <Button variant="outline" onClick={onClose}>
          {t("admin:actions.cancel")}
        </Button>
        {editable && <Button onClick={onEdit}>{t("article.edit")}</Button>}
        {submittable && (
          <Button onClick={onSubmit}>
            {article.status === "pending_ai" ? t("retryReview") : t("submit")}
          </Button>
        )}
        {reviewable && (
          <>
            <Button variant="outline" onClick={() => onDecide(false)}>
              {t("reject")}
            </Button>
            <Button onClick={() => onDecide(true)}>{t("approve")}</Button>
          </>
        )}
      </div>
    </div>
  );
}
