import type { RuntimeApplicationV2 } from "@/api/generated/runtime";
// Local resource namespaces include the immutable configuration version so an
// old preview cannot replace translations belonging to a newer published page.
export function preparePresentation(source: RuntimeApplicationV2) {
  if (!source.messages?.length) return source;
  const namespace = `${source.id}_${source.applicationVersion ?? source.metadataVersion}`;
  const prefix = `${source.id}_local:`;
  const visit = (node: unknown): unknown => {
    if (Array.isArray(node)) return node.map(visit);
    if (node && typeof node === "object")
      return Object.fromEntries(
        Object.entries(node).map(([key, value]) => [
          key,
          [
            "labelKey",
            "titleKey",
            "confirmKey",
            "successKey",
            "valueLabelPrefix",
            "key",
          ].includes(key) &&
          typeof value === "string" &&
          value.startsWith(prefix)
            ? `${namespace}:${value.slice(prefix.length)}`
            : visit(value),
        ]),
      );
    return node;
  };
  return visit(source) as RuntimeApplicationV2;
}
