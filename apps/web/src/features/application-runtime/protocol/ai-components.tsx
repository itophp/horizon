import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  runtimeAIAnalysisPanelPropsSchema,
  runtimeAIResultSchema,
  runtimeAISuggestionsSchema,
  type RuntimeAIAnalysisSpec,
  type RuntimeAIFormAssistant,
  type RuntimeAIResult,
  type RuntimeAIFieldSuggestion,
} from "@/api/generated/runtime";
import type { ComponentRendererProps } from "./component-registry";

type AIEmitter = ComponentRendererProps["emit"];
const quotaExhausted = "ai_quota_exhausted";
type AnalysisResult = RuntimeAIResult & { quotaExhausted?: boolean };

type RecordData = Record<string, unknown>;

export async function requestAI(
  emit: AIEmitter,
  event: string,
  input: RecordData,
) {
  let result: unknown;
  let errorCode: string | undefined;
  const succeeded = await emit(event, input, {
    ephemeral: true,
    onError: (code) => {
      errorCode = code;
    },
    onAIResult: (data) => {
      result = data;
    },
  });
  if (errorCode === quotaExhausted) throw new Error(quotaExhausted);
  return succeeded ? result : undefined;
}

export function AIResult({
  value,
  compact = false,
  retry,
}: {
  value?: AnalysisResult;
  compact?: boolean;
  retry?: () => void;
}) {
  const { t } = useTranslation("runtime");
  if (!value || value.status === "idle")
    return <span className="text-muted-foreground">{t("ai.idle")}</span>;
  if (value.status === "running")
    return <span role="status">{t("ai.running")}</span>;
  if (value.status === "failed")
    return (
      <span className="inline-flex items-center gap-2">
        <span role="status">
          {t(value.quotaExhausted ? "ai.quotaExhausted" : "ai.failed")}
        </span>
        {retry && !value.quotaExhausted && (
          <Button type="button" variant="outline" size="sm" onClick={retry}>
            {t("ai.retry")}
          </Button>
        )}
      </span>
    );
  const labels = {
    high: t("ai.high"),
    medium: t("ai.medium"),
    low: t("ai.low"),
    insufficient: t("ai.insufficient"),
  };
  const label =
    value.label && Object.hasOwn(labels, value.label)
      ? labels[value.label as keyof typeof labels]
      : undefined;
  const details = (
    <div className="space-y-1 whitespace-normal">
      <p>{value.summary}</p>
      {value.evidence?.length ? (
        <ul className="list-disc pl-5">
          {value.evidence.map((item, index) => (
            <li key={index}>{item}</li>
          ))}
        </ul>
      ) : null}
      {value.coverage && (
        <p className="text-muted-foreground">
          {t("ai.coverage", { ...value.coverage })}
        </p>
      )}
    </div>
  );
  return (
    <div className="space-y-1">
      {value.status === "stale" && (
        <span className="text-muted-foreground">{t("ai.stale")}</span>
      )}
      {compact && label ? (
        <details>
          <summary className="cursor-pointer">
            <Badge variant="outline">{label}</Badge>
          </summary>
          {details}
        </details>
      ) : (
        <>
          {label && <Badge variant="outline">{label}</Badge>}
          {details}
        </>
      )}
    </div>
  );
}

export function AIResultComponent({ data, props }: ComponentRendererProps) {
  const result = runtimeAIResultSchema.safeParse(data);
  return <AIResult value={result.data} compact={props.variant === "compact"} />;
}

type StoredAnalysis = { value: AnalysisResult; snapshot: string };

export function useAIAnalysis(emit: AIEmitter, componentId?: string) {
  const [results, setResults] = useState<Record<string, StoredAnalysis>>({});
  const active = useRef(new Set<string>());
  const mounted = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  function key(sceneId: string, recordId: number) {
    return JSON.stringify([sceneId, recordId]);
  }
  function value(sceneId: string, recordId: number, snapshot: string) {
    const result = results[key(sceneId, recordId)];
    if (result?.value.status === "succeeded" && result.snapshot !== snapshot)
      return { ...result.value, status: "stale" };
    return result?.value;
  }
  async function run(
    sceneId: string,
    spec: RuntimeAIAnalysisSpec,
    recordId: number,
    snapshot: string,
  ) {
    const id = key(sceneId, recordId);
    if (!componentId || !mounted.current || active.current.has(id)) return;
    active.current.add(id);
    setResults((current) => ({
      ...current,
      [id]: { value: { status: "running" }, snapshot },
    }));
    let result: AnalysisResult = { status: "failed" };
    try {
      const response = await requestAI(emit, spec.action, {
        componentId,
        sceneId,
        recordId,
      });
      const parsed = runtimeAIResultSchema.safeParse(response);
      if (parsed.success && parsed.data.status === "succeeded")
        result = parsed.data;
    } catch (error) {
      result = {
        status: "failed",
        quotaExhausted:
          error instanceof Error && error.message === quotaExhausted,
      };
    } finally {
      active.current.delete(id);
      if (mounted.current)
        setResults((current) => ({
          ...current,
          [id]: { value: result, snapshot },
        }));
    }
  }
  return { run, value };
}

export function aiSnapshot(record: RecordData, spec: RuntimeAIAnalysisSpec) {
  return JSON.stringify(spec.input[0]?.fields.map((field) => record[field]));
}

