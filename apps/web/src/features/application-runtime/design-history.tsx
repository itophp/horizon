import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Check, FileDiff, Trash2, Undo2 } from "lucide-react";
import {
  runtimeDefinitionHistorySchema,
  runtimeDefinitionDiffSchema,
  type RuntimeDefinitionView,
} from "@/api/generated/runtime";
import { runtimeRead, runtimeExecute } from "@/lib/capabilities";
import { formatDate } from "@/lib/format";
import { useSession } from "@/features/administration/auth";
import {
  ErrorBanner,
  EditorDialog,
  EmptyState,
  Loading,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";

export function DesignHistory({
  current,
  identity,
}: {
  current: RuntimeDefinitionView;
  identity: string;
}) {
  const { t, i18n } = useTranslation("runtime");
  const session = useSession();
  const client = useQueryClient();
  const [choice, setChoice] = useState<{
    action: "rollback" | "discard";
    version: string;
    digest: string;
    revision: number;
    key: string;
  }>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();
  const [showDiff, setShowDiff] = useState(false);
  const applicationId = current.application.id;
  const history = useQuery({
    queryKey: [
      "runtime-design-history",
      identity,
      applicationId,
      current.revision,
    ],
    queryFn: async ({ signal }) =>
      runtimeDefinitionHistorySchema.parse(
        await runtimeRead(
          "application.runtime.definition.history",
          { applicationId },
          signal,
        ),
      ),
    retry: false,
  });
  const diff = useQuery({
    queryKey: ["runtime-design-diff", identity, applicationId, current.version],
    enabled: showDiff && !!current.publishedVersion,
    queryFn: async ({ signal }) =>
      runtimeDefinitionDiffSchema.parse(
        await runtimeRead(
          "application.runtime.definition.diff",
          { applicationId, version: current.version },
          signal,
        ),
      ),
    retry: false,
  });
  return (
    <section className="design-history">
      <p className="design-tab-hint">{t("design.historyHint")}</p>
      <div className="design-history-toolbar">
        {current.publishedVersion && (
          <Button
            type="button"
            variant="outline"
            onClick={() => setShowDiff((value) => !value)}
          >
            <FileDiff size={14} />
            {t("design.diff")}
          </Button>
        )}
        {current.proposalVersion && (
          <Button
            type="button"
            variant="destructive"
            disabled={!session.permissions.includes("applications:design")}
            onClick={() => {
              setError(undefined);
              setChoice({
                action: "discard",
                version: current.version,
                digest: current.digest,
                revision: current.revision,
                key: crypto.randomUUID(),
              });
            }}
          >
            <Trash2 size={14} />
            {t("design.discard")}
          </Button>
        )}
      </div>
      {history.isError && <ErrorBanner error={history.error} />}
      {history.isPending ? (
        <Loading />
      ) : !history.data?.items.length ? (
        <EmptyState />
      ) : (
        <ul className="design-history-list">
          {history.data.items.map((item) => (
            <li key={item.version} className="design-history-item">
              <div className="design-history-meta">
                <strong>
                  {t("design.version", { version: item.version })}
                </strong>
                <small>
                  {t("design.sourceLine", {
                    source: t(`design.sources.${item.source}`),
                    date: formatDate(item.createdAt, i18n.language),
                  })}
                </small>
              </div>
              <div className="design-history-actions">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={
                    !session.permissions.includes("applications:manage")
                  }
                  onClick={() => {
                    setError(undefined);
                    setChoice({
                      action: "rollback",
                      version: item.version,
                      digest: item.digest,
                      revision: current.revision,
                      key: crypto.randomUUID(),
                    });
                  }}
                >
                  <Undo2 size={13} />
                  {t("design.rollback")}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {showDiff &&
        (diff.isError ? (
          <ErrorBanner error={diff.error} />
        ) : (
          diff.data && (
            <div className="design-diff">
              <ul>
                {diff.data.changes.map((change) => (
                  <li key={`${change.section}:${change.id}`}>
                    {t("design.impact", {
                      section: t(`design.sections.${change.section}`),
                      id: change.id,
                      change: t(`design.changes.${change.change}`),
                    })}
                  </li>
                ))}
              </ul>
              <div className="runtime-layout-split">
                <pre>{JSON.stringify(diff.data.baseDefinition, null, 2)}</pre>
                <pre>
                  {JSON.stringify(diff.data.candidateDefinition, null, 2)}
                </pre>
              </div>
            </div>
          )
        ))}
      <EditorDialog
        open={!!choice}
        onOpenChange={(open) => {
          if (!open && !pending) setChoice(undefined);
        }}
        title={t(
          choice?.action === "rollback" ? "design.rollback" : "design.discard",
        )}
        description={t("design.historyHint")}
      >
        {error !== undefined && <ErrorBanner error={error} />}
        <Button
          type="button"
          disabled={pending}
          onClick={async () => {
            if (!choice) return;
            setPending(true);
            setError(undefined);
            try {
              const result = await runtimeExecute(
                `application.runtime.definition.${choice.action}`,
                "1.0.0",
                {
                  applicationId,
                  version: choice.version,
                  digest: choice.digest,
                  revision: choice.revision,
                },
                choice.key,
                crypto.randomUUID(),
              );
              if (result.status !== "succeeded")
                throw new Error(result.errorCode || "unavailable");
              await client.invalidateQueries({
                predicate: (query) =>
                  typeof query.queryKey[0] === "string" &&
                  query.queryKey[0].startsWith("runtime"),
              });
              setChoice(undefined);
            } catch (error) {
              setError(error);
            } finally {
              setPending(false);
            }
          }}
        >
          <Check size={14} />
          {t("confirm")}
        </Button>
      </EditorDialog>
    </section>
  );
}
