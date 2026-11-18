// Code generated from Knowledge Base capability definitions. DO NOT EDIT.

export type KnowledgeQueryInput = {
  beforeId?: number;
  keyword?: string;
  limit?: number;
};
export type KnowledgeQueryOutput = {
  documents: Array<{
    chunks: number;
    createdAt: string;
    id: number;
    source: "manual" | "file";
    title: string;
  }>;
  nextBeforeId?: number;
};

export type KnowledgeReadInput = { id: number; offset?: number };
export type KnowledgeReadOutput = {
  chunks: number;
  content: string;
  createdAt: string;
  id: number;
  nextOffset?: number;
  source: "manual" | "file";
  title: string;
  version?: string;
};

export type KnowledgeUpdateInput = {
  content?: string;
  expectedVersion: string;
  id: number;
  replacement?: { find: string; text: string };
  title?: string;
};
export type KnowledgeUpdateOutput = { id: number };
