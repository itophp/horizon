import { readFileSync } from "node:fs";
import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import type { RuntimeApplicationV2 } from "@/api/generated/runtime";
import { DesignPreview } from "./design-preview";

vi.mock("./protocol", async () => {
  const { useContext } = await import("react");
  const { RuntimeRouteParams } = await import("./route-params");
  return {
    ProtocolPage: ({
      application,
      page,
      readOnly,
    }: {
      application: { id: string };
      page: { id: string };
      readOnly: boolean;
    }) => (
      <output>
        {JSON.stringify({
          application: application.id,
          page: page.id,
          readOnly,
          params: useContext(RuntimeRouteParams),
        })}
      </output>
    ),
  };
});

it("previews another protocol page with explicit route parameters and read-only context", async () => {
  await changeLanguage("en", false);
  const application = (
    JSON.parse(
      readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
    ) as RuntimeApplicationV2[]
  ).find((app) => app.id === "projects")!;
  render(
    <I18nextProvider i18n={i18n}>
      <DesignPreview
        current={{
          application,
          version: "2.0.0.1",
          digest: "a",
          revision: 2,
          publishedVersion: "2.0.0.0",
          proposalVersion: "2.0.0.1",
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
    '"application":"projects"',
  );
});
