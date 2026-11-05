import { Suspense } from "react";
import { ContributionBoundary } from "./contributions";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { useSession } from "@/features/administration/auth";
import { runtimeApplication } from "@/lib/capabilities";
import { ErrorBanner, Loading } from "@/features/administration/shared";
import { validateDefinition, LocalBoundary, Diagnostic } from "./v2-registry";
import { ApiError } from "@/api/client/request";
import { PageView } from "./page-view";
export { EmbeddedPage } from "./page-view";
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
        identity={identity}
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
  identity,
  readOnly,
}: {
  application: import("@/api/generated/runtime").RuntimeApplicationV2;
  pageId: string;
  identity: string;
  readOnly: boolean;
}) {
  const { t } = useTranslation("runtime");
  return (
    <Suspense fallback={<Loading />}>
      <ContributionBoundary application={application} enabled={!readOnly}>
        {(effective, stale) => {
          const page = effective.pages.find((page) => page.id === pageId);
          if (!page) return <div role="alert">{t("denied")}</div>;
          return (
            <>
              <div>
                {readOnly && <p role="alert">{t("definitionExpired")}</p>}
              </div>
              <PageView
                application={effective}
                page={page}
                identity={identity}
                readOnly={readOnly || stale}
              />
            </>
          );
        }}
      </ContributionBoundary>
    </Suspense>
  );
}
