import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";
import { changeLanguage, i18n } from "@/i18n";
import { AIModelSettings } from "./ai-model-settings";
import type { ComponentRendererProps } from "./component-registry";

it("uses the same editor for Bailian and GLM and saves the entered API key", async () => {
  await changeLanguage("en", false);
  const user = userEvent.setup();
  const emit = vi.fn().mockResolvedValue(true);
  const entries = [
    { key: "knowledge.embedding.profile", value: '"legacy-embedding"' },
    {
      key: "aiModels.bailian.baseUrl",
      value: '"https://dashscope.aliyuncs.com/api/v1"',
    },
    { key: "aiModels.bailian.type", value: '"embedding"' },
    { key: "aiModels.bailian.model", value: '"qwen3-vl-embedding"' },
    { key: "aiModels.bailian.providerId", value: '"bailian"' },
    { key: "aiModels.bailian.protocol", value: '"dashscope-embedding"' },
    { key: "aiModels.bailian.dimensions", value: "2560" },
    { key: "aiModels.bailian.token", value: "", configured: false },
    { key: "aiModels.glm.providerId", value: '"glm"' },
    { key: "aiModels.glm.type", value: '"text"' },
    { key: "agent.baseUrl", value: '"https://agents.example.test/v1"' },
    { key: "assistant.baseUrl", value: '"https://responses.example.test/v1"' },
    { key: "assistant.model", value: '"glm-example"' },
  ];
  render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <AIModelSettings
          emit={emit}
          data={{ entries }}
          props={{ view: "models" }}
          readOnly={false}
        />
      </I18nextProvider>
    </MemoryRouter>,
  );
  const row = screen.getByRole("row", { name: /qwen3-vl-embedding/ });
  expect(row).toHaveTextContent("Embedding model");
  await user.click(within(row).getByRole("button", { name: "Actions" }));
  await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
  const dialog = screen.getByRole("dialog", {
    name: "Alibaba Cloud Model Studio",
  });
  expect(within(dialog).getByLabelText("API base URL")).toHaveValue(
    "https://dashscope.aliyuncs.com/api/v1",
  );
  expect(within(dialog).getByLabelText("Provider ID")).toHaveValue("bailian");
  expect(within(dialog).getByLabelText("Embedding dimensions")).toHaveValue(
    2560,
  );
  expect(
    within(dialog).getByLabelText("Responses API base URL (optional)"),
  ).toBeDisabled();
  expect(
    within(dialog).getByRole("combobox", { name: "API protocol" }),
  ).toHaveTextContent("DashScope");

  const key = within(dialog).getByLabelText("API key");
  expect(key).toHaveValue("");
  await user.type(key, "test-bailian-key");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  expect(emit).toHaveBeenCalledWith(
    "submit",
    {
      changes: [
        {
          key: "aiModels.bailian.token",
          operation: "set",
          value: '"test-bailian-key"',
        },
      ],
    },
    { onError: expect.any(Function) },
  );
  expect(await screen.findByText("Changes saved")).toBeVisible();
  const glmRow = screen.getByRole("row", { name: /glm-example/ });
  await user.click(within(glmRow).getByRole("button", { name: "Actions" }));
  await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
  const glm = screen.getByRole("dialog", { name: "GLM" });
  expect(within(glm).getByLabelText("API base URL")).toHaveValue(
    "https://agents.example.test/v1",
  );
  expect(
    within(glm).getByLabelText("Responses API base URL (optional)"),
  ).toHaveValue("https://responses.example.test/v1");
  expect(within(glm).getByLabelText("Embedding dimensions")).toBeDisabled();
  expect(within(glm).getByLabelText("API key")).toHaveValue("");
  expect(
    within(glm).getByRole("combobox", { name: "API protocol" }),
  ).toHaveTextContent("Chat Completions");
});

it("shows the active default and excludes embedding models from assignment", async () => {
  await changeLanguage("en", false);
  const user = userEvent.setup();
  const emit = vi.fn().mockResolvedValue(true);
  const entries = [
    ["aiModels.default", '"deepseek"'],
    ["aiModels.deepseek.displayName", '"DeepSeek-V4.1-Flash"'],
    ["aiModels.glm.displayName", '"GLM"'],
    ["aiModels.glm.type", '"text"'],
    ["aiModels.custom-vector.displayName", '"Vector model"'],
    ["aiModels.custom-vector.type", '"embedding"'],
    ["aiModels.custom-chat.displayName", '"Custom chat"'],
  ].map(([key, value]) => ({ key, value, configured: true }));
  render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <AIModelSettings
          emit={emit}
          data={{ entries }}
          props={{ view: "default" }}
          readOnly={false}
        />
      </I18nextProvider>
    </MemoryRouter>,
  );
  expect(screen.getByText(/Active model.*DeepSeek/)).toBeVisible();
  screen.getByRole("combobox", { name: "Default model" }).focus();
  await user.keyboard("{ArrowDown}");
  const glm = await screen.findByRole("option", { name: "GLM" });
  expect(screen.queryByRole("option", { name: "Vector model" })).toBeNull();
  expect(screen.getByRole("option", { name: "Custom chat" })).toBeVisible();
  await user.click(glm);
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(emit).toHaveBeenCalledWith(
    "submit",
    {
      changes: [{ key: "aiModels.default", operation: "set", value: '"glm"' }],
    },
    undefined,
  );
});

