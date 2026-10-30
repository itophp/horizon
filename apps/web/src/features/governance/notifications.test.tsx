import { expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router";
import { i18n, changeLanguage } from "@/i18n";
import type { InboxMessage, SentNotification } from "@/api/generated/models";
import { NotificationMarkdown } from "./markdown";
import { NotificationComposer } from "./notification-composer";
import { NotificationsPage, NotificationDetailPage } from "./notifications";
import { NotificationComposePage } from "./notification-compose";
const mocks = vi.hoisted(() => ({
  preview: vi.fn(async () => ({ status: 200, data: { recipientCount: 1 } })),
  send: vi.fn(async () => ({ status: 201, data: { id: 7 } })),
  download: vi.fn(),
  attachment: vi.fn(async () => ({
    status: 200,
    data: { name: "secret.txt", mime: "text/plain", content: "c2FmZQ==" },
  })),
  inbox: vi.fn(async () => ({ status: 200, data: [] as InboxMessage[] })),
  sent: vi.fn(async () => ({ status: 200, data: [] as SentNotification[] })),
  message: vi.fn(async () => ({
    status: 200,
    data: undefined as InboxMessage | undefined,
  })),
  permissions: ["notifications:send", "files:create"] as string[],
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
    permissions: mocks.permissions,
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
  notificationsInbox: mocks.inbox,
  notificationsSent: mocks.sent,
  notificationsMessage: mocks.message,
  notificationsDeliveries: vi.fn(async () => ({ status: 200, data: [] })),
  notificationsPreview: mocks.preview,
  notificationsSend: mocks.send,
  notificationsAttachment: mocks.attachment,
}));
// 富文本编辑器是第三方 Lexical 组件，组件测试只验证表单接线，编辑器行为由 e2e 覆盖。
vi.mock("./notification-editor", () => ({
  NotificationEditor: ({
    value,
    onChange,
    readOnly,
  }: {
    value: string;
    onChange: (markdown: string) => void;
    readOnly?: boolean;
  }) => (
    <textarea
      aria-label="Markdown body"
      defaultValue={value}
      readOnly={readOnly}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}));
function wrapper(children: React.ReactNode, entry = "/notifications") {
  return (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <MemoryRouter initialEntries={[entry]}>{children}</MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>
  );
}
function notificationsApp(entry = "/notifications") {
  return wrapper(
    <Routes>
      <Route path="/notifications" element={<NotificationsPage />} />
      <Route
        path="/notifications/compose"
        element={<NotificationComposePage />}
      />
      <Route path="/notifications/:id" element={<NotificationDetailPage />} />
    </Routes>,
    entry,
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
it("previews recipients and submits the editor Markdown via the generated SDK", async () => {
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
    await screen.findByRole("textbox", { name: "Markdown body" }),
    "# Scheduled work",
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
        markdown: "# Scheduled work",
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
it("links the compose button to the standalone editor page instead of a tab", async () => {
  await changeLanguage("en");
  const user = userEvent.setup();
  render(notificationsApp());
  expect(screen.getByRole("tab", { name: "Inbox" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(screen.queryByRole("tab", { name: "Compose" })).toBeNull();
  const entry = screen.getByRole("link", { name: "Compose notification" });
  expect(entry).toHaveAttribute("href", "/notifications/compose");
  await user.click(entry);
  expect(
    await screen.findByRole("heading", { name: "Compose notification" }),
  ).toBeVisible();
  expect(
    screen.getByRole("heading", { name: "Message content" }),
  ).toBeVisible();
});
it("opens the sent tab from the URL after a successful send", async () => {
  await changeLanguage("en");
  render(notificationsApp("/notifications?tab=sent"));
  expect(screen.getByRole("tab", { name: "Sent" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
});
it("hides the compose entry and page without the send permission", async () => {
  await changeLanguage("en");
  mocks.permissions = ["notifications:read"];
  try {
    const { unmount } = render(notificationsApp());
    expect(
      screen.queryByRole("link", { name: "Compose notification" }),
    ).toBeNull();
    unmount();
    render(wrapper(<NotificationComposePage />, "/notifications/compose"));
    expect(
      screen.getByText("You do not have access to this page."),
    ).toBeVisible();
  } finally {
    mocks.permissions = ["notifications:send", "files:create"];
  }
});
const announcement: InboxMessage = {
  id: 11,
  event: "announcement",
  title: "Planned maintenance",
  markdown: "## Maintenance window\n\nTonight 22:00.",
  attachments: [],
  requestId: 0,
  applicantName: "",
  departmentName: "",
  status: "sent",
  read: false,
  createdAt: "2026-10-30T06:00:00Z",
};
it("lists inbox notifications as title and time only, then opens the detail page", async () => {
  await changeLanguage("en");
  const user = userEvent.setup();
  mocks.inbox.mockImplementation(async () => ({
    status: 200,
    data: [announcement],
  }));
  mocks.message.mockImplementation(async () => ({
    status: 200,
    data: announcement,
  }));
  render(notificationsApp());
  const link = await screen.findByRole("link", { name: /Planned maintenance/ });
  expect(link).toHaveAttribute("href", "/notifications/11");
  expect(link.querySelector("time")).toHaveAttribute(
    "datetime",
    "2026-10-30T06:00:00Z",
  );
  expect(
    screen.queryByRole("heading", { name: "Maintenance window" }),
  ).toBeNull();
  await user.click(link);
  expect(
    await screen.findByRole("heading", {
      level: 1,
      name: "Planned maintenance",
    }),
  ).toBeVisible();
  expect(
    await screen.findByRole("heading", { name: "Maintenance window" }),
  ).toBeVisible();
  expect(mocks.message).toHaveBeenCalledWith(11);
});
it("keeps the sent body collapsed until it is expanded", async () => {
  await changeLanguage("en");
  const user = userEvent.setup();
  mocks.sent.mockImplementation(async () => ({
    status: 200,
    data: [
      {
        id: 7,
        title: "Quarterly notice",
        markdown: "### Body block\n\nDetails",
        attachments: [],
        recipientCount: 3,
        createdAt: "2026-10-29T02:00:00Z",
      },
    ],
  }));
  render(notificationsApp("/notifications?tab=sent"));
  expect(await screen.findByText("Quarterly notice")).toBeVisible();
  expect(screen.getByText("3 recipients")).toBeVisible();
  // jsdom 不计算 details 内容的可见性，用 open 属性确认默认收起。
  const disclosure = screen.getByText("View content").closest("details");
  expect(disclosure).not.toHaveAttribute("open");
  expect(disclosure).toContainElement(
    screen.getByRole("heading", { name: "Body block" }),
  );
  await user.click(screen.getByText("View content"));
  expect(disclosure).toHaveAttribute("open");
  expect(
    await screen.findByRole("heading", { name: "Body block" }),
  ).toBeVisible();
});
