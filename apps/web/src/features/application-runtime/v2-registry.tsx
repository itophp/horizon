import { preparePresentation } from "./presentation";
import { contributionResources } from "./contributions";
import { componentRegistry, type RecordData } from "./components";
export {
  componentRegistry,
  ComponentRegistry,
  RegisteredControl,
  RegisteredDisplay,
  type RecordData,
  type RuntimeComponent,
  type ControlProps,
} from "./components";
import {
  createContext,
  useContext,
  Component,
  lazy,
  type ReactNode,
  type ComponentType,
} from "react";
import {
  runtimeApplicationV2Schema,
  runtimeSemanticContextSchema,
  type RuntimeAction,
  type RuntimeRecordReference,
  type RuntimeApplicationV2,
  type RuntimePageV2,
} from "@/api/generated/runtime";
import { useTranslation } from "react-i18next";
import { evaluate, pathValue, type Json, type Scope } from "./expression";
export type ViewContext = {
  application: RuntimeApplicationV2;
  page: RuntimePageV2;
  scope: Scope;
  identity: string;
  selection: Json[];
  setSelection: (ids: Json[]) => void;
  correlationId: string;
  readOnly: boolean;
};
export const RuntimeContext = createContext<ViewContext | null>(null);
export function useRuntime() {
  const value = useContext(RuntimeContext);
  if (!value) throw new Error("runtime_context_invalid");
  return value;
}
export type Extension = {
  id: string;
  source: string;
  version: string;
  type:
    | "component"
    | "action"
    | "context"
    | "provider"
    | "renderer"
    | "custom-view"
    | "shell";
  render?: ComponentType<{ record: RecordData }>;
  load?: () => Promise<{ default: ComponentType<{ record: RecordData }> }>;
  normalize?: (
    input: Record<string, unknown>,
    scope: Scope,
  ) => Record<string, unknown>;
  dispose?: () => void;
  propsSchema?: import("zod").z.ZodType<{ record: RecordData }>;
  context?: (scope: Scope) => RuntimeRecordReference[];
};
export class ExtensionRegistry {
  private items = new Map<string, Extension>();
  register(x: Extension) {
    const key = `${x.id}@${x.version}`;
    if (this.items.has(key) || !x.source || !x.version)
      throw new Error("plugin_incompatible");
    this.items.set(key, x.load ? { ...x, render: lazy(x.load) } : x);
    return this;
  }
  attach(x: Extension) {
    this.register(x);
    return () => {
      const key = `${x.id}@${x.version}`;
      const item = this.items.get(key);
      try {
        item?.dispose?.();
      } finally {
        this.items.delete(key);
      }
    };
  }
  unregisterSource(source: string) {
    for (const [id, item] of this.items)
      if (item.source === source) {
        item.dispose?.();
        this.items.delete(id);
      }
  }
  resolve(id: string, app: RuntimeApplicationV2) {
    const declaration = app.extensions.find((e) => e.id === id);
    const x = declaration
      ? this.items.get(`${id}@${declaration.version}`)
      : undefined;
    if (
      !x ||
      !declaration ||
      declaration.source !== x.source ||
      declaration.version !== x.version ||
      declaration.type !== x.type
    )
      throw new Error("component_not_registered");
    return x;
  }
}
export const extensions = new ExtensionRegistry();
export function validateDefinition(raw: unknown) {
  const app = runtimeApplicationV2Schema.parse(raw);
  if (
    !["2.0", "2.1", "2.2"].includes(app.schemaVersion) ||
    !["2.0", "2.1", "2.2"].includes(app.runtimeCompatibility)
  )
    throw new Error("schema_version_unsupported");
  const ids = new Set<string>();
  for (const p of app.pages) {
    if (ids.has(p.id)) throw new Error("schema_invalid");
    ids.add(p.id);
    if (
      p.type === "form" &&
      p.actions.filter(
        (action) =>
          action.type === "dialog" &&
          (action.mode === "create" || action.mode === "edit"),
      ).length !== 1
    )
      throw new Error("schema_invalid");
    for (const f of [
      ...p.fields,
      ...p.table.columns,
      ...p.forms.flatMap((f) => [...f.create.fields, ...f.edit.fields]),
    ]) {
      const declaration = app.extensions.find(
        (x) =>
          x.type === "component" &&
          x.entry === f.component &&
          x.version === f.componentVersion,
      );
      if (
        declaration &&
        contributionResources.has(
          declaration.source,
          declaration.entry,
          declaration.version,
        )
      )
        continue;
      componentRegistry.resolve(f);
      const owner = componentRegistry.owner(f);
      if (
        owner !== "core" &&
        !app.extensions.some(
          (x) =>
            x.type === "component" &&
            x.entry === f.component &&
            x.version === f.componentVersion &&
            x.source === owner,
        )
      )
        throw new Error("plugin_incompatible");
    }
  }
  return preparePresentation(app);
}
const diagnosticCodes = new Set([
  "schema_invalid",
  "schema_version_unsupported",
  "component_not_registered",
  "plugin_incompatible",
  "action_not_found",
  "field_not_allowed",
  "page_not_found",
  "runtime_context_invalid",
]);
function diagnosticCode(error: unknown) {
  const code = error instanceof Error ? error.message.split(":")[0] : "";
  return diagnosticCodes.has(code) ? code : "schema_invalid";
}
export function Diagnostic({ error }: { error: unknown }) {
  const { t } = useTranslation("runtime");
  return (
    <div role="alert">
      {t("schemaError")} <code>{diagnosticCode(error)}</code>
    </div>
  );
}
export class LocalBoundary extends Component<
  { children: ReactNode },
  { failed: boolean; error: unknown }
