import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, useLocation } from "react-router";
import { createInstance } from "i18next";
import en from "@/i18n/locales/en/showcase.json";
import enGovernance from "@/i18n/locales/en/governance.json";
import { ProjectForm } from "./form";
import { Records } from "./records";
import { projects } from "./catalog";
import { expect, it, vi } from "vitest";
function LocationProbe() {
  const location = useLocation();
  return <output data-testid="showcase-location">{location.search}</output>;
}
async function language() {
  const i = createInstance();
  await i.init({
    lng: "en",
    resources: { en: { showcase: en, governance: enGovernance } },
  });
  return i;
}
it("preserves input after failed submission and allows a successful retry", async () => {
  const user = userEvent.setup();
  const saved = vi.fn();
  render(
    <I18nextProvider i18n={await language()}>
      <ProjectForm onSaved={saved} />
    </I18nextProvider>,
  );
  await user.click(screen.getByRole("button", { name: "Continue" }));
  expect(screen.getByRole("alert")).toHaveTextContent(en.valid);
  await user.type(
    screen.getByRole("textbox", { name: en.name }),
    "Quarterly launch",
  );
  await user.click(screen.getByRole("button", { name: "Continue" }));
  await user.click(screen.getByRole("button", { name: "Continue" }));
  await user.click(screen.getByRole("checkbox", { name: en.simulate }));
  await user.click(screen.getByRole("button", { name: en.save }));
  expect(screen.getByRole("alert")).toHaveTextContent(en.failed);
  expect(saved).not.toHaveBeenCalled();
  await user.click(screen.getByRole("checkbox", { name: en.simulate }));
  await user.click(screen.getByRole("button", { name: en.save }));
  expect(saved).toHaveBeenCalledWith(en.saved, "Quarterly launch");
});
it("filters records and applies a bulk state change", async () => {
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={await language()}>
      <MemoryRouter initialEntries={["/?engine=6"]}>
        <Records state="normal" onNew={vi.fn()} />
      </MemoryRouter>
    </I18nextProvider>,
  );
  await user.type(screen.getByRole("textbox", { name: en.search }), "Atlas");
  expect(screen.getAllByRole("row")).toHaveLength(3);
  await user.click(screen.getByRole("checkbox", { name: "2 selected" }));
  await user.click(screen.getByRole("button", { name: en.batch }));
  expect(screen.getAllByText(en.done)).toHaveLength(2);
});

it("combines the organization scope with status tabs and clears both", async () => {
  const { ListPatterns } = await import("./list-patterns");
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={await language()}>
      <MemoryRouter initialEntries={["/?pattern=listing"]}>
        <ListPatterns created={[]} renamed={{}} onNew={vi.fn()} />
      </MemoryRouter>
    </I18nextProvider>,
  );
  await user.click(screen.getByRole("button", { name: en.platformTeam }));
  expect(screen.getByText("8 records", { exact: true })).toBeVisible();
  await user.click(screen.getByRole("tab", { name: en.review }));
  expect(screen.getByRole("heading", { name: en.empty })).toBeVisible();
  await user.click(screen.getByRole("button", { name: en.clear }));
  expect(screen.getByText("24 records", { exact: true })).toBeVisible();
});

it("filters ERP inventory by warehouse and opens purchase line items", async () => {
  const { ErpWorkspace } = await import("./erp-workspace");
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={await language()}>
      <MemoryRouter useTransitions={false}>
        <ErpWorkspace />
      </MemoryRouter>
    </I18nextProvider>,
  );
  await user.click(screen.getByRole("tab", { name: en.erpProcurement }));
  await user.type(
    screen.getByRole("textbox", { name: en.search }),
    "PO-202611-018",
  );
  expect(screen.getAllByRole("row")).toHaveLength(2);
  await user.click(screen.getByRole("button", { name: "PO-202611-018" }));
  expect(screen.getByRole("dialog")).toBeVisible();
  expect(screen.getByRole("tab", { name: en.erpLines })).toBeVisible();
});

