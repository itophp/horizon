import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { useSession } from "@/features/administration/auth";
import { runtimeApplication } from "@/lib/capabilities";
import { ErrorBanner, Loading } from "@/features/administration/shared";
import { ApiError } from "@/api/client/request";
import {
  validateDefinition,
  LocalBoundary,
  Diagnostic,
} from "./definition-validation";
import { ProtocolPage } from "./protocol";
import "./runtime.css";

export function ApplicationPage({
  applicationId,
  pageId,
}: {
  applicationId: string;
  pageId: string;
}) {
  const session = useSession();
  const { i18n } = useTranslation();
  const authority = JSON.stringify([
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.authorizationVersion,
    [...session.permissions].sort(),
  ]);
  const identity = authority + ":" + i18n.language;
  return (
    <LocalBoundary key={authority + applicationId + pageId}>
      <ApplicationLoader
        applicationId={applicationId}
        pageId={pageId}
        identity={identity}
      />
    </LocalBoundary>
  );
}

function ApplicationLoader({
  applicationId,
  pageId,
  identity,
}: {
  applicationId: string;
  pageId: string;
  identity: string;
}) {
  const query = useQuery({
    queryKey: ["runtime-definition", identity, applicationId],
    queryFn: async ({ signal }) =>
      validateDefinition(await runtimeApplication(applicationId, signal)),
    retry: false,
    placeholderData: (previous) => previous,
    refetchInterval: 15000,
  });
  if (query.isPending) return <Loading />;
  if (query.data)
    return (
      <DefinitionView
        application={query.data}
        pageId={pageId}
        readOnly={query.isError}
      />
    );
  if (query.isError && !(query.error instanceof ApiError))
    return <Diagnostic error={query.error} />;
  if (query.isError)
    return (
      <ErrorBanner
        error={query.error}
        onRetry={() => {
          void query.refetch();
        }}
      />
    );
  return <Loading />;
}

function DefinitionView({
  application,
  pageId,
  readOnly,
}: {
  application: import("@/api/generated/runtime").RuntimeApplicationV2;
  pageId: string;
  readOnly: boolean;
}) {
  const { t } = useTranslation("runtime");
  const protocol = application.protocol;
  const protocolPage = protocol?.pages.find((page) => page.id === pageId);
  if (!protocol || !protocolPage) return <div role="alert">{t("denied")}</div>;
  return (
    <>
      {readOnly && <p role="alert">{t("definitionExpired")}</p>}
      <ProtocolPage
        application={protocol}
        page={protocolPage}
        readOnly={readOnly}
        invocation={{
          applicationId: application.id,
          applicationVersion: application.applicationVersion,
          catalogRevision: application.catalogRevision,
          definitionDigest: application.definitionDigest,
          installationRevision: application.installationRevision ?? 1,
          metadataVersion: application.metadataVersion,
          pageId: protocolPage.id,
        }}
      />
    </>
  );
}
