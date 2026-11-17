import { useId, useState } from "react";
import { Pencil } from "lucide-react";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import type { ChangeEntry, ConfigEntry } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  DataTable,
  EditorDialog,
  RowActions,
  type AdminColumn,
} from "@/features/administration/shared";
import {
  Choice,
  DraftForm,
  choiceOptions,
  modelKey,
  modelTypes,
  value,
  type Model,
  type Save,
} from "./ai-model-settings-controls";

const fields = {
  deepseek: [
    "providerId",
    "displayName",
    "baseUrl",
    "protocol",
    "model",
    "token",
  ],
  glm: [
    "type",
    "providerId",
    "displayName",
    "agent.baseUrl",
    "assistant.baseUrl",
    "assistant.model",
    "assistant.token",
  ],
  custom: [
    "type",
    "providerId",
    "displayName",
    "baseUrl",
    "protocol",
    "model",
    "token",
  ],
};

function ModelEdit({
  model,
  entries,
  save,
}: {
  model: Model;
  entries: ConfigEntry[];
  save: Save;
}) {
  const { t } = useTranslation("governance");
  const uid = useId();
  const [type, setType] = useState<string>(model.type);
  const { profile } = model;
  return (
    <DraftForm
      disabled={false}
      submit={(form, reason) => {
        const changes = fields[profile].flatMap((field): ChangeEntry[] => {
          const key = modelKey(profile, field);
          const next = field === "type" ? type : form.get(key);
          const secret = field === "token" || field === "assistant.token";
          if (
            typeof next !== "string" ||
            next === value(entries, key) ||
            (secret && !next)
          )
            return [];
          return [{ key, operation: "set", value: JSON.stringify(next) }];
        });
        return save(changes, reason);
      }}
    >
      {profile === "deepseek" && (
        <dl className="flex flex-wrap gap-x-2 text-sm">
          <dt className="text-muted-foreground">{t("aiSettings.type")}</dt>
          <dd>{t("aiSettings.types.multimodal")}</dd>
        </dl>
      )}
      {fields[profile].map((field) => {
        const key = modelKey(profile, field);
        const id = `${uid}-${field}`;
        const label = t(`labels.${key.replaceAll(".", "_")}`, {
          defaultValue: t("aiSettings.type"),
        });
        if (field === "type") {
          return (
            <Choice
              key={key}
              label={t("aiSettings.type")}
              selected={type}
              options={choiceOptions(modelTypes, (kind) =>
                t(`aiSettings.types.${kind}`),
              )}
              onChange={setType}
            />
          );
        }
        if (field === "protocol") {
          return (
            <dl className="flex flex-wrap gap-x-2 text-sm" key={key}>
              <dt className="text-muted-foreground">{label}</dt>
              <dd>{t("choices.chat-completions")}</dd>
            </dl>
          );
        }
        const secret = field === "token" || field === "assistant.token";
        return (
          <div key={key} className="grid gap-2">
            <Label htmlFor={id}>{label}</Label>
            <Input
              id={id}
              name={key}
              type={secret ? "password" : "text"}
              autoComplete={secret ? "new-password" : "off"}
              maxLength={field === "providerId" ? 64 : 500}
              defaultValue={secret ? "" : value(entries, key)}
              placeholder={secret ? t("secretHint") : undefined}
            />
          </div>
        );
      })}
      {type === "embedding" && (
        <p className="text-sm text-muted-foreground">
          {t("aiSettings.embeddingHint")}
        </p>
      )}
      {profile === "glm" && (
        <p className="text-sm text-muted-foreground">
          {t("aiSettings.glmProtocols")}
        </p>
      )}
    </DraftForm>
  );
}

export function ModelList({
  entries,
  models,
  disabled,
  save,
}: {
  entries: ConfigEntry[];
  models: Model[];
  disabled: boolean;
  save: Save;
}) {
  const { t } = useTranslation("governance");
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState<Model>();
  const filter =
    modelTypes.find((type) => type === params.get("modelType")) ?? "all";
  const display = (m: Model) => m.name || t(`choices.${m.profile}`);
  const columns: AdminColumn<Model>[] = [
    {
      id: "name",
      header: t("aiSettings.model"),
      cell: (m) => (
        <div>
          <strong>{display(m)}</strong>
          {m.requestName && (
            <p className="text-xs text-muted-foreground">{m.requestName}</p>
          )}
        </div>
      ),
    },
    {
      id: "type",
      header: t("aiSettings.type"),
      cell: (m) => t(`aiSettings.types.${m.type}`),
    },
    {
      id: "credential",
      header: t("aiSettings.credential"),
      cell: (m) => (
        <Badge variant="outline">
          {t(m.credential ? "configured" : "aiSettings.missingKey")}
        </Badge>
      ),
    },
    ...(!disabled
      ? [
          {
            id: "actions",
            header: t("admin:fields.actions"),
            cell: (m: Model) => (
              <RowActions
                actions={[
                  {
                    label: t("admin:actions.edit"),
                    icon: <Pencil size={16} />,
                    onClick: () => setEditing(m),
                  },
                ]}
              />
            ),
          },
        ]
      : []),
  ];
  return (
    <div className="admin-panel">
      <div className="table-toolbar">
        <Choice
          label={t("aiSettings.type")}
          selected={filter}
          options={[
            { id: "all", label: t("aiSettings.allTypes") },
            ...choiceOptions(modelTypes, (type) =>
              t(`aiSettings.types.${type}`),
            ),
          ]}
          onChange={(id) =>
            setParams(
              (previous) => {
                const next = new URLSearchParams(previous);
                if (id === "all") next.delete("modelType");
                else next.set("modelType", id);
                return next;
              },
              { replace: true },
            )
          }
        />
      </div>
      <DataTable
        rows={models.filter(({ type }) => filter === "all" || filter === type)}
        columns={columns}
      />
      <EditorDialog
        open={Boolean(editing)}
        onOpenChange={(open) => {
          if (!open) setEditing(undefined);
        }}
        title={editing ? display(editing) : t("aiSettings.model")}
        description={t("aiSettings.editHint")}
      >
        {editing && (
          <ModelEdit
            key={editing.profile}
            model={editing}
            entries={entries}
            save={async (changes, reason) => {
              const saved = await save(changes, reason);
              if (saved) setEditing(undefined);
              return saved;
            }}
          />
        )}
      </EditorDialog>
    </div>
  );
}
