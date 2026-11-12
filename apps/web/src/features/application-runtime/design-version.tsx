import { DesignHistory } from "./design-history";
import { DesignValidation } from "./design-validation";
import { z } from "zod";
import { DesignBaseline } from "./design-baseline";
import { DesignActivation } from "./design-activation";
import { DesignPreview } from "./design-preview";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { runtimeRead, runtimeExecute } from "@/lib/capabilities";
import {
  runtimeDefinitionViewSchema,
  runtimeInstallationSchema,
  type RuntimeInstallation,
  type RuntimeDefinitionView,
} from "@/api/generated/runtime";
import { useSession } from "@/features/administration/auth";
import {
  ErrorBanner,
  Loading,
  EditorDialog,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
export function DesignVersion({
  applicationId,
  identity,
}: {
  applicationId: string;
  identity: string;
}) {
  const { t } = useTranslation("runtime");
  const client = useQueryClient();
  const session = useSession();
  const [preview, setPreview] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [publicationKey, setPublicationKey] = useState(() =>
    crypto.randomUUID(),
  );
  const [reviewed, setReviewed] = useState<RuntimeDefinitionView>();
  const [reviewedInstallation, setReviewedInstallation] =
    useState<RuntimeInstallation>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();
  const query = useQuery({
    queryKey: ["runtime-design-version", identity, applicationId],
    queryFn: async ({ signal }) =>
      runtimeDefinitionViewSchema.parse(
        await runtimeRead(
          "application.runtime.definition.get",
          { applicationId },
          signal,
        ),
      ),
    retry: false,
    refetchInterval: 15000,
  });
  const installation = useQuery({
    queryKey: [
      "runtime-design-installation",
      identity,
      query.data?.application.source,
    ],
    enabled: !!query.data,
    retry: false,
    queryFn: async ({ signal }) =>
      z
        .object({ installation: runtimeInstallationSchema })
        .parse(
          await runtimeRead(
            "application.runtime.installation.get",
            { pluginId: query.data!.application.source },
            signal,
          ),
        ),
  });
  if (query.isPending) return <Loading />;
  if (query.isError) return <ErrorBanner error={query.error} />;
  const current = query.data;
  const candidate = current.version !== current.publishedVersion;
  return (
    <section className="admin-panel">
      {current.publishedVersion && (
        <DesignActivation
          applicationId={applicationId}
          source={current.application.source}
          enabled={current.enabled}
          identity={identity}
        />
      )}
      <h2>{t("design.version", { version: current.version })}</h2>
      <DesignValidation
        applicationId={applicationId}
        version={current.version}
        revision={current.revision}
        identity={identity}
      />
      <DesignHistory current={current} identity={identity} />
      <DesignBaseline
        applicationId={applicationId}
        identity={identity}
        revision={current.revision}
      />
      <p>{t("design.published", { version: current.publishedVersion })}</p>
      <Button type="button" onClick={() => void query.refetch()}>
        {t("design.refresh")}
      </Button>
      <Button type="button" onClick={() => setPreview(true)}>
        {t("design.preview")}
      </Button>
      <EditorDialog
        open={preview}
        onOpenChange={setPreview}
        title={t("design.preview")}
        description={t("design.previewHint")}
        wide
      >
        {preview && <DesignPreview current={current} />}
      </EditorDialog>
      <details>
        <summary>{t("design.definition")}</summary>
        <pre>{JSON.stringify(current.application, null, 2)}</pre>
      </details>
      {candidate && (
        <Button
          type="button"
          disabled={
            !installation.data ||
            !session.permissions.includes("applications:manage")
          }
          onClick={() => {
            setReviewed(current);
            setPublicationKey(crypto.randomUUID());
            setReviewedInstallation(installation.data?.installation);
            setConfirm(true);
          }}
        >
          {t("design.publish")}
        </Button>
      )}
      <EditorDialog
        open={confirm}
        onOpenChange={(open) => {
          if (!pending) setConfirm(open);
        }}
        title={t("design.publish")}
        description={t("design.publishHint")}
      >
        {error !== undefined && <ErrorBanner error={error} />}
        <Button
          type="button"
          disabled={pending}
          onClick={async () => {
            setPending(true);
            setError(undefined);
            try {
              if (!reviewed || !reviewedInstallation)
                throw new Error("conflict");
              const packageChange =
                reviewed.application.pluginVersion !==
                reviewedInstallation.version;
              const lifecycleAction =
                reviewedInstallation.previousVersion ===
                reviewed.application.pluginVersion
                  ? "rollback"
                  : "upgrade";
              const result = await runtimeExecute(
                packageChange
                  ? `application.runtime.installation.${lifecycleAction}`
                  : "application.runtime.definition.publish",
                "1.0.0",
                packageChange
                  ? {
                      pluginId: reviewed.application.source,
                      version: reviewed.application.pluginVersion,
                      revision: reviewedInstallation.revision,
                      definitions: [
                        {
                          applicationId,
                          version: reviewed.version,
                          digest: reviewed.digest,
                          revision: reviewed.revision,
                        },
                      ],
                    }
                  : {
                      applicationId,
                      version: reviewed.version,
                      digest: reviewed.digest,
                      revision: reviewed.revision,
                    },
                publicationKey,
                crypto.randomUUID(),
              );
              if (result.status !== "succeeded")
                throw new Error(result.errorCode || "unavailable");
              await client.invalidateQueries({
                predicate: (query) =>
                  typeof query.queryKey[0] === "string" &&
                  query.queryKey[0].startsWith("runtime"),
              });
              setConfirm(false);
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
