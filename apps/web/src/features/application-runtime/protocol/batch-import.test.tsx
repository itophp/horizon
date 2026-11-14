import { readFileSync } from "node:fs";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { runtimeFieldV2Schema } from "@/api/generated/runtime";
import { BatchImport } from "./batch-import";
import type { ComponentRendererProps } from "./component-registry";

const applications = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as {
  id: string;
  version: string;
  protocol: { pages: { id: string; root: unknown }[] };
}[];

function fields() {
  const page = applications
    .find((app) => app.id === "projects" && app.version === "2.0.3")
    ?.protocol.pages.find((page) => page.id === "project.import");
  const children = (
    page?.root as {
      layout: { children: { component?: { props: { fields: unknown } } }[] };
    }
  ).layout.children;
  return runtimeFieldV2Schema
    .array()
    .parse(children.find((node) => node.component)?.component?.props.fields);
}

function excel(count: number, extra: string[] = []) {
  const columns = ["code", "name", "priority", "dueDate", ...extra];
  const line = (number: number, values: string[]) => ({
    number,
    cells: values.map((value, i) => ({
      address: String.fromCharCode(65 + i) + number,
      value,
      type: "text",
    })),
  });
  return {
    sheets: ["Projects"],
    sheet: "Projects",
    rows: [
      line(1, columns),
      ...Array.from({ length: count }, (_, i) =>
        line(i + 2, [
          "00" + (i + 1),
          "Project " + (i + 1),
          "normal",
          "",
          ...extra.map(() => ""),
        ]),
      ),
    ],
  };
}

async function chooseWorkbook(
  user: ReturnType<typeof userEvent.setup>,
  button: string,
) {
  const input = screen.getByLabelText("Excel workbook");
  const opened = vi.spyOn(input, "click");
  await user.click(screen.getByRole("button", { name: button }));
  expect(opened).toHaveBeenCalledOnce();
  opened.mockRestore();
  await user.upload(input, new File(["test"], "projects.xlsx"));
}

async function mount(
  emit: ComponentRendererProps["emit"],
  definitions = fields(),
) {
  await changeLanguage("en");
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={i18n}>
      <BatchImport
        props={{
          fields: definitions,
          uniqueField: "code",
          templatePath: "/templates/projects-import.xlsx",
        }}
        emit={emit}
        readOnly={false}
        events={["parse", "extract", "submit", "success", "search-owner"]}
      />
    </I18nextProvider>,
  );
  expect(
    screen.getByRole("link", { name: "Download template" }),
  ).toHaveAttribute("href", "/templates/projects-import.xlsx");
  expect(
    screen.getByRole("link", { name: "Download template" }),
  ).toHaveAttribute("download");
  await chooseWorkbook(user, "Choose Excel file");
  await screen.findByRole("heading", { name: "Review draft" });
  return user;
}

it("submits edited values, retains failed drafts and reuses a key until a revision", async () => {
  const submissions: {
    operationKey: string;
    rows: { values: Record<string, unknown> }[];
  }[] = [];
  const emit = vi.fn<ComponentRendererProps["emit"]>(
    async (event, payload, options) => {
      if (event === "parse") options?.onResult?.(excel(9));
      if (event === "submit") {
        submissions.push(payload as (typeof submissions)[number]);
        if (submissions.length < 3) {
          options?.onError?.("conflict");
          return false;
        }
      }
      return true;
    },
  );
  const user = await mount(emit);
  await user.click(screen.getByRole("button", { name: "Row 2 · Name" }));
  const name = screen.getByRole("textbox", { name: "Row 2 · Name" });
  await user.clear(name);
  await user.type(name, "Edited project");
  await user.tab();
  await user.click(screen.getByRole("button", { name: "Next" }));
  await screen.findByRole("button", { name: "Row 10 · Name" });
  await user.click(screen.getByRole("button", { name: "Previous" }));
  expect(
    screen.getByRole("button", { name: "Row 2 · Name" }),
  ).toHaveTextContent("Edited project");

  await user.click(screen.getByRole("button", { name: "Row 2 · Priority" }));
  await user.click(screen.getByRole("combobox", { name: "Row 2 · Priority" }));
  await user.click(await screen.findByRole("option", { name: "High" }));

  const submit = screen.getByRole("button", { name: "Import 9 rows" });
  await user.click(submit);
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "The import conflicts with existing data",
  );
  await user.click(submit);
  await waitFor(() => expect(submissions).toHaveLength(2));
  expect(submissions[0].rows[0].values).toMatchObject({
    code: "001",
    name: "Edited project",
    priority: "high",
  });
  expect(submissions[1].operationKey).toBe(submissions[0].operationKey);
  await user.click(screen.getByRole("button", { name: "Remove source row 3" }));
  await user.click(screen.getByRole("button", { name: "Import 8 rows" }));
  await waitFor(() => expect(emit).toHaveBeenCalledWith("success"));
  expect(submissions[2].operationKey).not.toBe(submissions[0].operationKey);
});