export function AIAnalysisPanel({
  data,
  props,
  readOnly,
  componentId,
  events,
  dataMap,
  dataRefreshing,
  emit,
}: ComponentRendererProps) {
  const { t } = useTranslation(["runtime", "projects"]);
  const analysis = useAIAnalysis(emit, componentId);
  const [selected, setSelected] = useState("");
  const parsed = runtimeAIAnalysisPanelPropsSchema.safeParse(props);
  if (!parsed.success || !data || typeof data !== "object") return null;
  const record = data as RecordData;
  const recordId = Number(record.id);
  const scenes = parsed.data.scenarios.filter(
    (scene) => readOnly || events?.includes(scene.analysis.action),
  );
  if (!scenes.length) return null;
  const active = scenes.find((scene) => scene.id === selected) ?? scenes[0];
  const snapshot = (spec: RuntimeAIAnalysisSpec) =>
    JSON.stringify([
      aiSnapshot(record, spec),
      ...spec.input.slice(1).map((input) => {
        const source = dataMap?.[input.source.slice(1)] as
          { items?: RecordData[]; total?: number } | undefined;
        return [
          source?.total,
          source?.items?.map((row) => input.fields.map((field) => row[field])),
        ];
      }),
    ]);
  const value = analysis.value(active.id, recordId, snapshot(active.analysis));
  return (
    <section className="admin-panel ai-analysis-panel">
      <h2>{t("ai.analysis")}</h2>
      <div className="flex flex-wrap gap-2">
        {scenes.map((scene) => (
          <Button
            key={scene.id}
            type="button"
            size="sm"
            variant={scene.id === active.id ? "secondary" : "outline"}
            disabled={
              readOnly ||
              dataRefreshing ||
              !Number.isSafeInteger(recordId) ||
              recordId < 1
            }
            aria-pressed={scene.id === active.id}
            onClick={() => {
              setSelected(scene.id);
              if (!analysis.value(scene.id, recordId, snapshot(scene.analysis)))
                void analysis.run(
                  scene.id,
                  scene.analysis,
                  recordId,
                  snapshot(scene.analysis),
                );
            }}
          >
            <Sparkles aria-hidden="true" size={14} />
            {t(scene.labelKey)}
          </Button>
        ))}
      </div>
      <AIResult
        value={value}
        retry={
          readOnly || dataRefreshing
            ? undefined
            : () =>
                void analysis.run(
                  active.id,
                  active.analysis,
                  recordId,
                  snapshot(active.analysis),
                )
        }
      />
      {!readOnly &&
        !dataRefreshing &&
        (value?.status === "succeeded" || value?.status === "stale") && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() =>
              void analysis.run(
                active.id,
                active.analysis,
                recordId,
                snapshot(active.analysis),
              )
            }
          >
            {t("ai.rerun")}
          </Button>
        )}
    </section>
  );
}

export function AIFormAssistant({
  spec,
  request,
  current,
  label,
  canApply,
  apply,
  disabled,
}: {
  spec: RuntimeAIFormAssistant;
  request: () => Promise<unknown>;
  current: (field: string) => unknown;
  label: (field: string) => string;
  canApply: (suggestion: RuntimeAIFieldSuggestion) => boolean;
  apply: (suggestion: RuntimeAIFieldSuggestion) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation(["runtime", "projects"]);
  const [state, setState] = useState<
    "idle" | "running" | "failed" | "quotaExhausted" | "succeeded"
  >("idle");
  const [items, setItems] = useState<RuntimeAIFieldSuggestion[]>([]);
  async function generate() {
    setState("running");
    setItems([]);
    try {
      const parsed = runtimeAISuggestionsSchema.safeParse(await request());
      if (!parsed.success) {
        setState("failed");
        return;
      }
      setItems(
        parsed.data.suggestions.filter((item) =>
          spec.targetFields.includes(item.field),
        ),
      );
      setState("succeeded");
    } catch (error) {
      setState(
        error instanceof Error && error.message === quotaExhausted
          ? "quotaExhausted"
          : "failed",
      );
    }
  }
  return (
    <div className="space-y-2">
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled || state === "running"}
        onClick={() => void generate()}
      >
        <Sparkles aria-hidden="true" size={14} />
        {t(spec.labelKey)}
      </Button>
      {state === "running" && <p role="status">{t("ai.running")}</p>}
      {state === "failed" && <p role="status">{t("ai.failed")}</p>}
      {state === "quotaExhausted" && (
        <p role="status">{t("ai.quotaExhausted")}</p>
      )}
      {state === "succeeded" && !items.length && (
        <p role="status">{t("ai.noSuggestions")}</p>
      )}
      {items.map((item) => (
        <div key={item.field} className="space-y-1 rounded-md border p-3">
          <p className="font-medium">{label(item.field)}</p>
          <p className="whitespace-pre-wrap">
            {t("ai.current", { value: String(current(item.field) ?? "") })}
          </p>
          <p className="whitespace-pre-wrap">
            {t("ai.suggestion", { value: item.value })}
          </p>
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={disabled || !canApply(item)}
            onClick={() => {
              apply(item);
              setItems((previous) =>
                previous.filter((entry) => entry.field !== item.field),
              );
              setState("idle");
            }}
          >
            {t("ai.apply")}
          </Button>
        </div>
      ))}
    </div>
  );
}
