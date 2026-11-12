import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { RuntimeDefinitionView } from "@/api/generated/runtime";
import { RuntimeRouteParams } from "./route-params";
import { ProtocolPage } from "./protocol";
import { Button } from "@/components/ui/button";

export function DesignPreview({ current }: { current: RuntimeDefinitionView }) {
  const { t } = useTranslation("runtime");
  const [selected, setSelected] = useState("");
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [params, setParams] = useState<Record<string, string>>({});
  const protocol = current.application.protocol;
  const pageId = protocol?.pages.some((page) => page.id === selected)
    ? selected
    : protocol?.pages[0]?.id;
  const page = protocol?.pages.find((page) => page.id === pageId);
  if (!protocol || !page) return null;
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
          {protocol.pages.map((page) => (
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
          <ProtocolPage
            key={page.id}
            application={protocol}
            page={page}
            readOnly
          />
        </RuntimeRouteParams.Provider>
      )}
    </>
  );
}
