import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import type { AIStreamEvent } from "@/api/generated/models";
import {
  Trace,
  TraceLine,
  TraceTitle,
  TraceIcon,
  TraceCollapsible,
  TraceCollapsibleTrigger,
  TraceCollapsibleContent,
} from "@/components/ui/trace";
import { capabilityLabel } from "@/lib/capability";

// Events belong to one assistant turn, including after a later turn starts.
export function RunProgress({ events }: { events: AIStreamEvent[] }) {
  const { t } = useTranslation("ai");
  const visible = events.filter(
    (event) =>
      event.kind === "planning" ||
      event.kind === "tool_start" ||
      event.kind === "tool_end",
  );
  if (!visible.length) return null;
  return (
    <Trace>
      <TraceCollapsible defaultOpen>
        <TraceCollapsibleTrigger>
          <TraceLine>
            <TraceTitle>{t("workbench.progress")}</TraceTitle>
          </TraceLine>
        </TraceCollapsibleTrigger>
        <TraceCollapsibleContent>
          <ol className="space-y-2">
            {visible.map((event) => (
              <li key={event.sequence}>
                <TraceLine
                  status={event.status === "failed" ? "error" : undefined}
                >
                  <TraceIcon>
                    <Sparkles />
                  </TraceIcon>
                  <TraceTitle className="whitespace-normal">
                    {event.capability
                      ? t("workbench.flow", {
                          capability: capabilityLabel(t, event.capability),
                          status: t(
                            event.status === "running"
                              ? "runStatus.running"
                              : `capability:invocations.status.${event.status}`,
                            { defaultValue: event.status },
                          ),
                        })
                      : t("workbench.planning")}
                  </TraceTitle>
                </TraceLine>
              </li>
            ))}
          </ol>
        </TraceCollapsibleContent>
      </TraceCollapsible>
    </Trace>
  );
}
