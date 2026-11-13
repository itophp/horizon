import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router";
import { I18nextProvider } from "react-i18next";
import { afterEach, expect, it, vi } from "vitest";
import { runtimeUIContracts } from "@/api/generated/runtime";
import { i18n } from "@/i18n";
import { createAdminRegistry } from "./admin-components";
import { ProtocolRenderer } from "./layout-renderer";
import type { ComponentSpec } from "./types";

const registry = createAdminRegistry();
afterEach(() => vi.unstubAllGlobals());

it("resolves every generated element contract in the default registry", () => {
  for (const contract of runtimeUIContracts) {
    if (contract.category === "element") {
      expect(
        registry.resolve(contract.name, contract.version),
        contract.name,
      ).toBeTypeOf("function");
    }
  }
});

function view(
  component: ComponentSpec,
  readOnly = false,
  dataMap: Record<string, unknown> = {},
) {
  const handleEvent = vi.fn();
  function Query() {
    return <output>{useLocation().search}</output>;
  }
  render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={["/example?filter=active&page=2"]}>
        <ProtocolRenderer
          node={{ type: "component", component }}
          dataMap={dataMap}
          dataIndex={{}}
          registry={registry}
          onEvent={handleEvent}
          readOnly={readOnly}
        />
        <Query />
      </MemoryRouter>
    </I18nextProvider>,
  );
  return handleEvent;
}

it("dispatches a typed checkbox event through the Runtime renderer", async () => {
  const events = [{ event: "change", action: "setData", target: "_checked" }];
  const handleEvent = view({
    id: "checked",
    kind: "checkbox",
    version: "1.0",
    props: { labelKey: "components.loading", value: false },
    events,
  });

  await userEvent.click(screen.getByRole("checkbox", { name: "Loading" }));
  expect(handleEvent).toHaveBeenCalledWith(events, "change", true, undefined);
});

it("allows local pagination with existing query parameters", async () => {
  const handleEvent = view({
    id: "paging",
    kind: "pagination",
    version: "1.0",
    props: { labelKey: "components.pageOf", pages: 3 },
  });

  await userEvent.click(screen.getByRole("link", { name: "Next" }));
  expect(screen.getByText("?filter=active&page=3")).toBeVisible();
  expect(handleEvent).not.toHaveBeenCalled();
});

it("disables write controls in preview", async () => {
  const handleEvent = view(
    {
      id: "checked",
      kind: "checkbox",
      version: "1.0",
      props: { labelKey: "components.loading" },
    },
    true,
  );
  const checkbox = screen.getByRole("checkbox");
  expect(checkbox).toHaveAttribute("aria-disabled", "true");
  await userEvent.click(checkbox);
  expect(handleEvent).not.toHaveBeenCalled();
});

it("updates a field when its bound value changes and still accepts edits", async () => {
  const handleEvent = vi.fn();
  const component: ComponentSpec = {
    id: "bound-input",
    kind: "input",
    version: "1.0",
    props: { labelKey: "components.loading" },
    data: "$initial",
    events: [{ event: "change", action: "setData", target: "_text" }],
  };
  function field(initial: string) {
    return (
      <I18nextProvider i18n={i18n}>
        <ProtocolRenderer
          node={{ type: "component", component }}
          dataMap={{ initial }}
          dataIndex={{}}
          registry={registry}
          onEvent={handleEvent}
        />
      </I18nextProvider>
    );
  }
  const { rerender } = render(field("first"));
  const input = screen.getByRole("textbox");
  await userEvent.type(input, "!");
  expect(input).toHaveValue("first!");
  expect(handleEvent).toHaveBeenLastCalledWith(
    component.events,
    "change",
    "first!",
    undefined,
  );
  rerender(field("replacement"));
  expect(input).toHaveValue("replacement");
});

it("emits both ends of a date range without converting to timestamps", async () => {
  const handleEvent = view({
    id: "calendar",
    kind: "calendar",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      mode: "range",
      range: { from: "2026-11-06" },
    },
    events: [{ event: "change", action: "setData", target: "_range" }],
  });
  await userEvent.click(screen.getByRole("button", { name: /November 9/ }));
  expect(handleEvent).toHaveBeenLastCalledWith(
    [{ event: "change", action: "setData", target: "_range" }],
    "change",
    { from: "2026-11-06", to: "2026-11-09" },
    undefined,
  );
});

