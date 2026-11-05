import {
  runtimeDefinitionBaselineListSchema,
  runtimeDefinitionMergeReportSchema,
} from "@/api/generated/runtime";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { runtimeRead, runtimeExecute } from "@/lib/capabilities";
import { useSession } from "@/features/administration/auth";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/features/administration/shared";
export function DesignBaseline({
  applicationId,
  identity,
  revision,
}: {
  applicationId: string;
  identity: string;
  revision: number;
}) {
  const session = useSession();
  const { t } = useTranslation("runtime");
  const client = useQueryClient();
  const [digest, setDigest] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();
  const [conflicts, setConflicts] = useState<string[]>([]);
  const query = useQuery({
    queryKey: ["runtime-design-baselines", identity, applicationId],
    queryFn: async ({ signal }) =>
      runtimeDefinitionBaselineListSchema.parse(
        await runtimeRead(
          "application.runtime.definition.baselines",
          { applicationId },
          signal,
        ),
      ),
    retry: false,
  });
  return (
    <section>
      {conflicts.length > 0 && (
        <div role="alert">
          <p>{t("design.conflicts")}</p>
          <ul>
            {conflicts.map((path) => (
              <li key={path}>
                <code>{path}</code>
              </li>
            ))}
          </ul>
        </div>
      )}
      <label>
        {t("design.baseline")}
        <select
          value={digest}
          onChange={(event) => setDigest(event.target.value)}
        >
          <option value="">{t("choose")}</option>
          {query.data?.items.map((item) => (
            <option key={item.digest} value={item.digest}>
              {t("design.baselineVersion", {
                version: item.version,
                pluginVersion: item.pluginVersion,
              })}
            </option>
          ))}
        </select>
      </label>
      {(query.isError || error !== undefined) && (
        <ErrorBanner error={error ?? query.error} />
      )}
      <Button
        type="button"
        disabled={
          !digest ||
          pending ||
          !session.permissions.includes("applications:design")
        }
        onClick={async () => {
          setPending(true);
          setError(undefined);
          try {
            const report = runtimeDefinitionMergeReportSchema.parse(
              await runtimeRead("application.runtime.definition.rebase.check", {
                applicationId,
                targetDigest: digest,
              }),
            );
            setConflicts(report.conflicts);
            if (!report.compatible) return;
            const result = await runtimeExecute(
              "application.runtime.definition.rebase",
              "1.0.0",
              { applicationId, targetDigest: digest, revision },
              crypto.randomUUID(),
              crypto.randomUUID(),
            );
            if (result.status !== "succeeded")
              throw new Error(result.errorCode || "unavailable");
            await client.invalidateQueries({
              queryKey: ["runtime-design-version", identity, applicationId],
            });
          } catch (error) {
            setError(error);
          } finally {
            setPending(false);
          }
        }}
      >
        {t("design.rebase")}
      </Button>
    </section>
  );
}