it("moves kanban cards by drag and drop and through the card menu", async () => {
  const { WorkViews } = await import("./work-views");
  const user = userEvent.setup();
  render(
    <I18nextProvider i18n={await language()}>
      <WorkViews />
    </I18nextProvider>,
  );
  const doing = screen.getByRole("region", { name: en.boardDoing });
  const review = screen.getByRole("region", { name: en.boardReviewStage });
  expect(within(doing).getByText(projects[0])).toBeVisible();
  const atlasCard = within(doing)
    .getByText(projects[0])
    .closest(".sc-board-card");
  expect(atlasCard).not.toBeNull();
  fireEvent.dragStart(atlasCard as HTMLElement);
  fireEvent.drop(review);
  expect(within(review).getByText(projects[0])).toBeVisible();
  expect(screen.getByRole("status")).toHaveTextContent(
    en.boardMoved
      .replace("{{card}}", projects[0])
      .replace("{{column}}", en.boardReviewStage),
  );
  await user.click(
    within(review).getByRole("button", {
      name: en.boardActionsFor.replace("{{card}}", projects[0]),
    }),
  );
  await user.click(
    await screen.findByRole("menuitem", { name: en.boardMoveBack }),
  );
  expect(within(doing).getByText(projects[0])).toBeVisible();
});

it("filters the command palette and navigates with the keyboard", async () => {
  const { InteractionLab } = await import("./interaction-lab");
  const user = userEvent.setup();
  render(
    <MemoryRouter initialEntries={["/?pattern=interactionLab"]}>
      <LocationProbe />
      <I18nextProvider i18n={await language()}>
        <InteractionLab />
      </I18nextProvider>
    </MemoryRouter>,
  );
  await user.click(screen.getByRole("button", { name: en.commandOpen }));
  await user.type(
    screen.getByRole("textbox", { name: en.commandTitle }),
    "review",
  );
  expect(
    screen.getByRole("option", { name: new RegExp(en.reviewDesk) }),
  ).toBeVisible();
  await user.keyboard("{Enter}");
  expect(screen.getByTestId("showcase-location")).toHaveTextContent(
    "pattern=reviewDesk",
  );
});

it("validates inline edits before saving a row", async () => {
  const { InteractionLab } = await import("./interaction-lab");
  const user = userEvent.setup();
  render(
    <MemoryRouter initialEntries={["/?pattern=interactionLab"]}>
      <I18nextProvider i18n={await language()}>
        <InteractionLab />
      </I18nextProvider>
    </MemoryRouter>,
  );
  await user.click(screen.getAllByRole("button", { name: en.inlineEdit })[0]);
  const title = screen.getByRole("textbox", { name: en.name });
  await user.clear(title);
  await user.click(screen.getByRole("button", { name: en.save }));
  expect(screen.getByRole("alert")).toHaveTextContent(en.inlineValidTitle);
  await user.type(title, "Northstar refresh");
  const quantity = screen.getByRole("spinbutton", { name: en.inlineQty });
  await user.clear(quantity);
  await user.click(screen.getByRole("button", { name: en.save }));
  expect(screen.getByRole("alert")).toHaveTextContent(en.inlineValidQty);
  await user.type(quantity, "32");
  await user.click(screen.getByRole("button", { name: en.save }));
  expect(screen.getByText("Northstar refresh")).toBeVisible();
  expect(screen.getByText(en.inlineSaved)).toBeVisible();
});

it("adds, rejects duplicate, and removes tags", async () => {
  const { InteractionLab } = await import("./interaction-lab");
  const user = userEvent.setup();
  render(
    <MemoryRouter initialEntries={["/?pattern=interactionLab"]}>
      <I18nextProvider i18n={await language()}>
        <InteractionLab />
      </I18nextProvider>
    </MemoryRouter>,
  );
  const input = screen.getByRole("textbox", { name: en.tagsTitle });
  await user.type(input, "Delivery{Enter}");
  expect(screen.getByText("Delivery")).toBeVisible();
  await user.type(input, "Delivery{Enter}");
  expect(screen.getByRole("alert")).toHaveTextContent(en.tagsDuplicate);
  await user.click(
    screen.getByRole("button", {
      name: en.tagRemove.replace("{{tag}}", "Design"),
    }),
  );
  expect(screen.queryByText("Design")).toBeNull();
});
