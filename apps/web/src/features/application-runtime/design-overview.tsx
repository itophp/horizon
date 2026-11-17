import { useTranslation } from "react-i18next";
import type { RuntimeDefinitionView } from "@/api/generated/runtime";
import { DesignActivation } from "./design-activation";

export function DesignOverview({
  applicationId,
  identity,
  label,
  view,
}: {
  applicationId: string;
  identity: string;
  label: string;
  view: RuntimeDefinitionView;
}) {
  const { t } = useTranslation("runtime");
  return (
    <aside className="design-side">
      <section className="admin-panel">
        <header className="panel-heading">
          <div>
            <h2>{t("design.overview")}</h2>
            <p>{label}</p>
          </div>
        </header>
        <dl className="design-facts">
          <div>
            <dt>{t("design.publishedLabel")}</dt>
            <dd>{view.publishedVersion || t("design.notPublished")}</dd>
          </div>
          <div>
            <dt>{t("design.candidateLabel")}</dt>
            <dd>
              {view.version === view.publishedVersion
                ? t("design.noCandidate")
                : view.version}
            </dd>
          </div>
          <div>
            <dt>{t("design.packageLabel")}</dt>
            <dd>
              {t("design.packageValue", {
                source: view.application.source,
                version: view.application.pluginVersion,
              })}
            </dd>
          </div>
          <div>
            <dt>{t("design.stateLabel")}</dt>
            <dd>
              {view.enabled
                ? t("design.stateEnabled")
                : t("design.stateDisabled")}
            </dd>
          </div>
        </dl>
        <div className="design-side-actions">
          <DesignActivation
            applicationId={applicationId}
            source={view.application.source}
            enabled={view.enabled}
            identity={identity}
          />
        </div>
      </section>
    </aside>
  );
}
