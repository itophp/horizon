import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useForm, useFieldArray, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { ListTree, SlidersHorizontal } from "lucide-react";
import {
  filesList,
  settingsWorkspace,
  settingsDraft,
  settingsPreferences,
  settingsExport,
} from "@/api/generated/baseline";
import type {
  ConfigTarget,
  ConfigDefinition,
  ConfigEntry,
  ChangeEntry,
} from "@/api/generated/models";
import { useSession } from "@/features/administration/auth";
import {
  PageHeading,
  ErrorBanner,
  Loading,
} from "@/features/administration/shared";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  ChangeHistory,
  NoAccess,
  useGovKey,
  useGovTask,
  downloadContent,
} from "./shared";
import { useConfiguration } from "./runtime";
import { DictionarySection } from "./dictionaries";

const personalDefinitions: ConfigDefinition[] = [
  ...[
    ["display.language", "enum", ["en", "zh-CN"]],
    ["display.theme", "enum", ["dark", "light"]],
    ["display.timezone", "string", []],
    ["display.dateStyle", "enum", ["short", "medium", "long"]],
    ["display.hour24", "boolean", []],
    ["display.grouping", "boolean", []],
    ["display.pageSize", "integer", []],
    ["notifications.inApp", "boolean", []],
    ["notifications.email", "boolean", []],
  ].map(([key, kind, choices]) => ({
    key: String(key),
    kind: String(kind),
    group: String(key).split(".")[0],
    choices: choices as string[],
    defaultValue: "",
    scopes: ["account"],
    secret: false,
    sensitive: false,
  })),
];
export function SettingsPage({ personal = false }: { personal?: boolean }) {
  const { t } = useTranslation("governance");
  const session = useSession();
  const runtime = useConfiguration();
  const [params, setParams] = useSearchParams();
  const canSettings = session.permissions.includes("settings:read");
  const canDictionaries = session.permissions.includes("dictionaries:read");
  const views: ("config" | "fields")[] = [];
  if (canSettings) views.push("config");
  if (canDictionaries) views.push("fields");
  const requested = params.get("tab");
  const view = views.find((item) => item === requested) ?? views[0];
  const target = { kind: "workspace" as const, id: 0 };
  const task = useGovTask();
  const query = useQuery({
    queryKey: useGovKey("settings", target.kind, target.id),
    queryFn: () =>
      settingsWorkspace({ targetKind: target.kind, targetId: target.id }),
    enabled: !personal && canSettings,
    retry: false,
  });
  if (!personal && !views.length) return <NoAccess />;
  const data = query.data?.status === 200 ? query.data.data : undefined;
  const showConfig = personal || view === "config";
  return (
    <>
      <PageHeading
        title={t(personal ? "preferences" : "settings")}
        subtitle={t(personal ? "preferencesHint" : "settingsHint")}
      >
        {!personal &&
          view === "config" &&
          session.permissions.includes("settings:export") && (
            <Button
              variant="outline"
              onClick={() =>
                task.mutate(async () => {
                  const r = await settingsExport({
                    targetKind: target.kind,
                    targetId: target.id,
                  });
                  if (r.status === 200)
                    downloadContent(
                      "settings.json",
                      JSON.stringify(r.data, null, 2),
                      "application/json",
                    );
                })
              }
            >
              {t("export")}
            </Button>
          )}
      </PageHeading>
      {!personal && views.length > 1 && (
        <div className="admin-page-tabs">
          <button
            className={view === "config" ? "active" : ""}
            onClick={() => setParams({ tab: "config" })}
          >
            <SlidersHorizontal size={15} />
            {t("generalTab")}
          </button>
          <button
            className={view === "fields" ? "active" : ""}
            onClick={() => setParams({ tab: "fields" })}
          >
            <ListTree size={15} />
            {t("fieldsTab")}
          </button>
        </div>
      )}
      {!showConfig ? (
        <DictionarySection />
      ) : (
        <>
          {!personal && (
            <div className="gov-toolbar">
              <span className="muted-text">
                {t("revision", { version: data?.revision ?? 0 })}
              </span>
            </div>
          )}
          {task.error && <ErrorBanner error={task.error} />}{" "}
          {personal ? (
            <SettingsForm
              key={runtime.entries.length ? "ready" : "loading"}
              definitions={personalDefinitions}
              entries={runtime.entries}
              target={{ kind: "account", id: session.account.id }}
              revision={0}
              personal
            />
          ) : query.isPending ? (
            <Loading />
          ) : query.error ? (
            <ErrorBanner error={query.error} />
          ) : (
            data && (
              <>
                <SettingsForm
                  key={`${target.kind}:${target.id}:${data.revision}`}
                  definitions={data.definitions}
                  entries={data.entries}
                  target={target}
                  revision={data.revision}
                />
                <ChangeHistory
                  drafts={data.drafts}
                  history={data.history}
                  domain="settings"
                  revision={data.revision}
                />
              </>
            )
          )}
        </>
      )}
    </>
  );
}
const schema = z.object({
  fields: z.array(
    z.object({
      key: z.string(),
      value: z.string(),
      operation: z.enum(["set", "reset", "clear"]),
    }),
  ),
  reason: z.string(),
});
function SettingsForm({
  definitions,
  entries,
  target,
  revision,
  personal = false,
}: {
  definitions: ConfigDefinition[];
  entries: ConfigEntry[];
  target: ConfigTarget;
  revision: number;
  personal?: boolean;
}) {
  const { t } = useTranslation("governance");
  const session = useSession();
  const task = useGovTask();
  const [group, setGroup] = useState(definitions[0]?.group ?? "display");
  const [notice, setNotice] = useState(false);
  const writable = personal || session.permissions.includes("settings:update");
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      reason: "",
      fields: definitions.map((d) => {
        const entry = entries.find((e) => e.key === d.key);
        return {
          key: d.key,
          value: d.secret ? "" : (entry?.value ?? d.defaultValue),
          operation: entry?.overridden ? ("set" as const) : ("reset" as const),
        };
      }),
    },
  });
  const logos = useQuery({
    queryKey: useGovKey("files"),
    queryFn: () => filesList(),
    enabled: !personal && session.permissions.includes("files:read"),
    retry: false,
  });
  const fields = useFieldArray({ control: form.control, name: "fields" });
  const watchedFields = useWatch({ control: form.control, name: "fields" });
  const groups = [...new Set(definitions.map((d) => d.group))];
  return (
    <form
      className="gov-settings"
      onSubmit={form.handleSubmit((values) => {
        if (!personal && values.reason.trim().length < 5) {
          form.setError("reason", { message: "invalid" });
          return;
        }
        const changes: ChangeEntry[] = [];
        for (const field of values.fields) {
          const original = entries.find((e) => e.key === field.key);
          const definition = definitions.find((d) => d.key === field.key);
          if (!definition) continue;
          if (field.operation === "reset") {
            if (original?.overridden)
              changes.push({ key: field.key, operation: "reset", value: "" });
          } else if (field.operation === "clear") {
            changes.push({ key: field.key, operation: "clear", value: "" });
          } else if (
            (!definition.secret || field.value !== "") &&
            (field.value !== original?.value || !original?.overridden)
          ) {
            try {
              JSON.parse(field.value);
            } catch {
              form.setError("root", { message: "invalid" });
              return;
            }
            changes.push({
              key: field.key,
              operation: "set",
              value: field.value,
            });
          }
        }
        if (changes.length === 0) {
          form.setError("root", { message: "noChanges" });
          return;
        }
        task.mutate(async () => {
          if (personal) await settingsPreferences({ changes });
          else
            await settingsDraft({
              target,
              baseRevision: revision,
              reason: values.reason,
              changes,
            });
          setNotice(true);
          form.clearErrors();
        });
      })}
    >
      <div className="gov-tabs" role="tablist">
        {groups.map((g) => (
          <button
            key={g}
            type="button"
            role="tab"
            aria-selected={group === g}
            onClick={() => setGroup(g)}
          >
            {t(`groups.${g}`)}
          </button>
        ))}
      </div>
      <section className="admin-panel entity-card gov-fields">
        <header className="gov-card-heading">
          <h2>{t(`groups.${group}`)}</h2>
          <p>{t("clearHint")}</p>
        </header>
        {fields.fields.map((field, index) => {
          const d = definitions[index];
          const entry = entries.find((e) => e.key === d.key);
          const current = watchedFields[index];
          const label = t(`labels.${d.key.replaceAll(".", "_")}`, {
            defaultValue: d.key,
          });
          let parsed: unknown;
          try {
            parsed = JSON.parse(current.value);
          } catch {
            parsed = current.value;
          }
          return (
            <div
              className="gov-setting-row"
              hidden={d.group !== group}
              key={field.id}
            >
              <div>
                <label htmlFor={`setting-${index}`}>{label}</label>

                <small>
                  {d.secret
                    ? entry?.configured
                      ? t("configured")
                      : t("secretHint")
                    : t("effective") + ": " + (entry?.value ?? d.defaultValue)}
                </small>
              </div>
              <div className="gov-setting-controls">
                <select
                  aria-label={`${label} · ${t("operation")}`}
                  disabled={!writable}
                  {...form.register(`fields.${index}.operation`)}
                >
                  <option value="reset">
                    {t(personal ? "useSystemDefault" : "resetDefault")}
                  </option>
                  <option value="set">{t("set")}</option>
                  {["string", "secret"].includes(d.kind) && (
                    <option value="clear">{t("clear")}</option>
                  )}
                </select>
                {d.key === "branding.logo" ? (
                  <div>
                    <select
                      id={`setting-${index}`}
                      value={String(parsed)}
                      disabled={
                        !writable ||
                        current.operation !== "set" ||
                        !session.permissions.includes("files:read")
                      }
                      onChange={(e) =>
                        form.setValue(`fields.${index}.value`, e.target.value)
                      }
                    >
                      <option value="0">{t("noLogo")}</option>
                      {logos.data?.status === 200 &&
                        logos.data.data
                          .filter((f) => f.purpose === "logo")
                          .map((f) => (
                            <option key={f.id} value={f.id}>
                              {f.name}
                            </option>
                          ))}
                    </select>
                    <small>{t("logoHint")}</small>
                  </div>
                ) : d.kind === "boolean" ? (
                  <select
                    id={`setting-${index}`}
                    disabled={!writable || current.operation !== "set"}
                    value={current.value}
                    onChange={(e) =>
                      form.setValue(`fields.${index}.value`, e.target.value)
                    }
                  >
                    <option value="true">{t("on")}</option>
                    <option value="false">{t("off")}</option>
                  </select>
                ) : d.kind === "enum" ? (
                  <select
                    id={`setting-${index}`}
                    value={typeof parsed === "string" ? parsed : ""}
                    disabled={!writable || current.operation !== "set"}
                    onChange={(e) =>
                      form.setValue(
                        `fields.${index}.value`,
                        JSON.stringify(e.target.value),
                      )
                    }
                  >
                    {d.choices.map((choice) => (
                      <option key={choice} value={choice}>
                        {t(`choices.${choice}`, { defaultValue: choice })}
                      </option>
                    ))}
                  </select>
                ) : d.kind === "string" || d.kind === "secret" ? (
                  <Input
                    id={`setting-${index}`}
                    type={d.secret ? "password" : "text"}
                    autoComplete={d.secret ? "new-password" : undefined}
                    disabled={!writable || current.operation !== "set"}
                    value={typeof parsed === "string" ? parsed : ""}
                    onChange={(e) =>
                      form.setValue(
                        `fields.${index}.value`,
                        JSON.stringify(e.target.value),
                      )
                    }
                  />
                ) : (
                  <Input
                    id={`setting-${index}`}
                    disabled={!writable || current.operation !== "set"}
                    {...form.register(`fields.${index}.value`)}
                  />
                )}
              </div>
            </div>
          );
        })}
      </section>
      {writable && (
        <div className="gov-save">
          <label>
            {!personal && (
              <>
                <span>{t("reason")}</span>
                <Input
                  placeholder={t("reasonHint")}
                  {...form.register("reason")}
                />
              </>
            )}
          </label>
          <Button type="submit" disabled={task.isPending}>
            {task.isPending ? t("busy") : t(personal ? "save" : "draft")}
          </Button>
        </div>
      )}
      {Object.values(form.formState.errors).length > 0 && (
        <p className="field-error" role="alert">
          {t(form.formState.errors.root?.message ?? "invalid")}
        </p>
      )}
      {task.error && <ErrorBanner error={task.error} />}{" "}
      {notice && <p role="status">{t("saved")}</p>}
    </form>
  );
}
