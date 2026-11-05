import { runtimeRead } from "@/lib/capabilities";
import type {
  RuntimeApplicationV2,
  RuntimeProvider,
} from "@/api/generated/runtime";
import { extensions, type RecordData } from "./v2-registry";
import { mapInput, pathValue, validateInput, type Scope } from "./expression";
export function providerOf(app: RuntimeApplicationV2, id: string) {
  const p = app.providers.find((p) => p.id === id);
  if (!p) throw new Error("field_not_allowed");
  return p;
}
export async function readProvider(
  provider: RuntimeProvider,
  scope: Scope,
  signal?: AbortSignal,
  application?: RuntimeApplicationV2,
  pageId?: string,
) {
  let input = mapInput(provider.input, scope);
  if (provider.adapter) {
    if (!application) throw new Error("runtime_context_invalid");
    const adapter = extensions.resolve(provider.adapter, application).normalize;
    if (!adapter) throw new Error("component_not_registered");
    input = adapter(input, scope);
  }
  validateInput(input, provider.inputSchema);
  const raw = await runtimeRead(
    application ? "application.runtime.data.read" : provider.capability.name,
    application
      ? {
          applicationId: application.id,
          pageId:
            pageId ??
            application.pages.find((page) => page.provider === provider.id)?.id,
          metadataVersion: application.metadataVersion,
          providerId: provider.id,
          ...(application.previewVersion
            ? { previewVersion: application.previewVersion }
            : {}),
          input,
        }
      : input,
    signal,
    application ? "1.0.0" : provider.capability.version,
  );
  validateInput(raw, provider.outputSchema);
  const value = pathValue(raw, provider.resultPath);
  const row = (v: unknown): RecordData => {
    if (
      !v ||
      typeof v !== "object" ||
      Array.isArray(v) ||
      !Object.hasOwn(v, provider.keyField)
    )
      throw new Error("schema_invalid");
    return v as RecordData;
  };
  if (Array.isArray(value))
    return {
      items: value.map(row),
      total:
        typeof pathValue(raw, "total") === "number"
          ? Number(pathValue(raw, "total"))
          : value.length,
      record: undefined,
    };
  return { items: [], total: 1, record: row(value) };
}
