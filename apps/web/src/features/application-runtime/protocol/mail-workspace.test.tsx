import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import type { ReactElement } from "react";
import { expect, it, vi } from "vitest";
import { i18n } from "@/i18n";
import { EmailHtml } from "./email-html";
import { MailWorkspace } from "./mail-workspace";

const mocks = vi.hoisted(() => ({
  mailboxes: vi.fn(async () => ({
    status: 200,
    data: {
      items: [
        {
          id: 8,
          name: "Work",
          emailAddress: "ada@example.test",
          defaultSend: true,
          status: "active",
          revision: 1,
          channels: [
            {
              direction: "send",
              protocol: "smtp",
              host: "smtp.old.test",
              port: 587,
              tlsMode: "starttls",
              username: "ada@example.test",
              verificationStatus: "ok",
              credential: { version: 1, fingerprint: "fingerprint" },
            },
            {
              direction: "receive",
              protocol: "pop3",
              host: "pop.old.test",
              port: 995,
              tlsMode: "tls",
              username: "ada@example.test",
              verificationStatus: "ok",
              credential: { version: 1, fingerprint: "fingerprint" },
            },
          ],
        },
      ],
    },
  })),
  onboard: vi.fn(async () => ({
    status: 200,
    data: { id: 9, status: "active" },
  })),
  update: vi.fn(async () => ({ status: 200, data: { id: 8 } })),
  credential: vi.fn(async () => ({
    status: 200,
    data: { mailboxId: 8, purpose: "smtp", version: 2 },
  })),
  deliveries: vi.fn(async () => ({
    status: 200,
    data: {
      items: [
        {
          id: 12,
          mailboxId: 8,
          messageId: 20,
          status: "failed",
          attempts: 1,
          nextAttemptAt: "",
          errorCode: "unavailable",
          sentAt: "",
          createdAt: "2026-11-19T04:00:00Z",
          subject: "Retry me",
          from: "ada@example.test",
        },
      ],
    },
  })),
  retry: vi.fn(async () => ({
    status: 200,
    data: { id: 12, status: "queued" },
  })),
  upload: vi.fn(async () => ({
    status: 200,
    data: { id: 30, name: "evidence.txt" },
  })),
  send: vi.fn(async () => ({
    status: 200,
    data: { messageId: 20, deliveryId: 12, status: "queued" },
  })),
}));

vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  mailMailboxList: mocks.mailboxes,
  mailMailboxOnboard: mocks.onboard,
  mailMailboxUpdate: mocks.update,
  mailCredentialReplace: mocks.credential,
  mailDeliveryList: mocks.deliveries,
  mailDeliveryRetry: mocks.retry,
  filesUpload: mocks.upload,
  mailMessageSend: mocks.send,
}));

function view(node: ReactElement) {
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <MemoryRouter initialEntries={["/mail"]}>{node}</MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

it("renders sanitized HTML in a sandboxed frame and falls back to plain text", () => {
  view(
    <EmailHtml
      html="<p onclick='alert(1)'>Safe</p><script>alert(2)</script>"
      text="Safe"
    />,
  );
  // The component deliberately does not normalize the server-sanitized source in this test.
  const frame = screen.getByTitle("Message body");
  expect(frame).toHaveAttribute(
    "sandbox",
    "allow-popups allow-popups-to-escape-sandbox",
  );
  expect(frame.getAttribute("srcdoc")).toContain("<p");

  view(<EmailHtml text="Plain fallback" />);
  expect(screen.getByText("Plain fallback")).toBeVisible();
});

it("automatically prefills protocol hosts, ports and usernames from the email domain", async () => {
  const user = userEvent.setup();
  view(
    <MailWorkspace
      props={{ mode: "mailboxes" }}
      readOnly={false}
      emit={vi.fn()}
    />,
  );
  await user.type(await screen.findByLabelText("Display name"), "Personal");
  await user.type(screen.getByLabelText("Email address"), "ada@example.test");
  await user.type(
    screen.getByLabelText("Password or app password"),
    "app-password",
  );
  expect(screen.getByLabelText("SMTP host")).toHaveValue("smtp.example.test");
  expect(screen.getByLabelText("POP3S host")).toHaveValue("pop.example.test");
  await user.click(screen.getByRole("button", { name: "Connect mailbox" }));
  await waitFor(() =>
    expect(mocks.onboard).toHaveBeenCalledWith({
      name: "Personal",
      emailAddress: "ada@example.test",
      password: "app-password",
      smtpHost: "smtp.example.test",
      smtpPort: 587,
      smtpTls: "starttls",
      smtpUsername: "ada@example.test",
      pop3Host: "pop.example.test",
      pop3Port: 995,
      pop3Username: "ada@example.test",
    }),
  );
});

it("automatically selects the default mailbox for composing", async () => {
  view(
    <MailWorkspace
      props={{ mode: "compose" }}
      readOnly={false}
      emit={vi.fn()}
    />,
  );
  expect(await screen.findByText("ada@example.test")).toBeVisible();
});

it("edits an existing mailbox and replaces only supplied credentials", async () => {
  const user = userEvent.setup();
  view(
    <MailWorkspace
      props={{ mode: "mailboxes" }}
      readOnly={false}
      emit={vi.fn()}
    />,
  );
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const editorFields = screen.getAllByLabelText("SMTP host");
  expect(editorFields[1]).toHaveValue("smtp.old.test");
  expect(screen.getAllByLabelText("POP3S host")[1]).toHaveValue("pop.old.test");
  await user.clear(editorFields[1]);
  await user.type(editorFields[1], "smtp.new.test");
  await user.type(screen.getByLabelText("New SMTP password"), "new-secret");
  await user.click(screen.getByRole("button", { name: "Save mailbox" }));
  await waitFor(() => expect(mocks.update).toHaveBeenCalled());
  await waitFor(() => expect(mocks.credential).toHaveBeenCalledTimes(1));
  expect(mocks.credential).toHaveBeenCalledWith(8, "smtp", "new-secret");
});

it("offers a manual retry after a terminal delivery failure", async () => {
  const user = userEvent.setup();
  view(
    <MailWorkspace
      props={{ mode: "deliveries" }}
      readOnly={false}
      emit={vi.fn()}
    />,
  );
  await user.click(await screen.findByRole("button", { name: "Retry" }));
  await waitFor(() => expect(mocks.retry).toHaveBeenCalledWith(12));
});

it("uploads an attachment and includes its managed file ID when queueing email", async () => {
  const user = userEvent.setup();
  view(
    <MailWorkspace
      props={{ mode: "compose" }}
      readOnly={false}
      emit={vi.fn()}
    />,
  );
  await user.type(await screen.findByLabelText("To"), "recipient@example.test");
  await user.type(screen.getByLabelText("Subject"), "With attachment");
  await user.type(screen.getByLabelText("Markdown body"), "Body");
  await user.upload(
    screen.getByLabelText("Attachments"),
    new File(["evidence"], "evidence.txt", { type: "text/plain" }),
  );
  expect(await screen.findByText("evidence.txt")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Queue email" }));
  await waitFor(() => expect(mocks.send).toHaveBeenCalled());
  expect(mocks.upload).toHaveBeenCalledWith(
    expect.objectContaining({
      name: "evidence.txt",
      purpose: "mail_attachment",
    }),
  );
  expect(mocks.send).toHaveBeenLastCalledWith(
    expect.objectContaining({ fileIds: [30] }),
  );
});