it("emits all active toggles and ignores disabled options", async () => {
  const events = [{ event: "change", action: "setData", target: "_active" }];
  const handleEvent = view({
    id: "toggles",
    kind: "toggle-group",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      multiple: true,
      values: ["first"],
      options: [
        { value: "first", labelKey: "components.previousPage" },
        { value: "second", labelKey: "components.nextPage" },
        { value: "blocked", labelKey: "components.cancel", disabled: true },
      ],
    },
    events,
  });
  await userEvent.click(screen.getByRole("button", { name: "Next" }));
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "change",
    ["first", "second"],
    undefined,
  );
  await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(handleEvent).toHaveBeenCalledTimes(1);
});

it("renders multiple independently expandable accordion sections", async () => {
  const events = [{ event: "change", action: "setData", target: "_sections" }];
  const handleEvent = view({
    id: "sections",
    kind: "accordion",
    version: "1.0",
    props: { labelKey: "components.loading", group: true, multiple: true },
    events,
    content: {
      id: "items",
      kind: "stack",
      props: {},
      children: ["first", "second"].map((value, index) => ({
        type: "layout",
        layout: {
          id: `item-${value}`,
          kind: "stack",
          props: {},
          children: [
            {
              type: "component",
              component: {
                id: value,
                kind: "accordion-item",
                version: "1.0",
                props: {
                  value,
                  labelKey: index
                    ? "components.nextPage"
                    : "components.previousPage",
                },
              },
            },
          ],
        },
      })),
    },
  });
  const previous = screen.getByRole("button", { name: "Previous" });
  const next = screen.getByRole("button", { name: "Next" });
  await userEvent.click(previous);
  await userEvent.click(next);
  expect(previous).toHaveAttribute("aria-expanded", "true");
  expect(next).toHaveAttribute("aria-expanded", "true");
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "change",
    ["first", "second"],
    undefined,
  );
});

it("dispatches the open state when toggling a popover", async () => {
  const events = [{ event: "openChange", action: "setData", target: "_open" }];
  const handleEvent = view({
    id: "pop",
    kind: "popover",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      titleKey: "components.notifications",
      side: "top",
    },
    events,
  });
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Loading" });
  await user.click(trigger);
  expect(await screen.findByText("Notifications")).toBeVisible();
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "openChange",
    true,
    undefined,
  );
  await user.keyboard("{Escape}");
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "openChange",
    false,
    undefined,
  );
});

it("dispatches the menu and command for a grouped menubar", async () => {
  const events = [{ event: "select", action: "setData", target: "_command" }];
  const handleEvent = view({
    id: "commands",
    kind: "menubar",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      menus: [
        { value: "first", labelKey: "components.previousPage", options: [] },
        {
          value: "second",
          labelKey: "components.nextPage",
          options: [{ value: "close", labelKey: "components.close" }],
        },
      ],
    },
    events,
  });
  const user = userEvent.setup();
  await user.click(screen.getByRole("menuitem", { name: "Next" }));
  await user.click(await screen.findByRole("menuitem", { name: "Close" }));
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "select",
    { menu: "second", value: "close" },
    undefined,
  );
});

it("shows all bound series and retains rows with missing measurements", async () => {
  view(
    {
      id: "metrics",
      kind: "chart",
      version: "1.0",
      props: {
        titleKey: "components.notifications",
        labelKey: "components.loading",
        series: [
          { field: "planned", labelKey: "components.previousPage" },
          { field: "actual", labelKey: "components.nextPage" },
        ],
      },
      data: "$metrics",
    },
    false,
    {
      metrics: [
        { label: "June", planned: 100, actual: 93 },
        { label: "July", actual: 111 },
      ],
    },
  );
  await userEvent.click(screen.getByText("Chart data"));
  const table = screen.getByRole("table");
  expect(
    within(table).getByRole("columnheader", { name: "Previous" }),
  ).toBeVisible();
  expect(
    within(table).getByRole("columnheader", { name: "Next" }),
  ).toBeVisible();
  const july = within(table).getByRole("row", { name: /July/ });
  expect(
    within(july)
      .getAllByRole("cell")
      .map((cell) => cell.textContent),
  ).toEqual(["July", "", "111"]);
});

