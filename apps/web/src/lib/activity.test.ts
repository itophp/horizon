import { afterEach, describe, expect, it, vi } from "vitest";
import { ActivityCollector, activityPage } from "./activity";
import { collectActivity } from "@/lib/capabilities";

vi.mock("@/lib/capabilities", () => ({
  collectActivity: vi.fn().mockResolvedValue({ data: { accepted: 1 } }),
}));

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe("activity collection", () => {
  it("maps only registered page names and excludes resource IDs and queries", () => {
    expect(activityPage("/drive/123?token=secret")).toBe("drive");
    expect(activityPage("/files/123")).toBe("attachments");
    expect(activityPage("/unregistered/secret")).toBeUndefined();
  });

  it("batches allowlisted observations and stops after session cleanup", async () => {
    vi.useFakeTimers();
    const collector = new ActivityCollector(1);
    collector.record("drive", "page_view");
    collector.record("unregistered", "interaction");
    await vi.advanceTimersByTimeAsync(5000);
    expect(collectActivity).toHaveBeenCalledOnce();
    const [events] = vi.mocked(collectActivity).mock.calls[0];
    expect(events).toHaveLength(1);
    expect(events[0]).toMatchObject({
      kind: "page_view",
      page: "drive",
      action: "view",
    });
    collector.stop();
    collector.record("drive", "page_view");
    await vi.advanceTimersByTimeAsync(5000);
    expect(collectActivity).toHaveBeenCalledOnce();
  });
});
