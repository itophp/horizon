import { render, screen, within } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { expect, it } from "vitest";
import { changeLanguage, i18n } from "@/i18n";
import { ChatFeed } from "./chat-feed";
import type { Turn } from "./use-workbench";

it("keeps each execution flow and loading indicator inside its own assistant turn", async () => {
  await changeLanguage("en", false);
  const messages: Turn[] = [
    { id: "q1", role: "user", content: "First request" },
    {
      id: "a1",
      role: "assistant",
      content: "First answer",
      progress: [
        {
          sequence: 1,
          kind: "tool_end",
          capability: "example.record.archive",
          status: "succeeded",
          delta: "",
          errorCode: "",
        },
      ],
    },
    { id: "q2", role: "user", content: "Second request" },
    {
      id: "a2",
      role: "assistant",
      content: "",
      progress: [
        {
          sequence: 1,
          kind: "planning",
          capability: "",
          status: "",
          delta: "",
          errorCode: "",
        },
      ],
    },
  ];
  const view = render(
    <I18nextProvider i18n={i18n}>
      <ChatFeed
        messages={messages}
        busy
        loading={false}
        onSuggestion={() => {}}
      />
    </I18nextProvider>,
  );
  const first = screen
    .getByText("First answer")
    .closest('[data-slot="message"]');
  const second = screen
    .getByText(
      "Understanding the request and selecting authorized capabilities",
    )
    .closest('[data-slot="message"]');
  if (!(first instanceof HTMLElement) || !(second instanceof HTMLElement))
    throw new Error("Expected assistant messages");
  expect(first).not.toBe(second);
  expect(
    within(first).getByRole("button", { name: "Execution progress" }),
  ).toBeVisible();
  expect(within(first).queryByRole("status")).not.toBeInTheDocument();
  expect(
    within(second).getByRole("button", { name: "Execution progress" }),
  ).toBeVisible();
  expect(within(second).getByRole("status")).toHaveTextContent(
    "Planning and calling capabilities",
  );
  expect(
    screen.getByText("Second request").compareDocumentPosition(second) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(
    first.compareDocumentPosition(screen.getByText("Second request")) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();

  view.rerender(
    <I18nextProvider i18n={i18n}>
      <ChatFeed
        messages={messages.map((message) =>
          message.id === "a2"
            ? { ...message, content: "Second answer" }
            : message,
        )}
        busy={false}
        loading={false}
        onSuggestion={() => {}}
      />
    </I18nextProvider>,
  );
  expect(
    screen.getAllByRole("button", { name: "Execution progress" }),
  ).toHaveLength(2);
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
  expect(
    screen.getByText("Second answer").closest('[data-slot="message"]'),
  ).toBe(second);
});

it("shows initial waiting inside the assistant placeholder before SSE starts", async () => {
  await changeLanguage("en", false);
  render(
    <I18nextProvider i18n={i18n}>
      <ChatFeed
        messages={[
          { id: "question", role: "user", content: "New request" },
          { id: "reply", role: "assistant", content: "" },
        ]}
        busy
        loading={false}
        onSuggestion={() => {}}
      />
    </I18nextProvider>,
  );
  const waiting = screen.getByRole("status");
  const assistant = waiting.closest('[data-slot="message"]');
  expect(assistant).toBeInstanceOf(HTMLElement);
  expect(
    screen.getByText("New request").closest('[data-slot="message"]'),
  ).not.toBe(assistant);
});
