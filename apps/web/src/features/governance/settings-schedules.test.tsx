import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import type { ChangeRecord, SettingSchedule } from "@/api/generated/models";
import { ChangeHistory } from "./shared";
import { localScheduleTime, scheduleInstant } from "./schedule-time";
const mocks = vi.hoisted(() => ({
  jobs: vi.fn(async () => ({ status: 200, data: [] as SettingSchedule[] })),
  schedule: vi.fn(async () => ({ status: 201, data: { id: 10 } })),
  cancel: vi.fn(async () => ({ status: 204 })),
  reauth: vi.fn(async () => ({
    status: 200,
    data: { proof: "one-time-schedule-proof" },
  })),
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "t",
    activeDepartmentId: 1,
    account: { id: 1 },
    permissions: ["settings:read", "settings:publish"],
  }),
}));
vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  settingsSchedules: mocks.jobs,
  settingsSchedule: mocks.schedule,
  settingsCancelSchedule: mocks.cancel,
  settingsReauthenticate: mocks.reauth,
}));
const draft: ChangeRecord = {
  id: 7,
  domain: "settings",
  target: { kind: "workspace", id: 0 },
  baseRevision: 2,
  revision: 0,
  status: "draft",
  reason: "Change system name",
  actorId: 1,
  createdAt: "2026-10-29T00:00:00Z",
  sensitive: true,
  changes: [{ key: "branding.name", operation: "set", value: '"New name"' }],
};
function show() {
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <ChangeHistory
          domain="settings"
          drafts={[draft]}
          history={[]}
          revision={2}
        />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}
beforeEach(async () => {
  await changeLanguage("en");
  mocks.jobs.mockResolvedValue({ status: 200, data: [] });
  mocks.schedule.mockClear();
  mocks.cancel.mockClear();
  mocks.reauth.mockClear();
});
it("schedules a sensitive draft with an operation-bound proof and UTC time", async () => {
  const user = userEvent.setup();
  show();
  await user.click(
    screen.getByRole("button", { name: "Schedule publication" }),
  );
  const dialog = screen.getByRole("dialog", { name: "Schedule publication" });
  const local = (
    within(dialog).getByLabelText("Publish at") as HTMLInputElement
  ).value;
  await user.type(
    within(dialog).getByLabelText("Current password"),
    "Test password",
  );
  await user.click(within(dialog).getByRole("button", { name: "Confirm" }));
  await waitFor(() =>
    expect(mocks.schedule).toHaveBeenCalledWith(7, {
      baseRevision: 2,
      proof: "one-time-schedule-proof",
      scheduledAt: new Date(local).toISOString(),
    }),
  );
  expect(mocks.reauth).toHaveBeenCalledWith({
    password: "Test password",
    operation: "settings.schedule",
    resourceId: 7,
    revision: 2,
  });
});
it("shows pending schedules, prevents immediate publication and cancels through the SDK", async () => {
  mocks.jobs.mockResolvedValue({
    status: 200,
    data: [
      {
        id: 10,
        draftId: 7,
        actorId: 1,
        departmentId: 1,
        baseRevision: 2,
        reason: draft.reason,
        scheduledAt: new Date(Date.now() + 3600000).toISOString(),
        createdAt: draft.createdAt,
        finishedAt: "",
        status: "pending",
        errorCode: "",
        versionId: 0,
      },
    ],
  });
  const user = userEvent.setup();
  show();
  await screen.findByRole("button", { name: "Scheduled" });
  expect(screen.getByRole("button", { name: "Publish" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Scheduled" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Cancel schedule" }));
  await waitFor(() => expect(mocks.cancel).toHaveBeenCalledWith(10));
});
it("validates future local times and converts them to an explicit UTC instant", () => {
  const now = Date.now();
  const future = localScheduleTime(new Date(now + 3600000));
  expect(scheduleInstant(future, now)).toBe(new Date(future).toISOString());
  for (const value of [
    "not a date",
    "2026-02-31T09:30",
    localScheduleTime(new Date(now - 3600000)),
    localScheduleTime(new Date(now + 366 * 86400000)),
  ])
    expect(scheduleInstant(value, now)).toBeUndefined();
});
