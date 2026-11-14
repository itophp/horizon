import { z } from "zod";
import type { RuntimeFieldV2 } from "@/api/generated/runtime";
import { evaluate, type Json } from "../expression";
import { formSchema } from "../form-schema";
import type { RecordData } from "../components";

const excelRows = z.array(
  z.object({
    number: z.number(),
    cells: z.array(
      z.object({ address: z.string(), value: z.string(), type: z.string() }),
    ),
  }),
);

export const workbookResult = z.object({
  sheets: z.array(z.string()).nonempty(),
  sheet: z.string(),
  rows: excelRows,
});

export const extractionResult = z.object({
  rows: z.array(
    z.object({
      sourceRow: z.number(),
      values: z.record(z.string(), z.unknown()),
    }),
  ),
});

export type DraftRow = {
  id: number;
  sourceRow: number;
  values: RecordData;
  issues: Record<string, string>;
};

function fieldDefault(field: RuntimeFieldV2): Json {
  if (field.default.kind) return evaluate(field.default, {}) as Json;
  if (field.multiple || field.type === "array") return [];
  if (field.type === "boolean") return false;
  return "";
}

export function draftRows(
  records: {
    sourceRow: number;
    values: Record<string, unknown>;
    issues?: Record<string, string>;
  }[],
  fields: RuntimeFieldV2[],
): DraftRow[] {
  return records.map(({ sourceRow, values, issues }, index) => ({
    id: index + 1,
    sourceRow,
    values: Object.fromEntries(
      fields.map((field) => [
        field.name,
        (values[field.name] as Json | undefined) ?? fieldDefault(field),
      ]),
    ),
    issues: issues ?? {},
  }));
}

function cellValue(value: string, field: RuntimeFieldV2): Json {
  if (field.multiple || field.type === "array")
    return value
      .split(/[,，]/)
      .map((part) => part.trim())
      .filter(Boolean);
  if (field.type === "integer" || field.type === "number")
    return value && Number.isFinite(Number(value)) ? Number(value) : value;
  if (field.type === "boolean") {
    if (/^(true|1)$/i.test(value)) return true;
    if (/^(false|0)$/i.test(value)) return false;
  }
  return value;
}

export function standardDraft(
  rows: z.infer<typeof excelRows>,
  fields: RuntimeFieldV2[],
  headerMap: Record<string, string> | undefined,
  maxRows: number,
) {
  const header = rows[0];
  const column = (address: string) => address.replace(/\d+$/, "");
  const byColumn = new Map(
    header?.cells.map((cell) => {
      const title = cell.value.trim();
      const name =
        headerMap && Object.hasOwn(headerMap, title) ? headerMap[title] : title;
      return [column(cell.address), name];
    }),
  );
  const headers = [...byColumn.values()];
  const recognized = headers.filter((name) =>
    fields.some((field) => field.name === name),
  );
  if (
    header?.number !== 1 ||
    fields.some(
      (field) =>
        field.required && !field.default.kind && !headers.includes(field.name),
    ) ||
    new Set(recognized).size !== recognized.length
  )
    throw new Error("excel_headers");
  if (rows.length <= 1) throw new Error("excel_empty");
  if (rows.length - 1 > maxRows) throw new Error("excel_limit");
  return draftRows(
    rows.slice(1).map((row) => {
      const values: RecordData = {};
      const issues: Record<string, string> = {};
      for (const cell of row.cells) {
        const field = fields.find(
          (item) => item.name === byColumn.get(column(cell.address)),
        );
        if (!field) continue;
        if (cell.type === "formula" || cell.type === "error") {
          issues[field.name] = "excel_cell";
          continue;
        }
        if (field.reference) {
          issues[field.name] = "excel_reference";
          continue;
        }
        values[field.name] = cellValue(cell.value.trim(), field);
      }
      return { sourceRow: row.number, values, issues };
    }),
    fields,
  );
}

export function validateDraft(
  rows: DraftRow[],
  fields: RuntimeFieldV2[],
  uniqueField: string,
) {
  const schema = formSchema(fields, {});
  const counts = new Map<string, number>();
  for (const row of rows) {
    const key = String(row.values[uniqueField] ?? "").trim();
    if (key) counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return rows.map((row) => {
    const values = Object.fromEntries(
      Object.entries(row.values).map(([field, value]) => [
        field,
        typeof value === "string" ? value.trim() : value,
      ]),
    );
    const issues = { ...row.issues };
    const result = schema.safeParse(values);
    if (!result.success) {
      for (const issue of result.error.issues) {
        const field = String(issue.path[0]);
        issues[field] ??= "invalid";
      }
    }
    if ((counts.get(String(values[uniqueField])) ?? 0) > 1)
      issues[uniqueField] = "duplicate";
    return { ...row, issues };
  });
}

export function readExcel(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("excel_invalid"));
    reader.onload = () => {
      const content =
        typeof reader.result === "string" ? reader.result.split(",")[1] : "";
      if (content) resolve(content);
      else reject(new Error("excel_invalid"));
    };
    reader.readAsDataURL(file);
  });
}
