// Code generated from AI configuration capability definitions. DO NOT EDIT.

export type AiConfigReadInput = {};
export type AiConfigReadOutput = {
  embeddingModels?: Array<{ id: string; label: string }>;
  entries: Array<{
    configured: boolean;
    key: string;
    overridden: boolean;
    value: string;
  }>;
  revision: number;
};

export type AiConfigSaveInput = {
  baseRevision: number;
  changes: Array<{
    key: string;
    operation: "set" | "reset" | "clear";
    value?: string;
  }>;
};
export type AiConfigSaveOutput = { revision: number };
