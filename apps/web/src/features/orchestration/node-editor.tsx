import { useForm, useWatch } from "react-hook-form";
import { useId } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { capabilityDefinitions } from "@/api/generated/baseline";
import { useCapabilityKey } from "@/features/capabilities/shared";
import { EditorDialog, ErrorBanner } from "@/features/administration/shared";
import type { Diagram } from "./graph";
import {
  ancestors,
  DraftError,
  type AddRequest,
  type DraftStep,
} from "./draft";

const schema = z.object({
  id: z.string().regex(/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/, "editor.idInvalid"),
  kind: z.enum(["step", "agent", "poll"]),
  capability: z.string(),
  constants: z.string(),
  input: z.string(),
  instruction: z.string(),
  tools: z.array(z.string()),
  maxIterations: z
    .number({ error: "editor.budget" })
    .int()
    .min(1, "editor.budget")
    .max(20, "editor.budget"),
  output: z.string(),
  doneField: z.string(),
  maxSteps: z
    .number({ error: "editor.budget" })
    .int()
    .min(1, "editor.budget")
    .max(100, "editor.budget"),
  from: z.string(),
  field: z.string(),
  equals: z.string(),
  originalCase: z.string(),
});
const object = z.record(z.string(), z.unknown());
const mappings = z.record(
  z.string(),
  z.object({ from: z.string().min(1), field: z.string().min(1) }).strict(),
);

