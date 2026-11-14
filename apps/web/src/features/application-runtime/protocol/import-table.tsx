import { useId, useRef, useState } from "react";
import { Check, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DataTable, type AdminColumn } from "@/features/administration/shared";
import type { RuntimeFieldV2 } from "@/api/generated/runtime";
import { RegisteredControl } from "../components";
import type { Json } from "../expression";
import type { DraftRow } from "./import-draft";
import type { ComponentRendererProps } from "./component-registry";
import { ImportChoice, type ImportOption } from "./import-choice";

export function ImportTable({
  rows,
  fields,
  disabled,
  update,
  remove,
  emit,
}: {
  rows: DraftRow[];
  fields: RuntimeFieldV2[];
  disabled: boolean;
  update: (id: number, field: string, value: Json) => void;
  remove: (id: number) => void;
  emit: ComponentRendererProps["emit"];
}) {
  const { t } = useTranslation();
  const [known, setKnown] = useState<Record<string, ImportOption[]>>({});
  const columns: AdminColumn<DraftRow>[] = [
    {
      id: "source",
      header: t("runtime:batchImport.sourceRow"),
      cell: (row) => row.sourceRow,
    },
    ...fields.map((field) => ({
      id: field.name,
      header: t(field.labelKey),
      cell: (row: DraftRow) => (
        <ImportCell
          field={field}
          row={row}
          disabled={disabled}
          emit={emit}
          known={known[field.name] ?? []}
          remember={(items) =>
            setKnown((current) => {
              const next = new Map(
                (current[field.name] ?? []).map((item) => [item.value, item]),
              );
              for (const item of items) next.set(item.value, item);
              return { ...current, [field.name]: [...next.values()] };
            })
          }
          update={(value) => update(row.id, field.name, value)}
        />
      ),
    })),
    {
      id: "remove",
      header: "",
      cell: (row) => (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled}
          aria-label={t("runtime:batchImport.removeRow", {
            row: row.sourceRow,
          })}
          onClick={() => remove(row.id)}
        >
          <Trash2 aria-hidden />
          {t("runtime:batchImport.remove")}
        </Button>
      ),
    },
  ];
  return (
    <div className="runtime-import-table">
      <DataTable rows={rows} columns={columns} />
    </div>
  );
}

function ImportCell({
  field,
  row,
  disabled,
  emit,
  update,
  known,
  remember,
}: {
  field: RuntimeFieldV2;
  row: DraftRow;
  disabled: boolean;
  emit: ComponentRendererProps["emit"];
  update: (value: Json) => void;
  known: ImportOption[];
  remember: (items: ImportOption[]) => void;
}) {
  const { t } = useTranslation();
  const id = useId();
  const [editing, setEditing] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  function closeEditor() {
    setEditing(false);
    requestAnimationFrame(() => trigger.current?.focus());
  }
  const value = row.values[field.name];
  const options = field.enum.map((item) => ({
    value: item,
    label: field.valueLabelPrefix ? t(field.valueLabelPrefix + item) : item,
  }));
  const label = t("runtime:batchImport.cellLabel", {
    row: row.sourceRow,
    field: t(field.labelKey),
  });
  const issue = row.issues[field.name];
  const shown = field.reference
    ? (Array.isArray(value) ? value : [value])
        .map((item) => known.find((option) => option.value === item)?.label)
        .filter(Boolean)
        .join(", ")
    : (Array.isArray(value) ? value : [value])
        .map(
          (item) =>
            options.find((option) => option.value === item)?.label ??
            (typeof item === "boolean"
              ? t(item ? "runtime:batchImport.yes" : "runtime:batchImport.no")
              : String(item ?? "")),
        )
        .join(", ");
  return (
    <div
      className="runtime-import-cell"
      data-invalid={Boolean(issue) || undefined}
      onKeyDown={(event) => {
        if (event.key === "Escape" && !event.defaultPrevented) closeEditor();
      }}
      onBlur={(event) => {
        if (
          !field.reference &&
          !field.multiple &&
          !field.enum.length &&
          !event.currentTarget.contains(event.relatedTarget)
        )
          setEditing(false);
      }}
    >
      {editing && !disabled ? (
        <>
          <label className="sr-only" htmlFor={id}>
            {label}
          </label>
          {field.multiple || field.reference ? (
            <ImportChoice
              id={id}
              label={label}
              field={field}
              value={value}
              options={options}
              emit={emit}
              known={known}
              remember={remember}
              update={update}
            />
          ) : field.enum.length ? (
            <Select
              value={String(value ?? "")}
              onValueChange={(next) => {
                if (next !== null) {
                  update(next);
                  closeEditor();
                }
              }}
            >
              <SelectTrigger
                id={id}
                aria-label={label}
                autoFocus
                aria-invalid={Boolean(issue)}
              >
                <SelectValue>
                  {shown || t("runtime:batchImport.emptyCell")}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {options.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <RegisteredControl
              id={id}
              name={field.name}
              definition={field}
              value={value}
              disabled={false}
              onChange={update}
              onBlur={() => {}}
              autoFocus
            />
          )}
        </>
      ) : (
        <Button
          ref={trigger}
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled}
          aria-label={label}
          aria-describedby={issue ? id + "-error" : undefined}
          onClick={() => setEditing(true)}
        >
          {shown || t("runtime:batchImport.emptyCell")}
        </Button>
      )}
      {editing &&
        (issue === "excel_cell" || issue === "excel_review") &&
        (!field.required || field.default.kind) && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              update(value ?? "");
              closeEditor();
            }}
          >
            <Check aria-hidden />
            {t("runtime:batchImport.confirmValue")}
          </Button>
        )}
      {issue && (
        <small id={id + "-error"} className="field-error">
          {t("runtime:batchImport.errors." + issue)}
        </small>
      )}
    </div>
  );
}
