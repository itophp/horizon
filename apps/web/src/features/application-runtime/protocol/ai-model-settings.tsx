import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { ChangeEntry, ConfigEntry, SettingsWorkspace } from "@/api/types";
import { Loading } from "@/features/administration/shared";
import type { ComponentRendererProps } from "./component-registry";
import {
  Choice,
  DraftForm,
  modelsFrom,
  value,
  type Model,
  type Save,
} from "./ai-model-settings-controls";
import { ModelList } from "./ai-model-settings-list";

const routes = [
  "workspace",
  "assistant",
  "knowledge",
  "workflow",
  "runtimeDesign",
  "projectImport",
  "eventTrigger",
  "automation",
] as const;

function ModelAssignment({
  entries,
  models,
  view,
  disabled,
  save,
}: {
  entries: ConfigEntry[];
  models: Model[];
  view: "default" | "routes";
  disabled: boolean;
  save: Save;
}) {
  const { t } = useTranslation("governance");
  const [changes, setChanges] = useState<Record<string, string>>({});
  const defaultKey = "aiModels.default";
  const activeDefault = value(entries, defaultKey) || "deepseek";
  const options = models
    .filter((model) => model.type !== "embedding")
    .map((model) => ({
      id: model.profile,
      label: model.name || t(`choices.${model.profile}`),
    }));
  const activeModel = (id: string) =>
    models.find(
      ({ profile }) => profile === (id === "inherit" ? activeDefault : id),
    );
  const keys =
    view === "default"
      ? [defaultKey]
      : routes.map((route) => `aiModels.route.${route}`);
  return (
    <div className="admin-panel ai-model-panel">
      <DraftForm
        disabled={disabled}
        submit={async (_form, reason) => {
          const draft = Object.entries(changes).flatMap(
            ([key, selected]): ChangeEntry[] =>
              selected === value(entries, key)
                ? []
                : [{ key, operation: "set", value: JSON.stringify(selected) }],
          );
          const saved = await save(draft, reason);
          if (saved) setChanges({});
          return saved;
        }}
      >
        {keys.map((key) => {
          const current = value(entries, key);
          const model = activeModel(current);
          const label = t(`labels.${key.replaceAll(".", "_")}`);
          const selected = changes[key] ?? current;
          return (
            <div key={key} className="grid gap-2">
              <Choice
                label={label}
                selected={selected}
                disabled={disabled}
                options={
                  key === defaultKey
                    ? options
                    : [
                        { id: "inherit", label: t("choices.inherit") },
                        ...options,
                      ]
                }
                onChange={(id) =>
                  setChanges((previous) => ({ ...previous, [key]: id }))
                }
              />
              {model && (
                <p className="flex flex-wrap gap-x-3 text-sm text-muted-foreground">
                  <span>
                    {t("aiSettings.effective", {
                      model: model.name || t(`choices.${model.profile}`),
                    })}
                  </span>
                  {model.provider && <span>{model.provider}</span>}
                  <span>
                    {t(
                      model.credential ? "configured" : "aiSettings.missingKey",
                    )}
                  </span>
                </p>
              )}
            </div>
          );
        })}
      </DraftForm>
    </div>
  );
}

export function AIModelSettings({
  data,
  props,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const { t } = useTranslation("governance");
  const [notice, setNotice] = useState(false);
  const workspace = data as SettingsWorkspace | undefined;
  const entries = workspace?.entries;
  if (!Array.isArray(entries)) return <Loading />;
  const disabled = readOnly || props.disabled === true;
  const models = modelsFrom(entries);
  const save: Save = async (changes, reason) => {
    setNotice(changes.length === 0);
    if (disabled || changes.length === 0) return false;
    return (await emit("submit", { changes, reason })) === true;
  };
  return (
    <>
      {props.view === "models" ? (
        <ModelList
          key={workspace?.revision}
          entries={entries}
          models={models}
          disabled={disabled}
          save={save}
        />
      ) : (
        <ModelAssignment
          key={workspace?.revision}
          entries={entries}
          models={models}
          view={props.view === "routes" ? "routes" : "default"}
          disabled={disabled}
          save={save}
        />
      )}
      {notice && <p role="status">{t("noChanges")}</p>}
    </>
  );
}
