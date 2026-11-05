import { useState } from "react";
import { useRuntime } from "./v2-registry";
type Preferences = { hidden: string[]; widths: Record<string, number> };
export function useTablePreferences() {
  const ctx = useRuntime();
  const key = JSON.stringify([
    "runtime-table",
    ctx.identity,
    ctx.application.id,
    ctx.page.id,
    ctx.application.metadataVersion,
  ]);
  const load = (): Preferences => {
    try {
      const raw: unknown = JSON.parse(sessionStorage.getItem(key) ?? "null");
      if (
        raw &&
        typeof raw === "object" &&
        "hidden" in raw &&
        "widths" in raw &&
        Array.isArray(raw.hidden) &&
        raw.widths &&
        typeof raw.widths === "object"
      ) {
        const names = new Set(
          ctx.page.table.columns.map((field) => field.name),
        );
        return {
          hidden: raw.hidden.filter(
            (name): name is string =>
              typeof name === "string" && names.has(name),
          ),
          widths: Object.fromEntries(
            Object.entries(raw.widths).filter(
              ([name, width]) =>
                names.has(name) &&
                typeof width === "number" &&
                width >= 64 &&
                width <= 1200,
            ),
          ),
        };
      }
    } catch {
      return { hidden: [], widths: {} };
    }
    return { hidden: [], widths: {} };
  };
  const [state, setState] = useState(() => ({ key, ...load() }));
  if (state.key !== key) setState({ key, ...load() });
  const save = (next: Preferences) => {
    setState({ key, ...next });
    sessionStorage.setItem(key, JSON.stringify(next));
  };
  return {
    hidden: state.hidden,
    widths: state.widths,
    setHidden: (update: string[] | ((old: string[]) => string[])) =>
      save({
        ...state,
        hidden: typeof update === "function" ? update(state.hidden) : update,
      }),
    setWidths: (
      update:
        | Record<string, number>
        | ((old: Record<string, number>) => Record<string, number>),
    ) =>
      save({
        ...state,
        widths: typeof update === "function" ? update(state.widths) : update,
      }),
  };
}
