import { collectActivity, type ActivityObservation } from "@/lib/capabilities";

// Only stable page names are sent; URLs, path parameters and query strings are
// never included in observations.
const pages = new Set([
  "analytics",
  "ai",
  "articles",
  "drive",
  "knowledge",
  "projects",
  "orchestration",
  "accounts",
  "organizations",
  "departments",
  "roles",
  "settings",
  "preferences",
  "dictionaries",
  "attachments",
  "notifications",
  "permissions",
  "temporary-access",
  "capabilities",
  "mcp",
  "webmcp",
]);

export function activityPage(pathname: string): string | undefined {
  const first = pathname.split(/[?#]/, 1)[0].split("/")[1];
  const page = first === "files" ? "attachments" : first;
  return pages.has(page) ? page : undefined;
}

export function recordNavigation(pathname: string) {
  const page = activityPage(pathname);
  if (page)
    window.dispatchEvent(new CustomEvent("memory:activity", { detail: page }));
}

export class ActivityCollector {
  private queue: ActivityObservation[] = [];
  private timer: ReturnType<typeof setTimeout> | undefined;
  private stopped = false;
  private flushing = false;

  constructor(private readonly sampleRate = 0.1) {}

  record(page: string, kind: "page_view" | "interaction") {
    if (
      this.stopped ||
      !pages.has(page) ||
      !globalThis.crypto?.randomUUID ||
      Math.random() >= this.sampleRate
    )
      return;
    if (this.queue.length >= 100) {
      console.warn("Activity collection queue is full");
      return;
    }
    this.queue.push({
      id: crypto.randomUUID(),
      kind,
      page,
      action: kind === "page_view" ? "view" : "open",
      occurredAt: new Date().toISOString(),
    });
    this.schedule(5000);
  }

  private schedule(delay: number) {
    if (this.timer || this.stopped || this.flushing) return;
    this.timer = setTimeout(() => {
      this.timer = undefined;
      void this.flush();
    }, delay);
  }

  private async flush() {
    if (this.stopped || this.flushing || this.queue.length === 0) return;
    const batch = this.queue.splice(0, 25);
    const key = crypto.randomUUID();
    this.flushing = true;
    let delivered = false;
    for (let attempt = 0; attempt < 3 && !this.stopped; attempt++) {
      try {
        await collectActivity(batch, key);
        delivered = true;
        break;
      } catch {
        if (attempt < 2 && !this.stopped)
          await new Promise((resolve) =>
            setTimeout(resolve, 1000 * (attempt + 1)),
          );
      }
    }
    if (!delivered && !this.stopped)
      console.warn("Activity collection failed after retries");
    this.flushing = false;
    if (this.queue.length > 0) this.schedule(5000);
  }

  stop() {
    this.stopped = true;
    if (this.timer) clearTimeout(this.timer);
    this.queue = [];
  }
}
