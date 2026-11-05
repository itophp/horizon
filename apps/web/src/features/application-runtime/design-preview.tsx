import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { RuntimeDefinitionView } from "@/api/generated/runtime";
import { ContributionBoundary } from "./contributions";
import { preparePresentation } from "./presentation";
import { RuntimeRouteParams } from "./route-params";
import { PageView } from "./page-view";
import { Button } from "@/components/ui/button";

export function DesignPreview({
  current,
  identity,
}: {
  current: RuntimeDefinitionView;
  identity: string;
}) {
  const { t } = useTranslation("runtime");
  const [selected, setSelected] = useState("");
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [params, setParams] = useState<Record<string, string>>({});
  const application = useMemo(
    () =>
      preparePresentation({
        ...current.application,
        previewVersion: current.version,
      }),
    [current],
  );
  const pageId = application.pages.some((page) => page.id === selected)
    ? selected
    : application.pages[0]?.id;
  return (
    <ContributionBoundary application={application}>
      {(effective) => {
        const page = effective.pages.find((page) => page.id === pageId);
        if (!page) return null;
        const routeKeys = [...page.route.matchAll(/:([A-Za-z0-9_]+)/g)].map(
          (match) => match[1],
        );
        return (
          <>
            <label>
              {t("design.previewPage")}
              <select
                value={page.id}
                onChange={(event) => {
                  setSelected(event.target.value);
                  setParams({});
                  setInputs({});
                }}
              >
                {effective.pages.map((page) => (
                  <option key={page.id} value={page.id}>
                    {t(page.titleKey)}
                  </option>
                ))}
              </select>
            </label>
            {routeKeys.length > 0 && (
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  setParams(inputs);
                }}
              >
                {routeKeys.map((key) => (
                  <label key={key}>
                    {t("design.previewParameter", { name: key })}
                    <input
                      value={inputs[key] ?? ""}
                      onChange={(event) =>
                        setInputs({ ...inputs, [key]: event.target.value })
                      }
                      required
                    />
                  </label>
                ))}
                <Button type="submit">{t("design.previewLoad")}</Button>
              </form>
            )}
            {routeKeys.every((key) => params[key]) && (
              <RuntimeRouteParams.Provider value={params}>
                <PageView
                  key={page.id}
                  application={effective}
                  page={page}
                  identity={identity}
                  readOnly
                  embedded
                />
              </RuntimeRouteParams.Provider>
            )}
          </>
        );
      }}
    </ContributionBoundary>
  );
}
