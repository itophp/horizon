import { useEffect, useRef } from "react";
import { useLocation } from "react-router";
import { ActivityCollector, activityPage } from "@/lib/activity";
import { useConfiguration } from "@/features/governance/runtime";

export function ActivityCapture({ enabled }: { enabled: boolean }) {
  const { pathname } = useLocation();
  const config = useConfiguration();
  const samplePercent = Number(config.value("memory.activitySamplePercent", 0));
  const collector = useRef<ActivityCollector | null>(null);

  useEffect(() => {
    if (!enabled) return;
    const active = new ActivityCollector(
      Math.min(100, Math.max(0, samplePercent)) / 100,
    );
    collector.current = active;
    return () => {
      collector.current = null;
      active.stop();
    };
  }, [enabled, samplePercent]);
  useEffect(() => {
    const page = activityPage(pathname);
    if (enabled && page) collector.current?.record(page, "page_view");
  }, [enabled, pathname, samplePercent]);
  useEffect(() => {
    if (!enabled) return;
    const onActivity = (event: Event) => {
      if (event instanceof CustomEvent && typeof event.detail === "string")
        collector.current?.record(event.detail, "interaction");
    };
    window.addEventListener("memory:activity", onActivity);
    return () => window.removeEventListener("memory:activity", onActivity);
  }, [enabled]);
  return null;
}
