import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Sparkles, ShieldCheck } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Empty, EmptyHeader, EmptyDescription } from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorBanner } from "@/features/administration/shared";
import { ApiError } from "@/api/client/request";
import { capabilityLabel, formatMoment } from "@/lib/capability";
import { ApprovalCard } from "./shared";
import { useWorkbench } from "./use-workbench";
import { ChatFeed } from "./chat-feed";
import { PromptInput } from "./prompt-input";

export function Workbench(props: {
  sessionId?: string | null;
  onTurnComplete?: () => void;
  onRunState?: (sessionId: string, title: string, running: boolean) => void;
}) {
  const { t, i18n } = useTranslation("ai");
  const state = useWorkbench(props);
  const {
    snapshot,
    blocked,
    messages,
    busy,
    pending,
    deciding,
    approval,
    error,
  } = state;
  return (
    <div className="grid min-w-0 items-start gap-4 2xl:grid-cols-[minmax(0,1fr)_260px]">
      <Card className="min-w-0 gap-0 py-0">
        <CardHeader className="flex flex-row items-center gap-3 border-b p-4">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-muted">
            <Sparkles className="size-4" />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-sm font-semibold">{t("workbench.title")}</h2>
            <p className="mt-1 text-xs text-muted-foreground">
              {snapshot
                ? t("workbench.runtime", {
                    model: snapshot.model || t("workbench.noModel"),
                    capabilities: snapshot.capabilityCount,
                  })
                : t("workbench.loadingRuntime")}
            </p>
          </div>
          {snapshot && !snapshot.configured && (
            <Badge variant="outline">{t("workbench.notConfigured")}</Badge>
          )}
        </CardHeader>
        <ChatFeed
          messages={messages}
          busy={busy}
          loading={Boolean(props.sessionId && state.history.isPending)}
          onSuggestion={state.setDraft}
        />
        <div className="space-y-3 px-4 empty:hidden">
          {state.history.isError && (
            <ErrorBanner
              error={state.history.error}
              onRetry={() => void state.history.refetch()}
            />
          )}
          {state.status.isError && (
            <ErrorBanner
              error={state.status.error}
              onRetry={() => void state.status.refetch()}
            />
          )}
          {blocked && (
            <p className="ai-empty-note" role="status">
              {t("workbench.disabled")}{" "}
              <Link to="/settings">{t("workbench.openSettings")}</Link>
            </p>
          )}
          {!!error && <ErrorBanner error={error} />}
          {error instanceof ApiError && error.code === "unavailable" && (
            <p className="ai-empty-note" role="status">
              {t("workbench.modelFailed")}
            </p>
          )}
          {state.stopped && (
            <p className="pb-3 text-xs text-muted-foreground" role="status">
              {t("workbench.stopped")}
            </p>
          )}
          {approval && (
            <ApprovalCard
              approval={approval}
              busy={deciding}
              onDecide={state.decide}
            />
          )}
        </div>
        <PromptInput
          draft={state.draft}
          onChange={state.setDraft}
          busy={busy}
          blocked={blocked}
          onSend={(value) => void state.send(value)}
          onStop={state.stop}
        />
      </Card>
      <Card className="min-w-0">
        <CardHeader>
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-sm font-semibold">{t("workbench.pending")}</h2>
            <Badge variant="secondary">{pending.length}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          {state.approvals.isPending ? (
            <Skeleton className="h-16 w-full" />
          ) : state.approvals.isError ? (
            <ErrorBanner
              error={state.approvals.error}
              onRetry={() => void state.approvals.refetch()}
            />
          ) : !pending.length ? (
            <Empty className="border-0 p-4">
              <EmptyHeader>
                <ShieldCheck className="mx-auto size-5 text-muted-foreground" />
                <EmptyDescription>{t("workbench.noPending")}</EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <ul className="space-y-3">
              {pending.map((item) => (
                <li key={item.id} className="space-y-3 rounded-lg border p-3">
                  <div>
                    <p className="text-sm font-medium">
                      {capabilityLabel(t, item.capability)}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {formatMoment(i18n.language, item.requestedAt)}
                    </p>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={deciding}
                    onClick={() => state.setApproval(item)}
                  >
                    {t("workbench.review")}
                  </Button>
                </li>
              ))}
            </ul>
          )}
          <dl className="ai-side-meta">
            <div>
              <dt>{t("workbench.department")}</dt>
              <dd>{state.session.activeDepartmentId}</dd>
            </div>
            <div>
              <dt>{t("workbench.registry")}</dt>
              <dd>{snapshot?.registryVersion ?? "—"}</dd>
            </div>
          </dl>
        </CardContent>
      </Card>
    </div>
  );
}
