import { expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { NotificationMarkdown } from "./markdown";
import { NotificationComposer } from "./notification-composer";
const mocks = vi.hoisted(() => ({
  preview: vi.fn(async () => ({ status: 200, data: { recipientCount: 1 } })),
  send: vi.fn(async () => ({ status: 201, data: { id: 7 } })),
  download: vi.fn(),
  upload: vi.fn(async () => ({
    status: 201,
    data: { id: 11, name: "plan.txt", mime: "text/plain" },
  })),
  attachment: vi.fn(async () => ({
    status: 200,
    data: { name: "secret.txt", mime: "text/plain", content: "c2FmZQ==" },
  })),
}));
vi.mock("./shared", async (original) => ({
  ...(await original<typeof import("./shared")>()),
  downloadContent: mocks.download,
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "t",
    activeDepartmentId: 1,
    account: { id: 1 },
    permissions: ["notifications:send", "files:create"],
  }),
}));
vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  notificationsAudience: vi.fn(async () => ({
    status: 200,
    data: [
      { kind: "account", id: 2, name: "Alice", recipients: 1 },
      { kind: "company", id: 3, name: "Company B", recipients: 1 },
    ],
  })),
  notificationsPreview: mocks.preview,
  notificationsSend: mocks.send,
  notificationsAttachment: mocks.attachment,
  filesUpload: mocks.upload,
}));
function wrapper(children: React.ReactNode) {
  return (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        {children}
      </QueryClientProvider>
    </I18nextProvider>
  );
}
it("renders GFM while rejecting raw HTML, dangerous URLs, external images and unbound files", async () => {
  await changeLanguage("en");
  const user = userEvent.setup();
  const { container } = render(
    wrapper(
      <NotificationMarkdown
        messageId={7}
        attachments={[{ id: 3, name: "secret.txt", mime: "text/plain" }]}
        markdown={
          "# Safe title\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\n<script>alert(1)</script>\n\n[Bad](javascript:alert%281%29)\n\n![Tracking](https://example.test/pixel.png)\n\n[Unbound](file:99)\n\n[Private guide](file:3)"
        }
      />,
    ),
  );
  expect(screen.getByRole("heading", { name: "Safe title" })).toBeVisible();
  expect(screen.getByRole("table")).toBeVisible();
  expect(container.querySelector("script")).toBeNull();
  expect(container.querySelector('a[href^="javascript:"]')).toBeNull();
  expect(container.querySelector('img[src^="https:"]')).toBeNull();
  expect(screen.queryByRole("button", { name: "Unbound" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "Private guide" }));
  await waitFor(() => expect(mocks.attachment).toHaveBeenCalledWith(7, 3));
});
it("previews recipients and submits the Markdown notification via generated SDK", async () => {
  await changeLanguage("en");
  const user = userEvent.setup();
  const sent = vi.fn();
  render(wrapper(<NotificationComposer onSent={sent} />));
  await user.type(
    screen.getByRole("textbox", { name: "Notification title" }),
    "Maintenance",
  );
  await user.click(await screen.findByRole("checkbox", { name: /Alice/ }));
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Recipient type" }),
    "company",
  );
  await user.click(screen.getByRole("checkbox", { name: /Company B/ }));
  expect(screen.getByText("2 targets selected")).toBeVisible();
  await user.type(
    screen.getByRole("textbox", { name: "Markdown body" }),
    "    Preserve indentation\n\n# Scheduled work",
  );
  await user.click(screen.getByRole("button", { name: "Review and send" }));
  await screen.findByRole("dialog", { name: "Confirm notification" });
  expect(mocks.preview).toHaveBeenCalledWith({
    targets: [
      { kind: "account", id: 2 },
      { kind: "company", id: 3 },
    ],
  });
  await user.click(screen.getByRole("button", { name: "Send notification" }));
  await waitFor(() =>
    expect(mocks.send).toHaveBeenCalledWith(
      expect.objectContaining({
        title: "Maintenance",
        markdown: "    Preserve indentation\n\n# Scheduled work",
        targets: [
          { kind: "account", id: 2 },
          { kind: "company", id: 3 },
        ],
        fileIds: [],
        email: false,
        clientKey: expect.any(String),
      }),
    ),
  );
  expect(sent).toHaveBeenCalledOnce();
});

it("inserts uploaded files outside a preceding GFM table and removes their references", async () => {
  await changeLanguage("en");
  const user = userEvent.setup();
  render(wrapper(<NotificationComposer onSent={vi.fn()} />));
  const body = screen.getByRole("textbox", { name: "Markdown body" });
  await user.type(body, "| Window | State |\n| --- | --- |\n| Today | Ready |");
  await user.upload(
    screen.getByLabelText("Upload and insert file"),
    new File(["plan content"], "plan.txt", { type: "text/plain" }),
  );
  await waitFor(() =>
    expect((body as HTMLTextAreaElement).value).toContain("file:11"),
  );
  expect(
    within(screen.getByRole("table")).queryByRole("button", {
      name: "plan.txt",
    }),
  ).toBeNull();
  await user.click(screen.getByRole("button", { name: "Remove plan.txt" }));
  expect((body as HTMLTextAreaElement).value).not.toContain("file:11");
});