it("renders grouped navigation triggers and opens the selected content", async () => {
  view({
    id: "navigation",
    kind: "navigation-menu",
    version: "1.0",
    props: { labelKey: "components.loading", group: true },
    content: {
      id: "navigation-items",
      kind: "stack",
      props: {},
      children: ["previousPage", "nextPage"].map((label, index) => ({
        type: "layout",
        layout: {
          id: label,
          kind: "stack",
          props: {},
          children: [
            {
              type: "component",
              component: {
                id: `nav-${index}`,
                kind: "navigation-menu-item",
                version: "1.0",
                props: { labelKey: `components.${label}` },
                content: {
                  id: `content-${index}`,
                  kind: "stack",
                  props: {},
                  children: [
                    {
                      type: "component",
                      component: {
                        id: `label-${index}`,
                        kind: "typography",
                        version: "1.0",
                        props: {
                          labelKey: index
                            ? "components.notifications"
                            : "components.close",
                        },
                      },
                    },
                  ],
                },
              },
            },
          ],
        },
      })),
    },
  });
  expect(screen.getByRole("button", { name: "Previous" })).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Next" }));
  expect(await screen.findByText("Notifications")).toBeVisible();
});

it("jumps directly to the final numbered page and preserves filters", async () => {
  view({
    id: "pagination",
    kind: "pagination",
    version: "1.0",
    props: { labelKey: "components.loading", pages: 10, numbered: true },
  });
  expect(screen.getByRole("link", { name: "Go to page 2" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await userEvent.click(screen.getByRole("link", { name: "Go to page 10" }));
  expect(screen.getByText("?filter=active&page=10")).toBeVisible();
  expect(screen.getByRole("link", { name: "Go to page 10" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});

it("dispatches the checked state of a dropdown menu option", async () => {
  const events = [{ event: "select", action: "setData", target: "_menu" }];
  const handleEvent = view({
    id: "menu",
    kind: "dropdown-menu",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      options: [
        { value: "enabled", labelKey: "components.nextPage", kind: "checkbox" },
      ],
    },
    events,
  });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Loading" }));
  await user.click(
    await screen.findByRole("menuitemcheckbox", { name: "Next" }),
  );
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "select",
    { value: "enabled", checked: true },
    undefined,
  );
});

it("shows the bound selections in a multiple combobox", () => {
  view({
    id: "choice",
    kind: "combobox",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      multiple: true,
      values: ["first", "second"],
      options: [
        { value: "first", labelKey: "components.previousPage" },
        { value: "second", labelKey: "components.nextPage" },
      ],
    },
  });
  expect(screen.getByRole("combobox", { name: "Loading" })).toBeVisible();
  expect(screen.getByText("Previous")).toBeVisible();
  expect(screen.getByText("Next")).toBeVisible();
});

it("reports progress against a custom numeric range", () => {
  view({
    id: "progress",
    kind: "progress",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      min: 10,
      max: 50,
      value: 20,
      showValue: true,
      format: "number",
    },
  });
  const progress = screen.getByRole("progressbar", { name: "Loading" });
  expect(progress).toHaveAttribute("aria-valuemin", "10");
  expect(progress).toHaveAttribute("aria-valuemax", "50");
  expect(progress).toHaveAttribute("aria-valuetext", "20");
});

it("opens a grouped command palette and dispatches its selection", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  const restoreScroll = Object.getOwnPropertyDescriptor(
    HTMLElement.prototype,
    "scrollIntoView",
  );
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: vi.fn(),
  });
  try {
    const events = [{ event: "select", action: "setData", target: "_command" }];
    const handleEvent = view({
      id: "palette",
      kind: "command",
      version: "1.0",
      props: {
        labelKey: "components.loading",
        mode: "dialog",
        options: [
          {
            value: "next",
            labelKey: "components.nextPage",
            groupKey: "components.notifications",
            shortcutKey: "components.close",
          },
        ],
      },
      events,
    });
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Loading" }));
    expect(
      await screen.findByRole("dialog", { name: "Loading" }),
    ).toBeVisible();
    expect(screen.getByText("Notifications")).toBeVisible();
    expect(screen.getByText("Close")).toBeVisible();
    await user.click(screen.getByRole("option", { name: /Next/ }));
    expect(handleEvent).toHaveBeenLastCalledWith(
      events,
      "select",
      "next",
      undefined,
    );
  } finally {
    if (restoreScroll)
      Object.defineProperty(
        HTMLElement.prototype,
        "scrollIntoView",
        restoreScroll,
      );
    else Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
  }
});

