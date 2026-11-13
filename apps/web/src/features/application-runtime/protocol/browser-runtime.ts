import { createContext } from "react";

export type BrowserRuntime = {
  presentation: Record<string, unknown>;
  execute: (
    target: string,
    values: Record<string, unknown>,
  ) => Promise<boolean>;
  formDefaults: (
    preset: string,
    signal: AbortSignal,
  ) => Promise<Record<string, string> | null>;
};

export const BrowserRuntimeContext = createContext<BrowserRuntime | null>(null);

// Only the validated Runtime page passes its adapter to form components.
export const BrowserFormContext = createContext<
  BrowserRuntime["formDefaults"] | null
>(null);
