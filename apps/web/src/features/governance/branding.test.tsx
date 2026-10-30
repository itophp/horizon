import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider, useTranslation } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { ThemeProvider } from "@/app/providers/theme";
import { useBranding } from "./branding";
vi.mock("@/features/administration/auth", () => ({
  useAuth: () => ({ profile: undefined }),
}));
vi.mock("@/api/generated/baseline", async (original) => {
  const actual = await original<typeof import("@/api/generated/baseline")>();
  return {
    ...actual,
    brandingGet: vi.fn(async () => ({
      status: 200,
      data: {
        revision: "brand",
        entries: [
          {
            key: "branding.tagline",
            value: '"Administration workspace"',
            source: "public",
            configured: false,
            overridden: false,
          },
          {
            key: "branding.footer",
            value: '""',
            source: "public",
            configured: true,
            overridden: false,
          },
        ],
      },
    })),
  };
});
function BrandView() {
  const brand = useBranding();
  const { t } = useTranslation();
  return (
    <>
      <p>{brand.text("branding.tagline", t("admin:workspace"))}</p>
      <footer>{brand.text("branding.footer", t("admin:footer"))}</footer>
    </>
  );
}
it("localizes default branding while preserving an explicitly empty published value", async () => {
  await changeLanguage("zh-CN", false);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        <QueryClientProvider client={client}>
          <BrandView />
        </QueryClientProvider>
      </ThemeProvider>
    </I18nextProvider>,
  );
  expect(await screen.findByText("管理工作空间")).toBeVisible();
  await vi.waitFor(() =>
    expect(screen.getByRole("contentinfo")).toBeEmptyDOMElement(),
  );
});
