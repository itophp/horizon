import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TreePanel, TreeTable, type TreeTableRow } from "./tree";

const items = [
  {
    key: "1",
    label: "Root",
    children: [{ key: "1-1", label: "Child" }],
  },
];
const rows: TreeTableRow<{ id: string; name: string }>[] = [
  { key: "1", record: { id: "1", name: "Root" }, depth: 0, hasChildren: true },
  {
    key: "1-1",
    record: { id: "1-1", name: "Child" },
    depth: 1,
    hasChildren: false,
  },
];

describe("TreePanel", () => {
  it("selects nodes without business knowledge", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <TreePanel
        title="Tree"
        items={items}
        selectedKey=""
        onSelect={onSelect}
        allLabel="All"
        allCount={2}
      />,
    );
    await user.click(screen.getByRole("button", { name: /Root/ }));
    expect(onSelect).toHaveBeenCalledWith("1");
    await user.click(screen.getByRole("button", { name: /All/ }));
    expect(onSelect).toHaveBeenCalledWith("");
  });
});

describe("TreeTable", () => {
  const labels = {
    expand: "Expand",
    collapse: "Collapse",
    range: (from: number, to: number, total: number) =>
      `${from}-${to}/${total}`,
    page: (page: number, total: number) => `${page}/${total}`,
    previous: "Previous",
    next: "Next",
  };
  it("renders tree rows with depth and toggle state", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(
      <TreeTable
        rows={rows}
        columns={[
          { id: "name", header: "Name", cell: (r) => <>{r.name}</> },
          { id: "id", header: "ID", cell: (r) => <>{r.id}</> },
        ]}
        collapsedKeys={new Set(["1"])}
        onToggle={onToggle}
        labels={labels}
      />,
    );
    expect(screen.getByText("Root")).toBeVisible();
    expect(screen.getByText("Child")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Expand" }));
    expect(onToggle).toHaveBeenCalledWith("1");
  });
});
