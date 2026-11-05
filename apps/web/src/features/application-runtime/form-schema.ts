import { z } from "zod";
import type { RuntimeFieldV2 } from "@/api/generated/runtime";
import { evaluate, type Scope } from "./expression";
import type { RecordData } from "./v2-registry";
export function formSchema(fields: RuntimeFieldV2[], scope: Scope) {
  return z.record(z.string(), z.unknown()).superRefine((values, refinement) => {
    for (const f of fields) {
      if (
        f.hidden ||
        f.readOnly ||
        !evaluate(f.visibleWhen, { ...scope, form: values as RecordData })
      )
        continue;
      const value = values[f.name];
      if (
        !f.required &&
        (f.type === "integer" ||
          f.type === "number" ||
          f.type === "reference") &&
        (value === "" || value === undefined || value === null)
      )
        continue;
      let invalid =
        f.required && (value === undefined || value === null || value === "");
      if (f.type === "integer" || f.type === "number" || f.type === "reference")
        invalid ||=
          typeof value !== "number" ||
          !Number.isFinite(value) ||
          (f.type !== "number" && !Number.isInteger(value)) ||
          (f.minimum !== undefined && Number(value) < f.minimum) ||
          (f.maximum !== undefined && Number(value) > f.maximum);
      else if (f.type === "boolean") invalid ||= typeof value !== "boolean";
      else if (f.type === "array" || f.multiple) {
        invalid ||=
          !Array.isArray(value) ||
          value.length > (f.maxItems || 100) ||
          value.length < (f.minItems || 0);
        if (Array.isArray(value) && f.children?.length) {
          for (const row of value)
            if (!formSchema(f.children, scope).safeParse(row).success)
              invalid = true;
        }
      } else {
        invalid ||= typeof value !== "string";
        if (typeof value === "string") {
          invalid ||=
            (!!f.minLength && [...value].length < f.minLength) ||
            (!!f.maxLength && [...value].length > f.maxLength) ||
            (f.enum.length > 0 && !f.enum.includes(value));
          if (f.type === "money" && value)
            invalid ||= !/^-?(?:0|[1-9]\d*)(?:\.\d+)?$/.test(value);
          if (f.type === "time" && value)
            invalid ||= !/^(?:[01]\d|2[0-3]):[0-5]\d(?::[0-5]\d)?$/.test(value);
          if (f.type === "datetime" && value)
            invalid ||=
              !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(value) ||
              Number.isNaN(Date.parse(value));
          if (f.type === "date" && value)
            invalid ||=
              !/^\d{4}-\d{2}-\d{2}$/.test(value) ||
              Number.isNaN(Date.parse(value)) ||
              new Date(value + "T00:00:00Z").toISOString().slice(0, 10) !==
                value;
        }
      }
      if (invalid)
        refinement.addIssue({
          code: "custom",
          path: [f.name],
          message: "invalid",
        });
    }
  });
}
