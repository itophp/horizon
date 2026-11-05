import { expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import { MemberForm } from "./forms";
const api = vi.hoisted(() => ({ add: vi.fn() }));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  driveMemberAdd: api.add,
}));
it("submits the selected member type and role through the existing capability", async () => {
  await changeLanguage("en", false);
  api.add.mockResolvedValue({ data: {} });
  const saved = vi.fn().mockResolvedValue(undefined);
  const close = vi.fn();
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={new QueryClient()}>
        <MemberForm spaceId={12} onSaved={saved} onClose={close} />
      </QueryClientProvider>
    </I18nextProvider>,
  );
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("combobox", { name: i18n.t("drive:targetType") }),
  );
  await user.click(
    await screen.findByRole("option", { name: i18n.t("drive:department") }),
  );
  await user.clear(screen.getByLabelText(i18n.t("drive:targetId")));
  await user.type(screen.getByLabelText(i18n.t("drive:targetId")), "42");
  await user.click(
    screen.getByRole("combobox", { name: i18n.t("drive:role") }),
  );
  await user.click(
    await screen.findByRole("option", { name: i18n.t("drive:roles.editor") }),
  );
  await user.click(screen.getByRole("button", { name: i18n.t("drive:save") }));
  await waitFor(() =>
    expect(api.add).toHaveBeenCalledWith(12, "department", 42, "editor"),
  );
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(saved).toHaveBeenCalledOnce();
});
