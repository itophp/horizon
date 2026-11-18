import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import type { AiConfigReadOutput } from "@/api/generated/capability-ai-config";
import { Loading } from "@/features/administration/shared";
import type { ComponentRendererProps } from "./component-registry";
import {
  Choice,
  SaveForm,
  modelsFrom,
  value,
  type AIConfigEntry,
  type AIConfigChange,
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

const embeddingKey = "knowledge.embedding.profile";

function ModelAssignment({
  entries,
  models,
  embeddingModels,
  view,
  disabled,
  save,
}: {
  entries: AIConfigEntry[];
  models: Model[];
  embeddingModels: NonNullable<AiConfigReadOutput["embeddingModels"]>;
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
      : [
          ...routes.map((route) => `aiModels.route.${route}`),
          ...(embeddingModels.length ? [embeddingKey] : []),
        ];
  return (
    <div className="admin-panel ai-model-panel">
      <p className="mb-4 text-sm text-muted-foreground">
        {t("aiSettings.assignmentHint")}{" "}
        <Link className="underline" to="?tab=models">
          {t("aiSettings.tabs.models")}
        </Link>
      </p>
      <SaveForm
        disabled={disabled}
        submit={async () => {
          const updates = Object.entries(changes).flatMap(
            ([key, selected]): AIConfigChange[] =>
              selected === value(entries, key)
                ? []
                : [{ key, operation: "set", value: JSON.stringify(selected) }],
          );
          const saved = await save(updates);
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
                  key === embeddingKey
                    ? embeddingModels
                    : key === defaultKey
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
      </SaveForm>
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
  const [notice, setNotice] = useState<"saved" | "noChanges">();
  const workspace = data as AiConfigReadOutput | undefined;
  const entries = workspace?.entries;
  if (!Array.isArray(entries)) return <Loading />;
  const disabled = readOnly || props.disabled === true;
  const models = modelsFrom(entries);
  const save: Save = async (changes, onError) => {
    setNotice(undefined);
    if (disabled || changes.length === 0) {
      onError?.(disabled ? "forbidden" : "no_changes");
      if (!onError && changes.length === 0) setNotice("noChanges");
      return false;
    }
    const payload = { changes };
    const saved =
      (await emit("submit", payload, onError ? { onError } : undefined)) ===
      true;
    if (saved) setNotice("saved");
    return saved;
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
          embeddingModels={workspace?.embeddingModels ?? []}
          view={props.view === "routes" ? "routes" : "default"}
          disabled={disabled}
          save={save}
        />
      )}
      {notice && <p role="status">{t(notice)}</p>}
    </>
  );
}
