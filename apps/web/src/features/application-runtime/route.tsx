import { RuntimeRouteParams } from "./route-params";
import { matchPath, useLocation } from "react-router";
import { useApplicationCatalog } from "./catalog";
import { ApplicationPage } from "./page";
import "./display-adapters";
import { Loading, ErrorBanner } from "@/features/administration/shared";
import { useTranslation } from "react-i18next";
export function DynamicApplicationRoute() {
  const location = useLocation();
  const catalog = useApplicationCatalog();
  const { t } = useTranslation("runtime");
  if (catalog.isPending) return <Loading />;
  if (catalog.isError) return <ErrorBanner error={catalog.error} />;
  for (const application of catalog.data?.items ?? []) {
    const declaredPages = application.protocol?.pages ?? [];
    for (const page of [...declaredPages].sort((a, b) => {
      const score = (path: string) =>
        path
          .split("/")
          .reduce((n, s) => n + (s.startsWith(":") ? 1 : s ? 10 : 0), 0);
      return score(b.route) - score(a.route);
    })) {
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
