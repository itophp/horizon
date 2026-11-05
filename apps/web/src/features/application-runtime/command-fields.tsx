import { Controller, type Control, type FieldErrors } from "react-hook-form";
import type { UseQueryResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { RuntimeFieldV2, RuntimeFormGroup } from "@/api/generated/runtime";
import { FormField } from "@/features/administration/forms";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { RegisteredControl, useRuntime, type RecordData } from "./v2-registry";
import { providerOf, readProvider } from "./data";
import { evaluate, type Json, type Scope } from "./expression";
export function CommandFields({
  fields,
  groups,
  control,
  errors,
  disabled,
  scope,
  watched,
  references,
  queries,
  id,
  searches,
  onSearch,
}: {
  fields: RuntimeFieldV2[];
  groups: RuntimeFormGroup[];
  control: Control<Record<string, unknown>>;
  errors: FieldErrors<Record<string, unknown>>;
  disabled: boolean;
  scope: Scope;
  watched: RecordData;
  references: RuntimeFieldV2[];
  queries: UseQueryResult<Awaited<ReturnType<typeof readProvider>>, Error>[];
  id: string;
  searches: Record<string, string>;
  onSearch: (name: string, value: string) => void;
}) {
  const { t } = useTranslation("runtime");
  const ctx = useRuntime();
  const nodes = fields
    .filter(
      (f) => !f.hidden && evaluate(f.visibleWhen, { ...scope, form: watched }),
    )
    .map((f) => {
      const referenceIndex = references.findIndex((x) => x.name === f.name);
      const provider = f.reference
        ? providerOf(ctx.application, f.reference)
        : undefined;
      const reference = queries[referenceIndex];
      const options = provider
        ? [
            { value: 0, label: t("none") },
            ...(reference?.data?.items ?? []).map((row) => ({
              value: row[provider.keyField],
              label: String(row[provider.labelField] ?? row[provider.keyField]),
            })),
          ]
        : f.enum.map((value) => ({
            value,
            label: f.valueLabelPrefix ? t(f.valueLabelPrefix + value) : value,
          }));
      return (
        <div key={f.name}>
          {f.searchParam && (
            <label>
              {t("searchOptions", { field: t(f.labelKey) })}
              <input
                type="search"
                value={searches[f.name] ?? ""}
                onChange={(event) => onSearch(f.name, event.target.value)}
              />
            </label>
          )}
          <Controller
            key={f.name}
            control={control}
            name={f.name}
            render={({ field }) => (
              <FormField
                label={t(f.labelKey)}
                error={errors[f.name] ? t("invalid") : undefined}
                wide
              >
                <RegisteredControl
                  definition={f}
                  id={`${id}-${f.name}`}
                  name={f.name}
                  value={field.value as Json | undefined}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  disabled={
                    disabled ||
                    (f.disabledWhen?.kind
                      ? Boolean(
                          evaluate(f.disabledWhen, {
                            ...scope,
                            form: watched,
                          }),
                        )
                      : false) ||
                    f.readOnly ||
                    !!reference?.isPending ||
                    !!reference?.isError
                  }
                  options={options}
                />
              </FormField>
            )}
          />
        </div>
      );
    });
  const visible = fields.filter(
    (field) =>
      !field.hidden && evaluate(field.visibleWhen, { ...scope, form: watched }),
  );
  const content = (group: string) =>
    nodes.filter((_, index) => (visible[index]?.group ?? "") === group);
  const tabGroups = groups.filter((group) => group.type === "tabs");
  return (
    <>
      {content("")}
      {groups
        .filter((group) => group.type !== "tabs")
        .map((group) =>
          group.type === "collapse" ? (
            <details key={group.id}>
              <summary>{t(group.labelKey)}</summary>
              {content(group.id)}
            </details>
          ) : (
            <fieldset key={group.id}>
              <legend>{t(group.labelKey)}</legend>
              {content(group.id)}
            </fieldset>
          ),
        )}
      {tabGroups.length > 0 && (
        <Tabs defaultValue={tabGroups[0].id}>
          <TabsList>
            {tabGroups.map((group) => (
              <TabsTrigger key={group.id} value={group.id}>
                {t(group.labelKey)}
              </TabsTrigger>
            ))}
          </TabsList>
          {tabGroups.map((group) => (
            <TabsContent key={group.id} value={group.id}>
              {content(group.id)}
            </TabsContent>
          ))}
        </Tabs>
      )}
    </>
  );
}
