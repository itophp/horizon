import { useId, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Save as SaveIcon } from "lucide-react";
import type {
  AiConfigReadOutput,
  AiConfigSaveInput,
} from "@/api/generated/capability-ai-config";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export type AIConfigEntry = AiConfigReadOutput["entries"][number];
export type AIConfigChange = AiConfigSaveInput["changes"][number];
export const modelTypes = ["text", "embedding", "multimodal"] as const;

const legacyModelKeys: Record<string, Readonly<Record<string, string>>> = {
  glm: {
    baseUrl: "agent.baseUrl",
    responsesBaseUrl: "assistant.baseUrl",
    model: "assistant.model",
    token: "assistant.token",
  },
  "legacy-embedding": {
    baseUrl: "knowledge.embedding.baseUrl",
    model: "knowledge.embedding.model",
    dimensions: "knowledge.embedding.dimensions",
    token: "knowledge.embedding.token",
  },
};

export function modelKey(profile: string, field: string) {
  return legacyModelKeys[profile]?.[field] ?? `aiModels.${profile}.${field}`;
}

export function modelProtocols(type: string) {
  return type === "embedding"
    ? ["openai-embedding", "dashscope-embedding"]
    : ["chat-completions"];
}

export function value(entries: AIConfigEntry[], key: string): string {
  try {
    const parsed: unknown = JSON.parse(
      entries.find((e) => e.key === key)?.value || "null",
    );
    return typeof parsed === "string" ? parsed : "";
  } catch {
    return "";
  }
}

export function modelsFrom(entries: AIConfigEntry[]) {
  const customProfiles = entries.flatMap(({ key }) => {
    const match = /^aiModels\.(custom-[a-z0-9-]+)\.displayName$/.exec(key);
    return match ? [match[1]] : [];
  });
  const legacyCustom = entries.some(
    ({ key, overridden }) => key.startsWith("aiModels.custom.") && overridden,
  );
  const knowledgeModels = entries.some(
    ({ key }) => key === "knowledge.embedding.profile",
  )
    ? ["legacy-embedding", "bailian"]
    : [];
  return [
    "deepseek",
    "glm",
    ...(legacyCustom ? ["custom"] : []),
    ...customProfiles,
    ...knowledgeModels,
  ].map((profile, id) => {
    const type = value(entries, modelKey(profile, "type"));
    return {
      id,
      profile,
      type: modelTypes.find((kind) => kind === type) ?? "text",
      name: value(entries, modelKey(profile, "displayName")),
      provider: value(entries, modelKey(profile, "providerId")),
      requestName: value(entries, modelKey(profile, "model")),
      credential: entries.some(
        (entry) => entry.key === modelKey(profile, "token") && entry.configured,
      ),
    };
  });
}

export type Model = ReturnType<typeof modelsFrom>[number];
type Option = { id: string; label: string };
export type Save = (
  changes: AIConfigChange[],
  onError?: (code: string) => void,
) => Promise<boolean>;

export function choiceOptions(
  ids: readonly string[],
  label: (id: string) => string,
): Option[] {
  return ids.map((id) => ({ id, label: label(id) }));
}

export function Choice({
  label,
  selected,
  options,
  onChange,
  disabled,
}: {
  label: string;
  selected: string;
  options: Option[];
  onChange: (id: string) => void;
  disabled?: boolean;
}) {
  const id = useId();
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Select
        value={selected}
        disabled={disabled}
        onValueChange={(next) => next && onChange(next)}
      >
        <SelectTrigger id={id} className="w-full">
          <SelectValue>
            {options.find((o) => o.id === selected)?.label}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {options.map(({ id, label }) => (
            <SelectItem key={id} value={id}>
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

export function SaveForm({
  children,
  disabled,
  submit,
}: {
  children: ReactNode;
  disabled: boolean;
  submit: (form: FormData) => Promise<boolean>;
}) {
  const { t } = useTranslation("governance");
  const [pending, setPending] = useState(false);
  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (disabled || pending) return;
    const form = new FormData(event.currentTarget);
    setPending(true);
    try {
      await submit(form);
    } finally {
      setPending(false);
    }
  }
  return (
    <form className="grid gap-4" onSubmit={onSubmit}>
      <fieldset disabled={disabled || pending} className="grid min-w-0 gap-4">
        {children}
        {!disabled && (
          <div>
            <Button type="submit">
              <SaveIcon aria-hidden="true" />
              {t("save")}
            </Button>
          </div>
        )}
      </fieldset>
    </form>
  );
}
