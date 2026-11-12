import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import {
  CascadeSelect,
  ChipGroup,
  EntityAvatar,
  OptionCheckList,
} from "./controls";

describe("EntityAvatar", () => {
  it("renders generic initials with a deterministic tone", () => {
    const { container } = render(<EntityAvatar name="Ada Lovelace" seed={7} />);
    expect(container.textContent).toBe("AL");
    expect(container.querySelector(".person-avatar")?.className).toContain(
      "tone-2",
    );
  });
});

describe("CascadeSelect", () => {
  it("resets the child when the parent changes", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const groups = [
      {
        id: 1,
        label: "North",
        children: [
          { id: 11, label: "Alpha" },
          { id: 12, label: "Beta" },
        ],
      },
      { id: 2, label: "South", children: [{ id: 21, label: "Gamma" }] },
    ];
    const view = render(
      <CascadeSelect
        groups={groups}
        parentId={1}
        childId={11}
        onChange={onChange}
      >
        <CascadeSelect.ParentSelect placeholder="pick" />
        <CascadeSelect.ChildSelect placeholder="pick" />
      </CascadeSelect>,
    );
    await user.selectOptions(screen.getAllByRole("combobox")[0], "2");
    expect(onChange).toHaveBeenCalledWith("2", 0);
    view.rerender(
      <CascadeSelect
        groups={groups}
        parentId={2}
        childId={0}
        onChange={onChange}
      >
        <CascadeSelect.ParentSelect placeholder="pick" />
        <CascadeSelect.ChildSelect placeholder="pick" />
      </CascadeSelect>,
    );
    const selects = document.querySelectorAll("select");
    await user.selectOptions(selects[1], "21");
    expect(onChange.mock.calls).toEqual([
      ["2", 0],
      [2, "21"],
    ]);
  });
});

describe("ChipGroup", () => {
  it("reports selections by key", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <ChipGroup
        items={[
          { key: "primary", label: "Primary", selected: true },
          { key: "5", label: "Extra" },
        ]}
        onSelect={onSelect}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Extra" }));
    expect(onSelect).toHaveBeenCalledWith("5");
  });
});

describe("OptionCheckList", () => {
  it("toggles options by key", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(
      <OptionCheckList
        options={[
          { key: "1", label: "Reader", description: "12 items" },
          { key: "2", label: "Editor", badge: <span>dept</span> },
        ]}
        selected={["1"]}
        onToggle={onToggle}
      />,
    );
    await user.click(screen.getByRole("checkbox", { name: /Editor/ }));
    expect(onToggle).toHaveBeenCalledWith("2", true);
  });
});
