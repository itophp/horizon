import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { ApiError } from "@/api/client/request";
import { PageHeading } from "@/features/administration/shared";
import { useSession } from "@/features/administration/auth";
import { formatMoment } from "@/lib/capability";
import {
  knowledgeCreate,
  knowledgeDelete,
  knowledgeList,
  knowledgeReindex,
} from "@/lib/capabilities";
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
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [busy, setBusy] = useState(false);
  const [reindexing, setReindexing] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const documents = useQuery({
    queryKey,
    queryFn: knowledgeList,
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
    try {
      await knowledgeReindex();
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

  const rows = documents.data?.data ?? [];

  return (
    <>
      <PageHeading title={t("title")} subtitle={t("subtitle")}>
        <span className="ai-badge">{t("count", { count: rows.length })}</span>
      </PageHeading>
      {!!error && (
        <p className="knowledge-error" role="alert">
          {error instanceof ApiError && error.code === "unavailable"
            ? t("notConfigured")
            : t("errors.failed")}
        </p>
      )}
      <div className="knowledge-layout">
        <section className="admin-panel entity-card knowledge-create">
          <h3>{t("create.title")}</h3>
          {canWrite ? (
            <form onSubmit={submit}>
              <label className="knowledge-field">
                <span>{t("create.name")}</span>
                <Input
                  value={title}
                  maxLength={200}
                  placeholder={t("create.namePlaceholder")}
                  onChange={(event) => setTitle(event.target.value)}
                />
              </label>
              <label className="knowledge-field">
                <span>{t("create.content")}</span>
                <textarea
                  value={content}
                  rows={10}
                  maxLength={100000}
                  placeholder={t("create.contentPlaceholder")}
                  onChange={(event) => setContent(event.target.value)}
                />
              </label>
              <p className="knowledge-hint">{t("create.hint")}</p>
              <button
                type="submit"
                className="knowledge-submit"
                disabled={busy || !title.trim() || !content.trim()}
              >
                {busy && <Loader2 size={14} className="animate-spin" />}
                {busy ? t("create.busy") : t("create.submit")}
              </button>
            </form>
          ) : (
            <p className="gov-empty">{t("errors.forbidden")}</p>
          )}
        </section>
        <section className="admin-panel entity-card knowledge-list">
          <div className="knowledge-list-heading">
            <h3>{t("list.title")}</h3>
            {canWrite && (
              <button
                type="button"
                className="knowledge-reindex"
                onClick={() => void reindex()}
                disabled={reindexing}
              >
                {reindexing ? t("reindex.busy") : t("reindex.label")}
              </button>
            )}
          </div>
          {documents.isPending ? (
            <p className="gov-empty">{t("list.loading")}</p>
          ) : !rows.length ? (
            <p className="gov-empty">{t("list.empty")}</p>
          ) : (
            <ul>
              {rows.map((document) => (
                <li key={document.id}>
                  <div>
                    <strong>{document.title}</strong>
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
                  {canDelete && (
                    <button
                      type="button"
                      aria-label={t("delete.label")}
                      title={t("delete.label")}
                      onClick={() => remove(document.id, document.title)}
                    >
                      <Trash2 size={15} />
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </>
  );
}
