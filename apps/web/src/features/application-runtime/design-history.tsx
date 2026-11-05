import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  runtimeDefinitionHistorySchema,
  runtimeDefinitionDiffSchema,
  type RuntimeDefinitionView,
} from "@/api/generated/runtime";
import { runtimeRead, runtimeExecute } from "@/lib/capabilities";
import { useSession } from "@/features/administration/auth";
import { Button } from "@/components/ui/button";
import { ErrorBanner, EditorDialog } from "@/features/administration/shared";
export function DesignHistory({
  current,
  identity,
}: {
  current: RuntimeDefinitionView;
  identity: string;
}) {
  const { t } = useTranslation("runtime");
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
    <section>
      <h3>{t("design.history")}</h3>
      {history.isError && <ErrorBanner error={history.error} />}
      <ul>
        {history.data?.items.map((item) => (
          <li key={item.version}>
            {t("design.version", { version: item.version })}
            <Button
              type="button"
              disabled={!session.permissions.includes("applications:manage")}
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
              {t("design.rollback")}
            </Button>
          </li>
        ))}
      </ul>
      {current.proposalVersion && (
        <Button
          type="button"
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
          {t("design.discard")}
        </Button>
      )}
      {current.publishedVersion && (
        <Button type="button" onClick={() => setShowDiff((value) => !value)}>
          {t("design.diff")}
        </Button>
      )}
      {showDiff &&
        (diff.isError ? (
          <ErrorBanner error={diff.error} />
        ) : (
          diff.data && (
            <details open>
              <summary>{t("design.diff")}</summary>
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
            </details>
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
          {t("confirm")}
        </Button>
      </EditorDialog>
    </section>
  );
}
