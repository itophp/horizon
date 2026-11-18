import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { FileText, Loader2, Plus, RefreshCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EditorDialog, PageHeading } from "@/features/administration/shared";
import { useSession } from "@/features/administration/auth";
import { formatMoment } from "@/lib/capability";
import {
  knowledgeCreate,
  knowledgeDelete,
  knowledgeQuery,
  knowledgeReindex,
} from "@/lib/capabilities";
import { KnowledgeSearchForm } from "./search-form";
import { KnowledgeDocumentFields } from "./document-fields";
import { knowledgeEmbeddingError } from "./errors";
import "./knowledge.css";

// Management view of the same corpus the agent retrieves from. Writing here is a
// capability, so it is audited like any other business action.
export function KnowledgePage() {
  const { t, i18n } = useTranslation(["knowledge", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const canRead = session.permissions.includes("knowledge:read");
  const canWrite = session.permissions.includes("knowledge:create");
  const canDelete = session.permissions.includes("knowledge:delete");
  const queryKey = ["knowledge", "documents"] as const;
  const [keyword, setKeyword] = useState("");
  const [cursors, setCursors] = useState<number[]>([]);
  const beforeId = cursors.at(-1);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [reindexing, setReindexing] = useState(false);
  const [rebuiltDocuments, setRebuiltDocuments] = useState<number>();
  const [error, setError] = useState<unknown>(null);
  const documents = useQuery({
    queryKey: [
      ...queryKey,
      session.tenantId,
      session.account.id,
      session.activeDepartmentId,
      session.authorizationVersion,
      keyword,
      beforeId,
    ],
    queryFn: ({ signal }) => knowledgeQuery({ keyword, beforeId }, signal),
    gcTime: 0,
    enabled: canRead,
    retry: false,
  });

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (busy || !title.trim() || !content.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await knowledgeCreate({ title: title.trim(), content });
      setTitle("");
      setContent("");
      setCreateOpen(false);
      await client.invalidateQueries({ queryKey });
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  }

  async function reindex() {
    if (reindexing) return;
    setReindexing(true);
    setError(null);
    setRebuiltDocuments(undefined);
    try {
      const { data } = await knowledgeReindex();
      setRebuiltDocuments(data.documents);
      await client.invalidateQueries({ queryKey });
    } catch (caught) {
      setError(caught);
    } finally {
      setReindexing(false);
    }
  }

  async function remove(id: number, name: string) {
    if (!window.confirm(t("delete.confirm", { title: name }))) return;
    setError(null);
    try {
      await knowledgeDelete(id);
      await client.invalidateQueries({ queryKey: ["knowledge", "document"] });
      await client.invalidateQueries({ queryKey });
    } catch (caught) {
      setError(caught);
    }
  }

  if (!canRead) {
    return (
      <section className="admin-page-heading">
        <div>
          <h1>{t("title")}</h1>
          <p>{t("errors.forbidden")}</p>
        </div>
      </section>
    );
  }

  const rows = documents.data?.data.documents ?? [];
  const nextBeforeId = documents.data?.data.nextBeforeId;
  const errorKey = knowledgeEmbeddingError(error);
  const errorMessage = error ? (
    <p className="knowledge-error" role="alert">
      {t(errorKey ?? "errors.failed")}{" "}
      {errorKey && session.permissions.includes("aiConfig:read") && (
        <Link className="underline" to="/ai-settings?tab=general">
          {t("governance:aiSettings.title")}
        </Link>
      )}
    </p>
  ) : null;

  return (
    <>
      <PageHeading title={t("title")} subtitle={t("subtitle")}>
        {canWrite && (
          <Button
            type="button"
            onClick={() => {
              setError(null);
              setCreateOpen(true);
            }}
          >
            <Plus aria-hidden="true" />
            {t("create.title")}
          </Button>
        )}
      </PageHeading>
      {!createOpen && errorMessage}
      <KnowledgeSearchForm
        keyword={keyword}
        onSearch={(value) => {
          setKeyword(value);
          setCursors([]);
        }}
      />
      {canWrite && (
        <EditorDialog
          open={createOpen}
          onOpenChange={(open) => {
            if (busy) return;
            setCreateOpen(open);
            if (!open) setError(null);
          }}
          title={t("create.title")}
          description={t("create.hint")}
        >
          {errorMessage}
          <form onSubmit={submit}>
            <KnowledgeDocumentFields
              title={title}
              content={content}
              disabled={busy}
              onTitleChange={setTitle}
              onContentChange={setContent}
            />
            <Button
              type="submit"
              disabled={busy || !title.trim() || !content.trim()}
            >
              {busy ? (
                <Loader2 aria-hidden="true" className="animate-spin" />
              ) : (
                <Plus aria-hidden="true" />
              )}
              {busy ? t("create.busy") : t("create.submit")}
            </Button>
          </form>
        </EditorDialog>
      )}
      <section className="admin-panel entity-card knowledge-list">
        <div className="knowledge-list-heading">
          <h3>{t("list.title")}</h3>
          {canWrite && (
            <Button
              type="button"
              variant="outline"
              onClick={() => void reindex()}
              disabled={reindexing}
            >
              <RefreshCw
                aria-hidden="true"
                className={reindexing ? "animate-spin" : undefined}
              />
              {reindexing ? t("reindex.busy") : t("reindex.label")}
            </Button>
          )}
        </div>
        {rebuiltDocuments !== undefined && (
          <p role="status">{t("reindex.done", { count: rebuiltDocuments })}</p>
        )}
        {documents.isPending ? (
          <p className="gov-empty">{t("list.loading")}</p>
        ) : documents.isError ? (
          <div className="gov-empty" role="alert">
            {t("list.failed")}{" "}
            <Button
              type="button"
              variant="outline"
              onClick={() => void documents.refetch()}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : !rows.length ? (
          <p className="gov-empty">
            {t(keyword ? "list.noMatches" : "list.empty")}
          </p>
        ) : (
          <ul>
            {rows.map((document) => (
              <li key={document.id}>
                <div>
                  <div className="text-sm font-semibold [overflow-wrap:anywhere]">
                    {document.title}
                  </div>
                  <small>
                    {t("list.meta", {
                      chunks: t("list.chunks", { count: document.chunks }),
                      source: t(`list.source.${document.source}`, {
                        defaultValue: document.source,
                      }),
                      created: t("list.created", {
                        time: formatMoment(i18n.language, document.createdAt),
                      }),
                    })}
                  </small>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Link
                    className="inline-flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm underline-offset-4 hover:underline"
                    to={`/knowledge/${document.id}`}
                    aria-label={t("list.readDocument", {
                      title: document.title,
                    })}
                  >
                    <FileText aria-hidden="true" size={15} />
                    {t("list.read")}
                  </Link>
                  {canDelete && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={t("delete.label")}
                      title={t("delete.label")}
                      onClick={() => remove(document.id, document.title)}
                    >
                      <Trash2 size={15} />
                    </Button>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
        {(beforeId !== undefined || nextBeforeId !== undefined) && (
          <div className="mt-3 flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={beforeId === undefined || documents.isPending}
              onClick={() => setCursors((previous) => previous.slice(0, -1))}
            >
              {t("admin:actions.previous")}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={nextBeforeId === undefined}
              onClick={() => {
                if (nextBeforeId !== undefined)
                  setCursors((previous) => [...previous, nextBeforeId]);
              }}
            >
              {t("admin:actions.next")}
            </Button>
          </div>
        )}
      </section>
    </>
  );
}