it("edits multi-select cells and searches authorized reference options", async () => {
  const definitions = fields();
  definitions.push(
    {
      ...definitions[2],
      name: "tags",
      labelKey: "projects:status",
      multiple: true,
      required: false,
    },
    {
      ...definitions[0],
      name: "owner",
      labelKey: "projects:owner",
      type: "reference",
      component: "reference",
      reference: "accounts",
      required: false,
    },
  );
  const emit = vi.fn<ComponentRendererProps["emit"]>(
    async (event, _payload, options) => {
      if (event === "parse") options?.onResult?.(excel(1, ["tags"]));
      if (event === "search-owner")
        options?.onResult?.({ items: [{ value: 17, label: "Ada Chen" }] });
      return true;
    },
  );
  const user = await mount(emit, definitions);
  await user.click(screen.getByRole("button", { name: "Row 2 · Status" }));
  const tags = screen.getByRole("combobox", { name: "Row 2 · Status" });
  await user.type(tags, "High");
  await user.click(await screen.findByRole("option", { name: "High" }));
  expect(within(tags.closest("td")!).getByText("High")).toBeVisible();

  await user.click(screen.getByRole("button", { name: "Row 2 · Owner" }));
  await user.type(
    screen.getByRole("textbox", { name: "Row 2 · Owner" }),
    "Ada",
  );
  await user.click(screen.getByRole("button", { name: "Search" }));
  await user.click(screen.getByRole("combobox", { name: "Row 2 · Owner" }));
  await user.click(await screen.findByRole("option", { name: "Ada Chen" }));
  await user.click(screen.getByRole("button", { name: "Import 1 rows" }));
  expect(emit).toHaveBeenCalledWith(
    "submit",
    expect.objectContaining({
      rows: [
        {
          rowId: 1,
          values: expect.objectContaining({ tags: ["high"], owner: 17 }),
        },
      ],
    }),
    expect.anything(),
  );
});

it("keeps the current draft until AI replacement is confirmed and allows confirming an empty optional date", async () => {
  const emit = vi.fn<ComponentRendererProps["emit"]>(
    async (event, _payload, options) => {
      if (event === "parse") options?.onResult?.(excel(1));
      if (event === "extract")
        options?.onResult?.({
          rows: [
            {
              sourceRow: 3,
              values: {
                code: "WEEK-202",
                name: "AI project",
                priority: "normal",
              },
            },
          ],
        });
      return true;
    },
  );
  const user = await mount(emit);
  let finish: () => void = () => {
    throw new Error("AI extraction has not started");
  };
  const pending = new Promise<void>((resolve) => {
    finish = resolve;
  });
  const handle = emit.getMockImplementation()!;
  emit.mockImplementation(async (...args) => {
    if (args[0] === "extract") await pending;
    return handle(...args);
  });
  await chooseWorkbook(user, "AI import");
  expect(
    await screen.findByRole("button", { name: "Extracting with AI…" }),
  ).toBeDisabled();
  expect(
    screen.getByRole("button", { name: "Choose Excel file" }),
  ).toBeDisabled();
  finish();
  expect(
    await screen.findByRole("button", { name: "Replace draft" }),
  ).toBeVisible();
  expect(
    screen.getByRole("button", { name: "Row 2 · Name" }),
  ).toHaveTextContent("Project 1");
  await user.click(screen.getByRole("button", { name: "Replace draft" }));
  const submit = screen.getByRole("button", { name: "Import 1 rows" });
  expect(submit).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Row 3 · Due" }));
  await user.click(screen.getByRole("button", { name: "Confirm value" }));
  await user.click(submit);
  expect(emit).toHaveBeenCalledWith(
    "submit",
    expect.objectContaining({
      rows: [
        {
          rowId: 1,
          values: expect.objectContaining({ code: "WEEK-202", dueDate: "" }),
        },
      ],
    }),
    expect.anything(),
  );
});

it("imports inventory records using configured worksheet headers and fields", async () => {
  const emptyExpression = {
    kind: "",
    literal: "",
    path: "",
    fields: [],
    items: [],
  };
  const sku = {
    name: "sku",
    labelKey: "drive:name",
    type: "string",
    component: "string",
    componentVersion: "2.0",
    required: true,
    readOnly: false,
    sensitive: false,
    derived: false,
    hidden: false,
    visibleWhen: emptyExpression,
    minLength: 1,
    maxLength: 24,
    enum: [],
    valueLabelPrefix: "",
    default: emptyExpression,
    reference: "",
    linkPage: "",
    width: 0,
    fixed: false,
  };
  const emit = vi.fn<ComponentRendererProps["emit"]>(
    async (event, _payload, options) => {
      if (event === "parse")
        options?.onResult?.({
          sheet: "Inventory",
          sheets: ["Inventory"],
          rows: [
            {
              number: 1,
              cells: [{ address: "A1", type: "text", value: "Stock ID" }],
            },
            {
              number: 2,
              cells: [{ address: "A2", type: "text", value: "SKU-007" }],
            },
          ],
        });
      return true;
    },
  );
  await changeLanguage("en");
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={i18n}>
      <BatchImport
        props={{
          fields: [sku],
          headers: { "Stock ID": "sku" },
          maxRows: 1,
          templatePath: "/templates/inventory.xlsx",
        }}
        emit={emit}
        readOnly={false}
        events={["parse", "submit", "success"]}
      />
    </I18nextProvider>,
  );
  await chooseWorkbook(user, "Choose Excel file");
  expect(await screen.findByText("SKU-007")).toBeVisible();
  expect(screen.queryByRole("button", { name: "AI import" })).toBeNull();
  expect(screen.queryByText(/rolled back/i)).toBeNull();
  await user.click(screen.getByRole("button", { name: "Import 1 rows" }));
  expect(emit).toHaveBeenCalledWith(
    "submit",
    expect.objectContaining({
      rows: [{ rowId: 1, values: { sku: "SKU-007" } }],
    }),
    expect.anything(),
  );
});
