import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Check, Code2, Eye, Rocket } from "lucide-react";
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
import { z } from "zod";
import { DesignValidation } from "./design-validation";
import { DesignPreview } from "./design-preview";

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
    <div className="design-candidate">
      <div className="design-candidate-head">
        <div>
          <span className="design-kicker">{t("design.candidateLabel")}</span>
          <strong>{current.version}</strong>
          <small>
            {t("design.published", {
              version: current.publishedVersion || t("design.notPublished"),
            })}
          </small>
        </div>
        <div className="design-actions">
          <Button
            type="button"
            variant="outline"
            onClick={() => setPreview(true)}
          >
            <Eye size={14} />
            {t("design.preview")}
          </Button>
          {candidate ? (
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
                setError(undefined);
              }}
            >
              <Rocket size={14} />
              {t("design.publish")}
            </Button>
          ) : (
            <span className="design-tab-hint">{t("design.noCandidate")}</span>
          )}
        </div>
      </div>
      <DesignValidation
        applicationId={applicationId}
        version={current.version}
        revision={current.revision}
        identity={identity}
      />
      <details className="design-definition">
        <summary>
          <Code2 size={14} />
          {t("design.definition")}
        </summary>
        <pre>{JSON.stringify(current.application, null, 2)}</pre>
      </details>
      <EditorDialog
        open={preview}
        onOpenChange={setPreview}
        title={t("design.preview")}
        description={t("design.previewHint")}
        wide
      >
        {preview && <DesignPreview current={current} />}
      </EditorDialog>
      <EditorDialog
        open={confirm}
        onOpenChange={(open) => {
          if (!pending) setConfirm(open);
        }}
        title={t("design.publish")}
        description={t("design.publishHint")}
      >
        {error !== undefined && <ErrorBanner error={error} />}
        <p className="design-tab-hint">{t("design.candidateHint")}</p>
        <dl className="design-publish-summary">
          <div>
            <dt>{t("design.publishVersion")}</dt>
            <dd>{reviewed?.version}</dd>
          </div>
          <div>
            <dt>{t("design.publishDigest")}</dt>
            <dd>
              {t("design.digestValue", {
                digest: reviewed?.digest.slice(0, 16) ?? "",
              })}
            </dd>
          </div>
        </dl>
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
          <Check size={14} />
          {t("confirm")}
        </Button>
      </EditorDialog>
    </div>
  );
}