> {
  state: { failed: boolean; error: unknown } = {
    failed: false,
    error: undefined,
  };
  static getDerivedStateFromError(error: unknown) {
    return { failed: true, error };
  }
  render() {
    return this.state.failed ? (
      <Diagnostic error={this.state.error} />
    ) : (
      this.props.children
    );
  }
}
// Semantic context deliberately exports references and action IDs, never record bodies or draft values.
export function actionVisible(
  ctx: ViewContext,
  action: RuntimeAction,
  scope: Scope,
): boolean {
  if (
    (ctx.readOnly &&
      action.type !== "navigation" &&
      !(action.type === "dialog" && action.mode === "detail")) ||
    !evaluate(action.when, scope) ||
    (action.type === "agent" &&
      action.capability.name !== "orchestration.workflow.run")
  )
    return false;
  if (action.type === "dialog" && action.mode !== "detail") {
    const form = ctx.page.forms.find((f) => f.id === action.target);
    const command = action.mode === "edit" ? form?.edit : form?.create;
    if (!command?.capability.name || !evaluate(command.when, scope))
      return false;
  }
  if (action.type === "custom") {
    if (!extensions.resolve(action.target, ctx.application).normalize)
      throw new Error("action_not_found");
  }
  return true;
}
export function semanticContext(ctx: ViewContext) {
  const provider = ctx.application.providers.find(
    (p) => p.id === ctx.page.provider,
  );
  const id = pathValue(ctx.scope.record, provider?.keyField ?? "id");
  const recordId =
    typeof id === "string" || typeof id === "number" ? String(id) : "";
  const selection = ctx.selection.map((id) => ({
    entityRef: ctx.page.entityRef,
    recordId: String(id),
  }));
  const allowed = new Set([recordId, ...selection.map((r) => r.recordId)]);
  const references: RuntimeRecordReference[] = [];
  for (const declaration of ctx.application.extensions.filter(
    (x) => x.type === "context",
  )) {
    const extension = extensions.resolve(declaration.id, ctx.application);
    if (!extension.context) throw new Error("component_not_registered");
    const result = extension.context({
      context: ctx.scope.context,
      record: { id: recordId },
      selection: { ids: selection.map((r) => r.recordId) },
    });
    for (const ref of result) {
      if (ref.entityRef !== ctx.page.entityRef || !allowed.has(ref.recordId))
        throw new Error("field_not_allowed");
      references.push(ref);
    }
  }
  const actions = ctx.page.actions
    .filter(
      (a) =>
        actionVisible(ctx, a, ctx.scope) &&
        (a.placement !== "selection" || ctx.selection.length > 0),
    )
    .map((a) => {
      const form = ctx.page.forms.find((f) => f.id === a.target);
      return {
        id: a.id,
        labelKey: a.labelKey,
        capability:
          a.type === "dialog"
            ? a.mode === "edit"
              ? form!.edit.capability
              : form!.create.capability
            : a.capability,
      };
    });
  return runtimeSemanticContextSchema.parse({
    applicationId: ctx.application.id,
    applicationVersion:
      ctx.application.applicationVersion ?? ctx.application.version,
    metadataVersion: ctx.application.metadataVersion,
    pageId: ctx.page.id,
    pageType: ctx.page.type,
    entityRef: ctx.page.entityRef,
    recordId,
    selection,
    references,
    availableActions: actions,
    locale: String(pathValue(ctx.scope, "context.locale") ?? "en"),
    timezone: String(pathValue(ctx.scope, "context.timezone") ?? "UTC"),
    source: ctx.application.source,
  });
}
