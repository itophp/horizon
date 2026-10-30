import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { it, expect } from "vitest";
import { DataTable, type AdminColumn } from "./shared";
import { changeLanguage } from "@/i18n";
function ActionProbe() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen((value) => !value)}>Row actions</button>
      {open && <div role="menu">Delete record</div>}
    </>
  );
}
it("preserves an open row action when query consumers rerender columns", async () => {
  await changeLanguage("en");
  const rows = [{ id: 42 }];
  const columns = (): AdminColumn<{ id: number }>[] => [
    { id: "actions", header: "Actions", cell: () => <ActionProbe /> },
  ];
  const { rerender } = render(<DataTable rows={rows} columns={columns()} />);
  await userEvent.click(screen.getByRole("button", { name: "Row actions" }));
  expect(screen.getByRole("menu")).toBeVisible();
  rerender(<DataTable rows={[...rows]} columns={columns()} />);
  expect(screen.getByRole("menu")).toBeVisible();
});
