import { expect, it } from "vitest";
import { runtimeFieldV2Schema } from "@/api/generated/runtime";
import { formSchema } from "./form-schema";

const emptyExpression = {
  kind: "",
  path: "",
  literal: "",
  items: [],
  fields: [],
};

it("keeps money exact and rejects malformed typed subtable values", () => {
  const base = runtimeFieldV2Schema.parse({
    name: "title",
    labelKey: "articles:title",
    type: "string",
    component: "string",
    componentVersion: "2.0",
    required: true,
    readOnly: false,
    sensitive: false,
    derived: false,
    hidden: false,
    visibleWhen: emptyExpression,
    disabledWhen: emptyExpression,
    compute: emptyExpression,
    minLength: 0,
    maxLength: 0,
    enum: [],
    valueLabelPrefix: "",
    default: emptyExpression,
    reference: "",
    linkPage: "",
    width: 0,
    fixed: false,
  });
  const money = runtimeFieldV2Schema.parse({
    ...base,
    name: "amount",
    type: "money",
    component: "money",
  });
  expect(
    formSchema([money], {}).parse({ amount: "900719254740993.01" }).amount,
  ).toBe("900719254740993.01");
  expect(
    formSchema([money], {}).safeParse({ amount: Number("900719254740993.01") })
      .success,
  ).toBe(false);
  const table = runtimeFieldV2Schema.parse({
    ...base,
    name: "rows",
    type: "array",
    component: "subtable",
    maxItems: 2,
    children: [{ ...money, name: "amount" }],
  });
  expect(
    formSchema([table], {}).safeParse({ rows: [{ amount: false }] }).success,
  ).toBe(false);
  expect(
    formSchema([table], {}).safeParse({
      rows: [{ amount: "1" }, { amount: "2" }, { amount: "3" }],
    }).success,
  ).toBe(false);
});
