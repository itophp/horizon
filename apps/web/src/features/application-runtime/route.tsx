import { RuntimeRouteParams } from "./route-params";
import { matchPath, useLocation } from "react-router";
import { useApplicationCatalog } from "./catalog";
import { ApplicationPage } from "./page";
import { Loading, ErrorBanner } from "@/features/administration/shared";
import { useTranslation } from "react-i18next";
export function DynamicApplicationRoute() {
  const location = useLocation();
  const catalog = useApplicationCatalog();
  const { t } = useTranslation("runtime");
  if (catalog.isPending) return <Loading />;
  if (catalog.isError) return <ErrorBanner error={catalog.error} />;
  for (const application of catalog.data?.items ?? []) {
    for (const page of application.pages) {
      const match = matchPath(
        { path: page.route, end: true },
        location.pathname,
      );
      if (match)
        return (
          <RuntimeRouteParams.Provider value={match.params}>
            <ApplicationPage applicationId={application.id} pageId={page.id} />
          </RuntimeRouteParams.Provider>
        );
    }
  }
  return <p role="alert">{t("denied")}</p>;
}
