import { expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import { WorkflowViewer } from "./viewer";

vi.mock("./graph", () => ({
  WorkflowGraph: ({
    onSelectEdge,
  }: {
    onSelectEdge?: (id: string) => void;
  }) => (
    <button
      type="button"
      onClick={() =>
        onSelectEdge?.(JSON.stringify(["review", "publish", "branch"]))
      }
    >
      Select branch connection
    </button>
  ),
}));

it("inspects a branch and switches to the workflow input schema", async () => {
  await changeLanguage("en", false);
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={i18n}>
      <WorkflowViewer
        diagram={{
          name: "demo",
          version: "1",
          hash: "hash",
          input: { properties: { title: { type: "string" } } },
          nodes: [
            { id: "start", kind: "start", compiled: true },
            {
              id: "publish",
              kind: "step",
              compiled: false,
              capability: "article.publish",
              version: "1",
              when: { from: "review", field: "decision", equals: "approve" },
            },
          ],
          edges: [{ from: "review", to: "publish", kind: "branch" }],
        }}
      />
    </I18nextProvider>,
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Selected node" }),
    "publish",
  );
  const inspector = screen.getByRole("complementary", {
    name: "Node properties",
  });
  expect(within(inspector).getByText("article.publish")).toBeInTheDocument();
  expect(within(inspector).getByText("review.decision")).toBeInTheDocument();
  expect(
    within(inspector).getByText("Declared only · not compiled"),
  ).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Input schema" }));
  expect(screen.getByText(/"title":/)).toBeInTheDocument();
  expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
});

it("shows a clicked connection predicate and clears it when a dependency or node is selected", async () => {
  await changeLanguage("en", false);
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={i18n}>
      <WorkflowViewer
        diagram={{
          name: "demo",
          version: "1",
          hash: "hash",
          nodes: [
            { id: "start", kind: "start", compiled: true },
            {
              id: "publish",
              kind: "step",
              compiled: true,
              when: { from: "review", field: "decision", equals: "approve" },
            },
          ],
          edges: [
            { from: "review", to: "publish", kind: "branch" },
            { from: "start", to: "publish", kind: "data" },
          ],
        }}
      />
    </I18nextProvider>,
  );
  await user.click(
    screen.getByRole("button", { name: "Select branch connection" }),
  );
  const inspector = screen.getByRole("complementary", {
    name: "Connection properties",
  });
  expect(within(inspector).getByText("decision")).toBeInTheDocument();
  expect(within(inspector).getByText('"approve"')).toBeInTheDocument();
  expect(within(inspector).getByText("Equals (=)")).toBeInTheDocument();
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Selected connection" }),
    JSON.stringify(["start", "publish", "data"]),
  );
  expect(
    within(inspector).getByText(
      "No condition; this connection expresses a dependency.",
    ),
  ).toBeInTheDocument();
  expect(within(inspector).queryByText('"approve"')).not.toBeInTheDocument();
  expect(
    within(inspector).getByText(
      "This root input data connection is hidden on the canvas.",
    ),
  ).toBeInTheDocument();
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Selected node" }),
    "publish",
  );
  expect(
    screen.getByRole("complementary", { name: "Node properties" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("combobox", { name: "Selected connection" }),
  ).toHaveValue("");
});