export function NodeEditor({
  diagram,
  request,
  onClose,
  onAdd,
}: {
  diagram: Diagram;
  request: AddRequest;
  onClose: () => void;
  onAdd: (step: DraftStep, originalCase?: unknown) => void;
}) {
  const { t } = useTranslation("orchestration");
  const hintId = useId();
  const queryKey = useCapabilityKey("definitions");
  const catalog = useQuery({
    queryKey,
    queryFn: () => capabilityDefinitions(),
    retry: false,
  });
  const contracts =
    catalog.data?.status === 200
      ? catalog.data.data.filter((item) => item.layer === "atomic")
      : [];
  const anchor =
    request.kind === "sibling"
      ? diagram.nodes.find((node) => node.id === request.nodeId)
      : undefined;
  const sources = anchor ? ancestors(diagram, anchor.id) : [];
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      id: "",
      kind: "step",
      capability: "",
      constants: "{}",
      input: "{}",
      instruction: "",
      tools: [],
      maxIterations: 5,
      output: "",
      doneField: "",
      maxSteps: 10,
      from: anchor?.when?.from ?? sources[0] ?? "",
      field: anchor?.when?.field ?? "",
      equals: "",
      originalCase: "",
    },
  });
  const kind = useWatch({ control: form.control, name: "kind" });
  const available =
    kind === "poll" ? contracts.filter((item) => item.readOnly) : contracts;
  const submit = form.handleSubmit((values) => {
    try {
      const constants = object.parse(JSON.parse(values.constants));
      const input = mappings.parse(JSON.parse(values.input));
      const step: DraftStep = { id: values.id, constants, input };
      if (kind === "agent") {
        if (
          !values.instruction.trim() ||
          !values.tools.length ||
          values.tools.some(
            (tool) =>
              !contracts.some(
                (item) => `${item.name}@${item.version}` === tool,
              ),
          )
        )
          throw new DraftError("agentRequired");
        const output = values.output.trim()
          ? object.parse(JSON.parse(values.output))
          : undefined;
        if (
          output &&
          (output.type !== "object" ||
            !output.properties ||
            !Object.keys(object.parse(output.properties)).length)
        )
          throw new DraftError("outputInvalid");
        step.agent = {
          instruction: values.instruction,
          tools: values.tools,
          maxIterations: values.maxIterations,
          ...(output ? { output } : {}),
        };
      } else {
        const contract = available.find(
          (item) => `${item.name}@${item.version}` === values.capability,
        );
        if (!contract) throw new DraftError("capabilityRequired");
        step.capability = contract.name;
        step.version = contract.version;
        if (kind === "poll") {
          if (!values.doneField.trim()) throw new DraftError("pollRequired");
          step.poll = {
            doneField: values.doneField.trim(),
            maxSteps: values.maxSteps,
          };
        }
      }
      let originalCase: unknown;
      if (request.kind === "sibling") {
        if (!values.from || !values.field.trim() || !values.equals.trim())
          throw new DraftError("conditionMissing");
        step.when = {
          from: values.from,
          field: values.field.trim(),
          equals: JSON.parse(values.equals),
        };
        const source = diagram.nodes.find((node) => node.id === values.from);
        const output =
          source?.draftStep?.agent?.output ??
          contracts.find(
            (item) =>
              item.name === source?.capability &&
              item.version === source?.version,
          )?.outputSchema;
        if (source?.draftStep?.agent && !output)
          throw new DraftError("outputInvalid");
        const outputSchema = object.safeParse(output);
        const properties = outputSchema.success
          ? object.safeParse(outputSchema.data.properties)
          : undefined;
        if (properties?.success && Object.keys(properties.data).length > 0) {
          const property = object.safeParse(properties.data[step.when.field]);
          if (!property.success) throw new DraftError("conditionFieldInvalid");
          const value = step.when.equals;
          const type = property.data.type;
          if (
            (type === "string" && typeof value !== "string") ||
            (type === "boolean" && typeof value !== "boolean") ||
            ((type === "number" || type === "integer") &&
              (typeof value !== "number" ||
                (type === "integer" && !Number.isInteger(value))))
          )
            throw new DraftError("caseType");
        }
        if (!anchor?.when) originalCase = JSON.parse(values.originalCase);
      }
      onAdd(step, originalCase);
    } catch (error) {
      if (!(
        error instanceof DraftError ||
        error instanceof SyntaxError ||
        error instanceof z.ZodError
      ))
        throw error;
      form.setError("root", {
        message: `editor.${error instanceof DraftError ? error.code : "invalidJson"}`,
      });
    }
  });
  const error = Object.values(form.formState.errors).find(
    (item) => item?.message,
  )?.message;
  const reg = form.register;
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t(request.kind === "insert" ? "editor.insert" : "editor.sibling")}
      description={t("editor.localHint")}
      wide
    >
      <form className="workflow-node-form" onSubmit={submit}>
        <label>
          <span>{t("editor.id")}</span>
          <input
            {...reg("id")}
            autoFocus
            aria-invalid={!!form.formState.errors.id}
          />
        </label>
        <label>
          <span>{t("editor.kind")}</span>
          <select {...reg("kind")}>
            <option value="step">{t("kind.step")}</option>
            <option value="agent">{t("kind.agent")}</option>
            <option value="poll">{t("kind.poll")}</option>
          </select>
        </label>
        {catalog.error && (
          <ErrorBanner
            error={catalog.error}
            onRetry={() => void catalog.refetch()}
          />
        )}
        {catalog.data && catalog.data.status !== 200 && (
          <p role="alert">{t("editor.catalogError")}</p>
        )}
        {kind !== "agent" ? (
          <label>
            <span>{t("editor.capability")}</span>
            <select {...reg("capability")}>
              <option value="">
                {t(
                  catalog.isPending
                    ? "editor.catalogLoading"
                    : "editor.chooseCapability",
                )}
              </option>
              {available.map((item) => (
                <option
                  key={`${item.name}@${item.version}`}
                  value={`${item.name}@${item.version}`}
                >{`${item.name}@${item.version}`}</option>
              ))}
            </select>
          </label>
        ) : (
          <>
            <label>
              <span>{t("editor.instruction")}</span>
              <textarea rows={3} {...reg("instruction")} />
            </label>
            <label>
              <span>{t("viewer.tools")}</span>
              <select multiple {...reg("tools")}>
                {contracts.map((item) => (
                  <option
                    key={`${item.name}@${item.version}`}
                    value={`${item.name}@${item.version}`}
                  >{`${item.name}@${item.version}`}</option>
                ))}
              </select>
            </label>
            <label>
              <span>{t("editor.iterations")}</span>
              <input
                type="number"
                min={1}
                max={20}
                {...reg("maxIterations", { valueAsNumber: true })}
              />
            </label>
            <label>
              <span>{t("editor.output")}</span>
              <textarea rows={3} {...reg("output")} />
            </label>
          </>
        )}
        {kind === "poll" && (
          <>
            <label>
              <span>{t("editor.doneField")}</span>
              <input {...reg("doneField")} />
            </label>
            <label>
              <span>{t("editor.maxSteps")}</span>
              <input
                type="number"
                min={1}
                max={100}
                {...reg("maxSteps", { valueAsNumber: true })}
              />
            </label>
          </>
        )}
        <label>
          <span>{t("editor.constants")}</span>
          <textarea rows={3} {...reg("constants")} />
        </label>
        <label>
          <span>{t("editor.input")}</span>
          <textarea
            rows={3}
            {...reg("input")}
            aria-label={t("editor.input")}
            aria-describedby={`${hintId}-mapping`}
          />
          <small id={`${hintId}-mapping`}>{t("editor.mappingHint")}</small>
        </label>
        {request.kind === "sibling" && (
          <fieldset>
            <legend>{t("viewer.condition")}</legend>
            <label>
              <span>{t("viewer.conditionSource")}</span>
              <select {...reg("from")} disabled={!!anchor?.when}>
                <option value="">{t("editor.chooseSource")}</option>
                {sources.map((source) => (
                  <option key={source} value={source}>
                    {source}
                  </option>
                ))}
              </select>
            </label>
            <label>
              <span>{t("viewer.conditionField")}</span>
              <input {...reg("field")} readOnly={!!anchor?.when} />
            </label>
            <label>
              <span>{t("viewer.conditionValue")}</span>
              <input
                {...reg("equals")}
                aria-label={t("viewer.conditionValue")}
                aria-describedby={`${hintId}-value`}
              />
              <small id={`${hintId}-value`}>{t("editor.valueHint")}</small>
            </label>
            {!anchor?.when && (
              <label>
                <span>{t("editor.originalCase")}</span>
                <input
                  {...reg("originalCase")}
                  aria-label={t("editor.originalCase")}
                  aria-describedby={`${hintId}-original`}
                />
                <small id={`${hintId}-original`}>
                  {t("editor.originalHint")}
                </small>
              </label>
            )}
          </fieldset>
        )}
        {error && (
          <p role="alert" className="orchestration-problem">
            {t(error)}
          </p>
        )}
        <div className="orchestration-actions">
          <button
            type="button"
            className="ai-button is-ghost"
            onClick={onClose}
          >
            {t("editor.cancel")}
          </button>
          <button
            type="submit"
            className="ai-button"
            disabled={
              catalog.isPending ||
              !!catalog.error ||
              catalog.data?.status !== 200
            }
          >
            {t("editor.add")}
          </button>
        </div>
      </form>
    </EditorDialog>
  );
}