it("associates an invalid email control with its localized error", () => {
  view({
    id: "email",
    kind: "input",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      type: "email",
      required: true,
      minLength: 3,
      maxLength: 80,
      invalid: true,
      errorKey: "components.noOptions",
    },
  });
  const input = screen.getByRole("textbox", { name: "Loading" });
  expect(input).toHaveAttribute("type", "email");
  expect(input).toBeRequired();
  expect(input).toHaveAttribute("minlength", "3");
  expect(input).toHaveAttribute("maxlength", "80");
  expect(input).toBeInvalid();
  expect(input).toHaveAccessibleDescription("No options");
});

it("emits changes from a collapsible with a bound initial state", async () => {
  const events = [{ event: "openChange", action: "setData", target: "_open" }];
  const handleEvent = view({
    id: "details",
    kind: "collapsible",
    version: "1.0",
    props: { labelKey: "components.loading", open: true },
    events,
  });
  const trigger = screen.getByRole("button", { name: "Loading" });
  expect(trigger).toHaveAttribute("aria-expanded", "true");
  await userEvent.click(trigger);
  expect(trigger).toHaveAttribute("aria-expanded", "false");
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "openChange",
    false,
    undefined,
  );
});

it("distinguishes confirmation and cancellation in a state-bound alert dialog", async () => {
  const events = [
    { event: "confirm", action: "setData", target: "_confirmed" },
    { event: "cancel", action: "setData", target: "_cancelled" },
    { event: "openChange", action: "setData", target: "_open" },
  ];
  const handleEvent = view({
    id: "confirmation",
    kind: "alert-dialog",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      titleKey: "components.notifications",
      confirmKey: "components.nextPage",
      open: true,
    },
    events,
  });
  const user = userEvent.setup();
  const dialog = await screen.findByRole("alertdialog", {
    name: "Notifications",
  });
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  expect(handleEvent).toHaveBeenCalledWith(
    events,
    "openChange",
    false,
    undefined,
  );
  expect(handleEvent).toHaveBeenCalledWith(
    events,
    "cancel",
    undefined,
    undefined,
  );
  await user.click(screen.getByRole("button", { name: "Loading" }));
  handleEvent.mockClear();
  await user.click(
    within(await screen.findByRole("alertdialog")).getByRole("button", {
      name: "Next",
    }),
  );
  expect(handleEvent).toHaveBeenCalledWith(
    events,
    "confirm",
    undefined,
    undefined,
  );
  expect(handleEvent.mock.calls.some((call) => call[1] === "cancel")).toBe(
    false,
  );
});

it("activates vertical tabs with the arrow keys when configured", async () => {
  const events = [{ event: "change", action: "setData", target: "_tab" }];
  const handleEvent = view({
    id: "vertical-tabs",
    kind: "tabs",
    version: "1.0",
    props: {
      labelKey: "components.loading",
      orientation: "vertical",
      activation: "automatic",
      options: [
        { value: "first", labelKey: "components.previousPage" },
        { value: "second", labelKey: "components.nextPage" },
      ],
    },
    events,
  });
  expect(screen.getByRole("tablist")).toHaveAttribute(
    "aria-orientation",
    "vertical",
  );
  const user = userEvent.setup();
  await user.tab();
  expect(screen.getByRole("tab", { name: "Previous" })).toHaveFocus();
  await user.keyboard("{ArrowDown}");
  expect(screen.getByRole("tab", { name: "Next" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(handleEvent).toHaveBeenLastCalledWith(
    events,
    "change",
    "second",
    undefined,
  );
});

it("mounts the requested horizontal scrollbar", () => {
  view({
    id: "scroller",
    kind: "scroll-area",
    version: "1.0",
    props: { axes: "horizontal", keepScrollbars: true },
  });
  const bars = document.querySelectorAll('[data-slot="scroll-area-scrollbar"]');
  expect(bars).toHaveLength(1);
  expect(bars[0]).toHaveAttribute("data-orientation", "horizontal");
});
