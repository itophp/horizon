import { useTranslation } from "react-i18next";
import { ArrowDown, Loader2, Sparkles } from "lucide-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Button } from "@/components/ui/button";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import {
  Message,
  MessageContent,
  MessageFooter,
} from "@/components/ui/message";
import {
  MessageScrollerProvider,
  MessageScroller,
  MessageScrollerViewport,
  MessageScrollerContent,
  MessageScrollerItem,
  MessageScrollerButton,
} from "@/components/ui/message-scroller";
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { formatMoment } from "@/lib/capability";
import { RunStatusBadge, RunTrace } from "./shared";
import type { Turn } from "./use-workbench";
import { RunProgress } from "./run-progress";

export function ChatFeed({
  messages,
  busy,
  loading,
  onSuggestion,
}: {
  messages: Turn[];
  busy: boolean;
  loading: boolean;
  onSuggestion: (value: string) => void;
}) {
  const { t, i18n } = useTranslation("ai");
  return (
    <MessageScrollerProvider autoScroll>
      <MessageScroller className="h-[min(62vh,720px)] min-h-80">
        <MessageScrollerViewport
          role="log"
          aria-label={t("workbench.conversation")}
          aria-live="polite"
          aria-busy={loading}
        >
          <MessageScrollerContent className="p-4 sm:p-6">
            {loading ? (
              <div
                className="space-y-5"
                aria-label={t("workbench.loadingHistory")}
              >
                <Skeleton className="ml-auto h-12 w-2/3" />
                <Skeleton className="h-24 w-4/5" />
                <span className="sr-only">{t("workbench.loadingHistory")}</span>
              </div>
            ) : (
              !messages.length && (
                <Empty className="min-h-64 border-0">
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      <Sparkles />
                    </EmptyMedia>
                    <EmptyTitle>{t("workbench.welcome")}</EmptyTitle>
                    <EmptyDescription>{t("workbench.intro")}</EmptyDescription>
                  </EmptyHeader>
                  <EmptyContent className="max-w-lg">
                    {(["one", "two", "three"] as const).map((key) => (
                      <Button
                        key={key}
                        type="button"
                        variant="outline"
                        className="h-auto w-full justify-start py-2 text-left text-xs whitespace-normal"
                        onClick={() =>
                          onSuggestion(t(`workbench.suggestion.${key}`))
                        }
                      >
                        {t(`workbench.suggestion.${key}`)}
                      </Button>
                    ))}
                  </EmptyContent>
                </Empty>
              )
            )}
            {messages.map((turn, index) => (
              <MessageScrollerItem
                key={turn.id}
                messageId={turn.id}
                scrollAnchor={turn.role === "user"}
              >
                <Message align={turn.role === "user" ? "end" : "start"}>
                  <MessageContent>
                    <span className="sr-only">
                      {t(
                        turn.role === "user"
                          ? "workbench.you"
                          : "workbench.assistant",
                      )}
                    </span>
                    {turn.role === "assistant" &&
                      (turn.run &&
                      turn.run.status !== "running" &&
                      turn.run.invocations.length > 0 ? (
                        <RunTrace run={turn.run} />
                      ) : (
                        <RunProgress events={turn.progress ?? []} />
                      ))}
                    {busy &&
                      index === messages.length - 1 &&
                      turn.role === "assistant" &&
                      !turn.content && (
                        <div
                          className="flex items-center gap-2 text-sm text-muted-foreground"
                          role="status"
                        >
                          <Loader2
                            className="size-4 animate-spin motion-reduce:animate-none"
                            aria-hidden="true"
                          />
                          {t("workbench.thinking")}
                        </div>
                      )}
                    {turn.content && (
                      <Bubble
                        variant={turn.role === "user" ? "muted" : "ghost"}
                        align={turn.role === "user" ? "end" : "start"}
                        className={
                          turn.role === "assistant" ? "w-full" : undefined
                        }
                      >
                        <BubbleContent>
                          {turn.role === "user" ? (
                            <p className="whitespace-pre-wrap">
                              {turn.content}
                            </p>
                          ) : (
                            <div className="ai-markdown">
                              <Markdown remarkPlugins={[remarkGfm]}>
                                {turn.content}
                              </Markdown>
                            </div>
                          )}
                        </BubbleContent>
                      </Bubble>
                    )}
                    {turn.run && !(busy && index === messages.length - 1) && (
                      <>
                        <MessageFooter className="flex-wrap gap-3">
                          <RunStatusBadge status={turn.run.status} />
                          <span>
                            {formatMoment(i18n.language, turn.run.createdAt)}
                          </span>
                        </MessageFooter>
                      </>
                    )}
                  </MessageContent>
                </Message>
              </MessageScrollerItem>
            ))}
          </MessageScrollerContent>
        </MessageScrollerViewport>
        <MessageScrollerButton aria-label={t("workbench.latest")}>
          <ArrowDown />
          <span className="sr-only">{t("workbench.latest")}</span>
        </MessageScrollerButton>
      </MessageScroller>
    </MessageScrollerProvider>
  );
}
