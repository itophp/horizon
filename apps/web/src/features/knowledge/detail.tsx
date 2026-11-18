import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { ApiError } from "@/api/client/request";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import { PageHeading } from "@/features/administration/shared";
import { formatMoment } from "@/lib/capability";
import { knowledgeRead } from "@/lib/capabilities";
import { EditKnowledgeDocument } from "./edit-document";

export function KnowledgeDetailPage() {
  const { t, i18n } = useTranslation("knowledge");
  const session = useSession();
  const client = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [saved, setSaved] = useState(false);
  const canRead = session.permissions.includes("knowledge:read");
  const canEdit = session.permissions.includes("knowledge:update");
  const { id } = useParams();
  const documentID = Number(id);
  const validID =
    /^\d+$/.test(id ?? "") &&
    Number.isSafeInteger(documentID) &&
    documentID > 0;
  const document = useQuery({
    queryKey: [
      "knowledge",
      "document",
      session.tenantId,
      session.account.id,
      session.activeDepartmentId,
      session.authorizationVersion,
      documentID,
    ],
    queryFn: ({ signal }) => knowledgeRead({ id: documentID }, signal),
    enabled: canRead && validID,
    retry: false,
    gcTime: 0,
  });
  const item = canRead && validID ? document.data?.data : undefined;
  const notFound =
    !validID ||
    (document.error instanceof ApiError && document.error.code === "not_found");

  return (
    <>
      <PageHeading
        title={item?.title ?? t("detail.title")}
        subtitle={
          item
            ? t("list.meta", {
                chunks: t("list.chunks", { count: item.chunks }),
                source: t(`list.source.${item.source}`),
                created: t("list.created", {
                  time: formatMoment(i18n.language, item.createdAt),
                }),
              })
            : t("title")
        }
      >
        <Link className="text-sm underline" to="/knowledge">
          {t("detail.back")}
        </Link>
        {canEdit && item && !document.isError && (
          <Button
            type="button"
            onClick={() => {
              setSaved(false);
              setEditing(true);
            }}
          >
            <Pencil aria-hidden="true" />
            {t("edit.title")}
          </Button>
        )}
      </PageHeading>
      {saved && (
        <p className="mb-3 text-sm" role="status">
          {t("edit.saved")}
        </p>
      )}
      {editing && canEdit && item && (
        <EditKnowledgeDocument
          key={documentID}
          document={item}
          onClose={(reload) => {
            setEditing(false);
            if (reload) void document.refetch();
          }}
          onSaved={async () => {
            setEditing(false);
            setSaved(true);
            await client.invalidateQueries({ queryKey: ["knowledge"] });
          }}
        />
      )}
      <section className="admin-panel p-5">
        {!canRead ? (
          <p role="alert">{t("errors.forbidden")}</p>
        ) : notFound ? (
          <p role="alert">{t("detail.notFound")}</p>
        ) : document.isError ? (
          <div role="alert" className="flex flex-wrap items-center gap-2">
            {t("detail.failed")}
            <Button
              type="button"
              variant="outline"
              onClick={() => void document.refetch()}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : item ? (
          <>
            <h2 className="mb-3 text-sm font-semibold">
              {t("create.content")}
            </h2>
            <p className="whitespace-pre-wrap text-sm leading-relaxed [overflow-wrap:anywhere]">
              {item.content || t("detail.empty")}
            </p>
          </>
        ) : (
          <p role="status">{t("detail.loading")}</p>
        )}
      </section>
    </>
  );
}
