import type { TFunction } from "i18next";

// Capability names contain dots, which i18next would read as nesting. The label
// helper flattens them so resources stay one level deep per capability.
export function capabilityKey(name: string) {
  return name.replace(/\./g, "_");
}

export function capabilityLabel(t: TFunction, name: string) {
  if (!name) return "";
  return t(`capability:definitions.${capabilityKey(name)}.title`, {
    defaultValue: name,
  });
}

export function capabilityDescription(
  t: TFunction,
  name: string,
  fallback: string,
) {
  return t(`capability:definitions.${capabilityKey(name)}.description`, {
    defaultValue: fallback,
  });
}

export function formatMoment(language: string, value: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat(language, {
    dateStyle: "short",
    timeStyle: "medium",
  }).format(date);
}

export const riskOrder = ["low", "medium", "high", "critical"] as const;

export function riskTone(risk: string) {
  switch (risk) {
    case "medium":
      return "is-medium";
    case "high":
      return "is-high";
    case "critical":
      return "is-critical";
    default:
      return "is-low";
  }
}
