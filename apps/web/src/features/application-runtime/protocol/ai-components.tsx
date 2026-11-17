import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, LoaderCircle, Sparkles } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  runtimeAIAnalysisPanelPropsSchema,
  runtimeAIResultSchema,
  type RuntimeAIAnalysisSpec,
  type RuntimeAIResult,
} from "@/api/generated/runtime";
import type { ComponentRendererProps } from "./component-registry";

type AIEmitter = ComponentRendererProps["emit"];
export const quotaExhausted = "ai_quota_exhausted";
export const modelImageUnsupported = "ai_model_image_unsupported";
export const attachmentInvalid = "ai_attachment_invalid";
type AnalysisResult = RuntimeAIResult & { quotaExhausted?: boolean };

type RecordData = Record<string, unknown>;

// localNowRFC3339 is the device clock with its UTC offset. The AI form
// assistant uses it only to resolve relative dates ("tomorrow at 09:00"); the
// suggested instant is revalidated by the business capability on save.
export function localNowRFC3339(date = new Date()): string {
  const offset = -date.getTimezoneOffset();
  const pad = (value: number, size = 2) =>
    String(Math.abs(value)).padStart(size, "0");
  const zone = `${offset >= 0 ? "+" : "-"}${pad(Math.trunc(offset / 60))}:${pad(offset % 60)}`;
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}${zone}`;
}

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
  if (
    errorCode === quotaExhausted ||
    errorCode === attachmentInvalid ||
    errorCode === modelImageUnsupported ||
    errorCode === "ai_pdf_unavailable"
  )
    throw new Error(errorCode);
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
    return <span className="ai-result-status">{t("ai.idle")}</span>;
  if (value.status === "running")
    return (
      <span className="ai-result-status" role="status">
        <LoaderCircle
          className="motion-safe:animate-spin"
          size={14}
          aria-hidden
        />
        {t("ai.running")}
      </span>
    );
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
    <div className="ai-result-details">
      <p className="ai-result-summary">{value.summary}</p>
      {value.evidence?.length ? (
        <ul className="list-disc pl-5">
          {value.evidence.map((item, index) => (
            <li key={index}>{item}</li>
          ))}
        </ul>
      ) : null}
      {value.coverage && (
        <p className="ai-result-coverage">
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
        <details className="ai-result-expand">
          <summary>
            <Badge variant="outline" className="ai-result-badge">
              {label}
            </Badge>
            <ChevronDown size={14} aria-hidden />
          </summary>
          {details}
        </details>
      ) : (
        <>
          {label && (
            <Badge variant="outline" className="ai-result-badge">
              {label}
            </Badge>
          )}
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
      <h2>
        <span className="ai-panel-icon">
          <Sparkles size={18} aria-hidden />
        </span>
        {t("ai.analysis")}
      </h2>
      <div className="flex flex-wrap gap-2">
        {scenes.map((scene) => (
          <Button
            key={scene.id}
            type="button"
            size="sm"
            variant={scene.id === active.id ? "secondary" : "outline"}
            className="ai-action"
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
            {t(scene.labelKey)}
          </Button>
        ))}
      </div>
      <div className="ai-analysis-output">
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
      </div>
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
