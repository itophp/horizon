import { useId, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { ChangeEntry, ConfigEntry } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
const profiles = ["deepseek", "glm", "custom"] as const;
type Profile = (typeof profiles)[number];
export const modelTypes = ["text", "embedding", "multimodal"] as const;
type ModelType = (typeof modelTypes)[number];

export function modelKey(profile: Profile, field: string) {
  return field.includes(".") ? field : `aiModels.${profile}.${field}`;
}

export function value(entries: ConfigEntry[], key: string): string {
  try {
    const parsed: unknown = JSON.parse(
      entries.find((e) => e.key === key)?.value || "null",
    );
    return typeof parsed === "string" ? parsed : "";
  } catch {
    return "";
  }
}

export function modelsFrom(entries: ConfigEntry[]) {
  return profiles.map((profile, id) => {
    const type =
      profile === "deepseek"
        ? "multimodal"
        : value(entries, modelKey(profile, "type")) || "text";
    return {
      id,
      profile,
      type: type as ModelType,
      name: value(entries, modelKey(profile, "displayName")),
      provider: value(entries, modelKey(profile, "providerId")),
      requestName: value(
        entries,
        modelKey(profile, profile === "glm" ? "assistant.model" : "model"),
      ),
      credential: entries.some(
        (e) =>
          e.key ===
            modelKey(
              profile,
              profile === "glm" ? "assistant.token" : "token",
            ) && e.configured,
      ),
    };
  });
}

export type Model = ReturnType<typeof modelsFrom>[number];
type Option = { id: string; label: string };
export type Save = (changes: ChangeEntry[], reason: string) => Promise<boolean>;

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

export function DraftForm({
  children,
  disabled,
  submit,
}: {
  children: ReactNode;
  disabled: boolean;
  submit: (form: FormData, reason: string) => Promise<boolean>;
}) {
  const { t } = useTranslation("governance");
  const id = useId();
  const [pending, setPending] = useState(false);
  const [saved, setSaved] = useState(false);
  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (disabled || pending) return;
    const form = new FormData(event.currentTarget);
    const reason = String(form.get("reason") ?? "").trim();
    if (reason.length < 5) return;
    setPending(true);
    setSaved(false);
    try {
      setSaved(await submit(form, reason));
    } finally {
      setPending(false);
    }
  }
  return (
    <form className="grid gap-4" onSubmit={onSubmit}>
      <fieldset disabled={disabled || pending} className="grid min-w-0 gap-4">
        {children}
        {!disabled && (
          <>
            <div className="grid gap-2">
              <Label htmlFor={id}>{t("reason")}</Label>
              <Input
                id={id}
                name="reason"
                required
                minLength={5}
                maxLength={500}
              />
            </div>
            <div>
              <Button type="submit">{t("draft")}</Button>
            </div>
          </>
        )}
      </fieldset>
      {saved && (
        <p role="status" className="text-sm text-muted-foreground">
          {t("aiSettings.draftSaved")}
        </p>
      )}
    </form>
  );
}
