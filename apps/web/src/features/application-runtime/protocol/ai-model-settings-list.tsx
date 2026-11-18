import { useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/api/client/request";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DataTable,
  EditorDialog,
  errorText,
  RowActions,
  type AdminColumn,
} from "@/features/administration/shared";
import { ModelEditor } from "./ai-model-editor";
import {
  Choice,
  choiceOptions,
  modelTypes,
  type AIConfigEntry,
  type Model,
  type Save,
} from "./ai-model-settings-controls";

export function ModelList({
  entries,
  models,
  disabled,
  save,
}: {
  entries: AIConfigEntry[];
  models: Model[];
  disabled: boolean;
  save: Save;
}) {
  const { t } = useTranslation("governance");
  const [params, setParams] = useSearchParams();
  const [editor, setEditor] = useState<{ profile: string; model?: Model }>();
  const [error, setError] = useState("");
  function openEditor(model?: Model) {
    setError("");
    setEditor({
      profile: model?.profile ?? `custom-${crypto.randomUUID()}`,
      model,
    });
  }
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
                    onClick: () => openEditor(m),
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
      <div className="table-toolbar flex-wrap items-end">
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
        {!disabled && (
          <Button type="button" onClick={() => openEditor()}>
            <Plus aria-hidden="true" />
            {t("aiSettings.add")}
          </Button>
        )}
      </div>
      <DataTable
        rows={models.filter(({ type }) => filter === "all" || filter === type)}
        columns={columns}
      />
      <EditorDialog
        open={!disabled && Boolean(editor)}
        onOpenChange={(open) => {
          if (!open) setEditor(undefined);
        }}
        title={editor?.model ? display(editor.model) : t("aiSettings.add")}
        description={t("aiSettings.editHint")}
      >
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error === "no_changes"
              ? t("noChanges")
              : errorText(new ApiError(error, 400), t)}
          </p>
        )}
        {editor && (
          <ModelEditor
            key={editor.profile}
            profile={editor.profile}
            model={editor.model}
            entries={entries}
            save={async (changes) => {
              setError("");
              const success = await save(changes, setError);
              if (success) {
                setEditor(undefined);
              }
              return success;
            }}
          />
        )}
      </EditorDialog>
    </div>
  );
}
