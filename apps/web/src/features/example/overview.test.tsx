import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { it, expect, vi } from "vitest";
import { i18n, changeLanguage } from "@/i18n";
import { Overview } from "./overview";
it("validates and renders the generated SDK response", async () => {
  await changeLanguage("en");
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (url: string) =>
        new Response(
          JSON.stringify(
            url.includes("/echo")
              ? { name: "Ada", timestamp: "2026-01-15T04:30:00Z" }
              : { status: "ok" },
          ),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
    ),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  try {
    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <Overview />
        </QueryClientProvider>
      </I18nextProvider>,
    );
    expect(await screen.findByText("API is healthy")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Send request" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Enter between 1 and 80 characters.",
    );
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "Ada");
    await userEvent.click(screen.getByRole("button", { name: "Send request" }));
    expect(await screen.findByText("Hello, Ada.")).toBeVisible();
  } finally {
    client.clear();
    vi.unstubAllGlobals();
  }
});
