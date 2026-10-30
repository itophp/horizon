import { afterEach, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { ThemeProvider } from "@/app/providers/theme";
import type { SessionProfile } from "@/api/generated/models";
import { AuthBoundary, RequireSession } from "./layout";
import { LoginPage } from "./login";
const profile: SessionProfile = {
  tenantId: "t",
  csrfToken: "server-csrf",
  permissions: ["accounts:read"],
  activeDepartmentId: 1,
  account: {
    memberships: [],
    id: 1,
    username: "admin",
    fullName: "Administrator",
    email: "",
    status: "active",
    organizationId: 1,
    departmentId: 1,
    roleIds: [],
    overrides: [],
    isAdmin: true,
    createdAt: "2026-10-28T00:00:00Z",
    lastLogin: "",
  },
};
afterEach(() => vi.unstubAllGlobals());
it("validates login, keeps password hidden and opens the verified session", async () => {
  await changeLanguage("en");
  const mock = vi.fn(
    async (url: string) =>
      new Response(
        JSON.stringify(
          url === "/api/auth/login" ? { profile } : { code: "unauthenticated" },
        ),
        {
          status: url === "/api/auth/login" ? 200 : 401,
          headers: { "Content-Type": "application/json" },
        },
      ),
  );
  vi.stubGlobal("fetch", mock);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      {
        element: <AuthBoundary />,
        children: [
          { path: "/login", element: <LoginPage /> },
          {
            element: <RequireSession />,
            children: [{ path: "/", element: <h1>Verified workspace</h1> }],
          },
        ],
      },
    ],
    { initialEntries: ["/login"] },
  );
  render(
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        <QueryClientProvider client={client}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </ThemeProvider>
    </I18nextProvider>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Sign in" }));
  expect(screen.getAllByRole("alert").length).toBeGreaterThan(0);
  expect(screen.getByLabelText("Password", { exact: true })).toHaveAttribute(
    "type",
    "password",
  );
  await user.type(screen.getByRole("textbox", { name: "Account" }), "admin");
  await user.type(
    screen.getByLabelText("Password", { exact: true }),
    "TestOnly!2026Secure",
  );
  await user.click(screen.getByRole("button", { name: "Sign in" }));
  expect(
    await screen.findByRole("heading", { name: "Verified workspace" }),
  ).toBeVisible();
  expect(mock.mock.calls.some(([url]) => url === "/api/auth/login")).toBe(true);
  client.clear();
});
it("persists an explicit appearance choice", async () => {
  const user = userEvent.setup();
  localStorage.setItem("baseline.theme", "dark");
  function Probe() {
    const theme = useThemeForTest();
    return <button onClick={theme.toggle}>{theme.theme}</button>;
  }
  render(
    <ThemeProvider>
      <Probe />
    </ThemeProvider>,
  );
  await user.click(screen.getByRole("button", { name: "dark" }));
  expect(document.documentElement).not.toHaveClass("dark");
  expect(localStorage.getItem("baseline.theme")).toBe("light");
  localStorage.removeItem("baseline.theme");
});
import { useTheme as useThemeForTest } from "@/app/providers/theme";
