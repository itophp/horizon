import { useId, useMemo, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { runtimeFieldV2Schema } from "@/api/generated/runtime";
import { Button, buttonVariants } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  ArrowLeft,
  Check,
  Download,
  FileSpreadsheet,
  Import,
  Replace,
  Sparkles,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { RuntimeSelect } from "../table-select";
import type { Json } from "../expression";
import type { ComponentRendererProps } from "./component-registry";
import { ImportTable } from "./import-table";
import {
  draftRows,
  extractionResult,
  readExcel,
  standardDraft,
  validateDraft,
  workbookResult,
  type DraftRow,
} from "./import-draft";

const emptyFields: unknown[] = [];
const defaultMaxFileBytes = 2 * 1024 * 1024;
const defaultMaxRows = 100;

function positiveLimit(value: unknown, fallback: number) {
  return typeof value === "number" && Number.isSafeInteger(value) && value > 0
    ? value
    : fallback;
}
type ImportMode = "standard" | "ai";

export function BatchImport({
  props,
  readOnly,
  emit,
  events,
}: ComponentRendererProps) {
  const { t } = useTranslation();
  const hintId = useId();
  const fileInput = useRef<HTMLInputElement>(null);
  const pickerMode = useRef<ImportMode>("standard");
  const draftHeading = useRef<HTMLHeadingElement>(null);
  const operation = useRef<string | undefined>(undefined);
  const inFlight = useRef(false);
  const [source, setSource] = useState<{ file: File; mode: ImportMode }>();
  const [parsing, setParsing] = useState<ImportMode>();
  const [sheets, setSheets] = useState<string[]>([]);
  const [sheet, setSheet] = useState("");
  const [rows, setRows] = useState<DraftRow[]>([]);
  const [replacement, setReplacement] = useState<DraftRow[]>();
  const [onlyIssues, setOnlyIssues] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const rawFields = props.fields ?? emptyFields;
  const fields = useMemo(() => {
    const parsed = runtimeFieldV2Schema.array().safeParse(rawFields);
    return parsed.success
      ? parsed.data.filter(
          (field) =>
            !field.hidden &&
            !field.readOnly &&
            !field.sensitive &&
            !field.derived,
        )
      : [];
  }, [rawFields]);
  const uniqueField = String(props.uniqueField ?? "");
  const maxFileBytes = positiveLimit(props.maxFileBytes, defaultMaxFileBytes);
  const maxRows = positiveLimit(props.maxRows, defaultMaxRows);
  const headerMap = props.headers as Record<string, string> | undefined;
  const commitHintKey =
    typeof props.commitHintKey === "string" ? props.commitHintKey : "";
  const checked = useMemo(
    () => validateDraft(rows, fields, uniqueField),
    [rows, fields, uniqueField],
  );
  const invalid = checked.filter((row) => Object.keys(row.issues).length > 0);
  const label = (name: string) => t("runtime:batchImport." + name);

  function accept(next: DraftRow[]) {
    setRows(next);
    setReplacement(undefined);
    setOnlyIssues(false);
    operation.current = undefined;
    requestAnimationFrame(() => draftHeading.current?.focus());
  }

  function update(rowId: number, field: string, value: Json) {
    operation.current = undefined;
    setRows((current) =>
      current.map((row) => {
        if (row.id !== rowId) return row;
        const issues = { ...row.issues };
        delete issues[field];
        return { ...row, values: { ...row.values, [field]: value }, issues };
      }),
    );
  }

  async function request(event: string, input: object) {
    let result: unknown;
    let failure = "unavailable";
    const ok = await emit(event, input, {
      ephemeral: true,
      ...(event !== "submit" && {
        onResult: (data: unknown) => {
          result = data;
        },
      }),
      onError: (code) => {
        failure = code;
      },
    });
    if (!ok) throw new Error(failure);
    return result;
  }

  async function run(action: () => Promise<void>) {
    if (readOnly || inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError(undefined);
    try {
      await action();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "unavailable");
    } finally {
      setParsing(undefined);
      setBusy(false);
      inFlight.current = false;
    }
  }

  function chooseFile(mode: ImportMode) {
    pickerMode.current = mode;
    fileInput.current?.click();
  }

  function parse(file: File, mode: ImportMode, selectedSheet?: string) {
    if (replacement) return;
    return run(async () => {
      setParsing(mode);
      if (!/\.xlsx$/i.test(file.name)) throw new Error("excel_invalid");
      if (file.size > maxFileBytes) throw new Error("excel_limit");
      const content = await readExcel(file);
      const parsed = workbookResult.safeParse(
        await request("parse", {
          content,
          ...(selectedSheet ? { sheet: selectedSheet } : {}),
        }),
      );
      if (!parsed.success) throw new Error("excel_invalid");
      const workbook = parsed.data;
      setSheets(workbook.sheets);
      if (mode === "ai" && !selectedSheet && workbook.sheets.length > 1) return;
      setSheet(workbook.sheet);
      let next: DraftRow[];
      if (mode === "ai") {
        const extracted = extractionResult.safeParse(
          await request("extract", { content, sheet: workbook.sheet }),
        );
        if (!extracted.success) throw new Error("excel_ai_invalid");
        next = draftRows(
          extracted.data.rows.map((row) => ({
            ...row,
            issues: Object.fromEntries(
              fields
                .filter((field) => !(field.name in row.values))
                .map((field) => [field.name, "excel_review"]),
            ),
          })),
          fields,
        );
      } else {
        next = standardDraft(workbook.rows, fields, headerMap, maxRows);
      }
      if (!next.length) throw new Error("excel_empty");
      if (next.length > maxRows) throw new Error("excel_limit");
      if (rows.length) setReplacement(next);
      else accept(next);
    });
  }

  async function submit() {
    if (invalid.length || !checked.length || replacement) return;
    await run(async () => {
      operation.current ??= crypto.randomUUID();
      await request("submit", {
        operationKey: operation.current,
        rows: checked.map(({ id, values }) => ({
          rowId: id,
          values: Object.fromEntries(
            Object.entries(values).map(([field, value]) => [
              field,
              typeof value === "string" ? value.trim() : value,
            ]),
          ),
        })),
      });
      await emit("success");
    });
  }

  if (readOnly || !fields.length) return null;
  const templatePath = props.templatePath;
  if (
    typeof templatePath !== "string" ||
    !/^\/([A-Za-z0-9_-]+\/)*[A-Za-z0-9_-]+[.]xlsx$/.test(templatePath)
  ) {
    return (
      <p role="alert" className="field-error">
        {label("errors.templatePath")}
      </p>
    );
  }
  const errorKey =
    error === "conflict" && typeof props.conflictKey === "string"
      ? props.conflictKey
      : "runtime:batchImport.errors." + error;
  return (
    <section className="runtime-import" aria-label={label("title")}>
      <input
        ref={fileInput}
        type="file"
        accept=".xlsx"
        aria-label={label("file")}
        hidden
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = "";
          if (!file) return;
          const mode = pickerMode.current;
          setSource({ file, mode });
          setSheet("");
          setSheets([]);
          void parse(file, mode);
        }}
      />
      <p className="muted-text" aria-busy={Boolean(parsing)}>
        <Trans
          i18nKey={
            events?.includes("extract")
              ? "runtime:batchImport.chooseSource"
              : "runtime:batchImport.chooseStandard"
          }
          components={{
            template: (
              <a
                className={buttonVariants({ size: "sm", variant: "outline" })}
                href={templatePath}
                download
              >
                <Download aria-hidden />
                {label("downloadTemplate")}
              </a>
            ),
            excel: (
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={busy || Boolean(replacement)}
                onClick={() => chooseFile("standard")}
              >
                {parsing === "standard" ? (
                  <Spinner aria-hidden />
                ) : (
                  <FileSpreadsheet aria-hidden />
                )}
                {label(parsing === "standard" ? "parsing" : "standard")}
              </Button>
            ),
            ai: (
              <Button
                type="button"
                size="sm"
                variant="outline"
                aria-describedby={hintId}
                disabled={busy || Boolean(replacement)}
                onClick={() => chooseFile("ai")}
              >
                {parsing === "ai" ? (
                  <Spinner aria-hidden />
                ) : (
                  <Sparkles aria-hidden />
                )}
                {label(parsing === "ai" ? "extracting" : "ai")}
              </Button>
            ),
          }}
        />
      </p>
      {source && sheets.length > 1 && (
        <RuntimeSelect
          label={label("sheet")}
          value={sheet}
          options={[
            ...(!sheet ? [{ value: "", label: label("chooseSheet") }] : []),
            ...sheets.map((name) => ({ value: name, label: name })),
          ]}
          disabled={busy || Boolean(replacement)}
          onChange={(next) => {
            if (next) void parse(source.file, source.mode, next);
          }}
        />
      )}
      <span role="status" className="sr-only">
        {parsing && label(parsing === "ai" ? "extracting" : "parsing")}
      </span>
      {events?.includes("extract") && (
        <span id={hintId} className="sr-only">
          {label("aiHint")}
        </span>
      )}
      {error && (
        <p role="alert" className="field-error">
          {t(errorKey, { defaultValue: label("errors.unavailable") })}
        </p>
      )}
      {replacement && (
        <div className="runtime-import-toolbar" role="status">
          <span>{label("replaceHint")}</span>
          <Button type="button" onClick={() => accept(replacement)}>
            <Replace aria-hidden />
            {label("replace")}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => setReplacement(undefined)}
          >
            <Check aria-hidden />
            {label("keep")}
          </Button>
        </div>
      )}
      {rows.length > 0 && (
        <>
          <div className="runtime-import-toolbar">
            <h2 ref={draftHeading} tabIndex={-1}>
              {label("draft")}
            </h2>
            <span role="status">
              {t("runtime:batchImport.summary", {
                total: rows.length,
                invalid: invalid.length,
              })}
            </span>
            <label className="runtime-import-filter">
              <Checkbox checked={onlyIssues} onCheckedChange={setOnlyIssues} />
              {label("onlyIssues")}
            </label>
          </div>
          {(onlyIssues ? invalid : checked).length > 0 ? (
            <ImportTable
              rows={onlyIssues ? invalid : checked}
              fields={fields}
              disabled={busy || Boolean(replacement)}
              emit={emit}
              update={update}
              remove={(rowId) => {
                operation.current = undefined;
                setRows((current) => current.filter((row) => row.id !== rowId));
              }}
            />
          ) : (
            <p role="status">{label("noIssues")}</p>
          )}
          {commitHintKey && <p className="muted-text">{t(commitHintKey)}</p>}
        </>
      )}
      <div className="runtime-import-toolbar">
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onClick={() => emit("cancel")}
        >
          <ArrowLeft aria-hidden />
          {label("back")}
        </Button>
        <Button
          type="button"
          disabled={
            busy ||
            !rows.length ||
            Boolean(invalid.length) ||
            Boolean(replacement)
          }
          onClick={submit}
        >
          <Import aria-hidden />
          {t("runtime:batchImport.submit", { count: rows.length })}
        </Button>
      </div>
    </section>
  );
}
