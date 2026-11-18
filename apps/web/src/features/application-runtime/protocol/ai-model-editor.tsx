import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Choice,
  SaveForm,
  choiceOptions,
  modelKey,
  modelProtocols,
  modelTypes,
  value,
  type AIConfigChange,
  type AIConfigEntry,
  type Model,
  type Save,
} from "./ai-model-settings-controls";

const fields = [
  "displayName",
  "providerId",
  "type",
  "protocol",
  "baseUrl",
  "model",
  "token",
  "dimensions",
  "responsesBaseUrl",
] as const;

export function ModelEditor({
  profile,
  model,
  entries,
  save,
}: {
  profile: string;
  model?: Model;
  entries: AIConfigEntry[];
  save: Save;
}) {
  const { t } = useTranslation("governance");
  const uid = useId();
  const initialType = model?.type ?? "text";
  const [type, setType] = useState<string>(initialType);
  const originalProtocol = value(entries, modelKey(profile, "protocol"));
  const initialProtocols = modelProtocols(initialType);
  const initialProtocol = initialProtocols.includes(originalProtocol)
    ? originalProtocol
    : initialProtocols[0];
  const [protocol, setProtocol] = useState(initialProtocol);
  const embedding = type === "embedding";
  const dimensionKey = modelKey(profile, "dimensions");
  const initialDimensions = entries.find(
    (entry) => entry.key === dimensionKey,
  )?.value;
  const [dimensions, setDimensions] = useState(
    initialDimensions ??
      (initialProtocol === "dashscope-embedding" ? "2560" : "1536"),
  );
  const original = (field: (typeof fields)[number]) => {
    if (field === "type") return model?.type ?? "";
    if (field === "protocol") return model ? initialProtocol : "";
    if (field === "dimensions") return initialDimensions ?? "";
    if (field === "displayName")
      return (
        model?.name ||
        (model ? t(`choices.${model.profile}`, { defaultValue: "" }) : "")
      );
    return value(entries, modelKey(profile, field));
  };
  const disabledField = (field: (typeof fields)[number]) =>
    (field === "dimensions" && !embedding) ||
    (field === "responsesBaseUrl" && embedding);

  return (
    <SaveForm
      disabled={false}
      submit={(form) => {
        const changes = fields.flatMap((field): AIConfigChange[] => {
          if (disabledField(field)) return [];
          const key = modelKey(profile, field);
          const next =
            field === "type"
              ? type
              : field === "protocol"
                ? protocol
                : form.get(key);
          if (
            typeof next !== "string" ||
            next.trim() === original(field) ||
            (field === "token" && !next)
          ) {
            return [];
          }
          return [
            {
              key,
              operation: "set",
              value: JSON.stringify(
                field === "dimensions" ? Number(next) : next.trim(),
              ),
            },
          ];
        });
        const connectionChanged = ["baseUrl", "model", "dimensions"].some(
          (field) =>
            changes.some(({ key }) => key === modelKey(profile, field)),
        );
        const protocolKey = modelKey(profile, "protocol");
        if (
          embedding &&
          connectionChanged &&
          !changes.some(({ key }) => key === protocolKey)
        ) {
          changes.push({
            key: protocolKey,
            operation: "set",
            value: JSON.stringify(protocol),
          });
        }
        return save(changes);
      }}
    >
      {fields.map((field) => {
        const key = modelKey(profile, field);
        const id = `${uid}-${field}`;
        const label = t(`aiSettings.fields.${field}`);
        if (field === "type") {
          return (
            <Choice
              key={key}
              label={label}
              selected={type}
              options={choiceOptions(modelTypes, (kind) =>
                t(`aiSettings.types.${kind}`),
              )}
              onChange={(next) => {
                const options = modelProtocols(next);
                setType(next);
                if (!options.includes(protocol)) setProtocol(options[0]);
              }}
            />
          );
        }
        if (field === "protocol") {
          return (
            <Choice
              key={key}
              label={label}
              selected={protocol}
              options={choiceOptions(modelProtocols(type), (kind) =>
                t(`choices.${kind}`),
              )}
              onChange={setProtocol}
            />
          );
        }
        const secret = field === "token";
        const numeric = field === "dimensions";
        const endpoint = field === "baseUrl" || field === "responsesBaseUrl";
        return (
          <div key={key} className="grid gap-2">
            <Label htmlFor={id}>{label}</Label>
            <Input
              id={id}
              name={key}
              type={
                secret
                  ? "password"
                  : numeric
                    ? "number"
                    : endpoint
                      ? "url"
                      : "text"
              }
              min={numeric ? 1 : undefined}
              max={numeric ? 8192 : undefined}
              autoComplete={secret ? "new-password" : "off"}
              maxLength={
                field === "providerId"
                  ? 64
                  : field === "displayName" || field === "model"
                    ? 100
                    : 500
              }
              disabled={disabledField(field)}
              required={field !== "responsesBaseUrl" && (!secret || !model)}
              pattern={
                field === "providerId" ? "[a-z][a-z0-9\\-]{0,63}" : undefined
              }
              title={
                field === "providerId"
                  ? t("aiSettings.providerIdHint")
                  : undefined
              }
              value={numeric ? dimensions : undefined}
              onChange={
                numeric
                  ? (event) => setDimensions(event.currentTarget.value)
                  : undefined
              }
              defaultValue={secret || numeric ? undefined : original(field)}
              placeholder={secret && model ? t("secretHint") : undefined}
            />
          </div>
        );
      })}
      {embedding && (
        <p className="text-sm text-muted-foreground">
          {t(
            protocol === "dashscope-embedding"
              ? "aiSettings.bailianHint"
              : "aiSettings.embeddingHint",
          )}
        </p>
      )}
    </SaveForm>
  );
}
