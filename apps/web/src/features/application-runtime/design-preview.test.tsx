import { readFileSync } from "node:fs";
import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import type {
  RuntimeApplicationV2,
  RuntimePageV2,
} from "@/api/generated/runtime";
import { DesignPreview } from "./design-preview";

vi.mock("./page-view", async () => {
  const { useContext } = await import("react");
  const { RuntimeRouteParams } = await import("./route-params");
  return {
    PageView: ({
      page,
      readOnly,
    }: {
      page: RuntimePageV2;
      readOnly: boolean;
    }) => (
      <output>
        {JSON.stringify({
          page: page.id,
          readOnly,
          params: useContext(RuntimeRouteParams),
        })}
      </output>
    ),
  };
});

it("previews another declared page with explicit route parameters and read-only context", async () => {
  await changeLanguage("en", false);
  const application = (
    JSON.parse(
      readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
    ) as RuntimeApplicationV2[]
  ).find((app) => app.id === "projects")!;
  render(
    <I18nextProvider i18n={i18n}>
      <DesignPreview
        identity="test"
        current={{
          application,
          version: "1.0.0.1",
          digest: "a",
          revision: 2,
          publishedVersion: "1.0.0.0",
          proposalVersion: "1.0.0.1",
          enabled: true,
          readOnly: true,
        }}
      />
    </I18nextProvider>,
  );
  const user = userEvent.setup();
  expect(await screen.findByText(/"page":"project.list"/)).toHaveTextContent(
    '"readOnly":true',
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Preview page" }),
    "project.detail",
  );
  expect(screen.queryByText(/"page":"project.detail"/)).not.toBeInTheDocument();
  await user.type(
    screen.getByRole("textbox", { name: "Route parameter: id" }),
    "42",
  );
  await user.click(
    screen.getByRole("button", { name: "Load authorized preview" }),
  );
  expect(screen.getByText(/"page":"project.detail"/)).toHaveTextContent(
    '"params":{"id":"42"}',
  );
  expect(screen.getByText(/"page":"project.detail"/)).toHaveTextContent(
    '"readOnly":true',
  );
});