it("assigns a custom chat model and a separate embedding model to knowledge", async () => {
  await changeLanguage("en", false);
  const user = userEvent.setup();
  const emit = vi.fn().mockResolvedValue(true);
  const entries = [
    ["aiModels.route.knowledge", '"inherit"'],
    ["aiModels.custom-chat.displayName", '"Custom chat"'],
    ["aiModels.custom-chat.type", '"multimodal"'],
    ["aiModels.bailian.type", '"embedding"'],
    ["knowledge.embedding.profile", '"legacy-embedding"'],
  ].map(([key, value]) => ({ key, value }));
  const embeddingModels = [
    { id: "legacy-embedding", label: "Legacy embedding" },
    { id: "bailian", label: "qwen3-vl-embedding" },
  ];
  render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <AIModelSettings
          emit={emit}
          data={{ entries, embeddingModels }}
          props={{ view: "routes" }}
          readOnly={false}
        />
      </I18nextProvider>
    </MemoryRouter>,
  );
  screen.getByRole("combobox", { name: "Knowledge Q&A" }).focus();
  await user.keyboard("{ArrowDown}");
  expect(
    screen.queryByRole("option", { name: "qwen3-vl-embedding" }),
  ).toBeNull();
  await user.click(await screen.findByRole("option", { name: "Custom chat" }));
  screen.getByRole("combobox", { name: "Knowledge embedding model" }).focus();
  await user.keyboard("{ArrowDown}");
  expect(screen.queryByRole("option", { name: "Custom chat" })).toBeNull();
  await user.click(
    await screen.findByRole("option", { name: "qwen3-vl-embedding" }),
  );
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(emit).toHaveBeenCalledWith(
    "submit",
    {
      changes: [
        {
          key: "aiModels.route.knowledge",
          operation: "set",
          value: '"custom-chat"',
        },
        {
          key: "knowledge.embedding.profile",
          operation: "set",
          value: '"bailian"',
        },
      ],
    },
    undefined,
  );
});

it("saves a new custom model without modifying the legacy model slot", async () => {
  await changeLanguage("en", false);
  const user = userEvent.setup();
  const emit = vi
    .fn<ComponentRendererProps["emit"]>()
    .mockImplementationOnce(async (_event, _payload, options) => {
      options?.onError?.("invalid");
      return false;
    })
    .mockResolvedValueOnce(true);
  const entries = [
    {
      key: "aiModels.custom.displayName",
      value: '"Existing custom model"',
      overridden: true,
    },
  ];
  const { rerender } = render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <AIModelSettings
          emit={emit}
          data={{ entries }}
          props={{ view: "models" }}
          readOnly={false}
        />
      </I18nextProvider>
    </MemoryRouter>,
  );
  expect(screen.getByText("Existing custom model")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Add custom model" }));
  expect(
    screen.getByRole("dialog", { name: "Add custom model" }),
  ).toBeVisible();
  await user.type(screen.getByLabelText("Provider ID"), "example");
  await user.type(screen.getByLabelText("Display name"), "New model");
  await user.type(
    screen.getByLabelText("API base URL"),
    "https://example.com/v1",
  );
  await user.type(
    screen.getByLabelText("Model name (API request value)"),
    "example-chat",
  );
  await user.type(screen.getByLabelText("API key"), "fake-key");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(screen.getByRole("dialog")).toBeVisible();
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Check the entered values",
  );
  expect(screen.getByLabelText("Display name")).toHaveValue("New model");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByText("Changes saved")).toBeVisible();
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(emit).toHaveBeenCalledTimes(2);
  const firstPayload = emit.mock.calls[0][1] as { changes: { key: string }[] };
  const profile = firstPayload.changes[0].key.split(".")[1];
  expect(profile).toMatch(/^custom-[a-z0-9-]+$/);
  expect(emit).toHaveBeenLastCalledWith(
    "submit",
    {
      changes: Object.entries({
        displayName: "New model",
        providerId: "example",
        type: "text",
        protocol: "chat-completions",
        baseUrl: "https://example.com/v1",
        model: "example-chat",
        token: "fake-key",
      }).map(([field, value]) => ({
        key: `aiModels.${profile}.${field}`,
        operation: "set",
        value: JSON.stringify(value),
      })),
    },
    { onError: expect.any(Function) },
  );
  rerender(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <AIModelSettings
          emit={emit}
          data={{ entries }}
          props={{ view: "models" }}
          readOnly
        />
      </I18nextProvider>
    </MemoryRouter>,
  );
  expect(screen.queryByRole("button", { name: "Add custom model" })).toBeNull();
});
