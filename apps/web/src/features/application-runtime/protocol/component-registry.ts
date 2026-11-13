import { createContext, type ReactNode } from "react";

export const PresentationPermissions = createContext<readonly string[]>([]);
export const PresentationContext = createContext<Record<string, unknown>>({});

export type ProtocolEventPayload = unknown;
export type ProtocolEventOptions = {
  ephemeral?: boolean;
  onAIResult?: (data: unknown) => void;
  onOutcome?: (status: "succeeded" | "failed") => void;
  onError?: (code: string) => void;
};

export interface ComponentRendererProps {
  props: Record<string, unknown>;
  data?: unknown;
  dataRefreshing?: boolean;
  children?: ReactNode;
  dataMap?: Record<string, unknown>;
  readOnly: boolean;
  componentId?: string;
  events?: readonly string[];
  emit: (
    event: string,
    payload?: ProtocolEventPayload,
    options?: ProtocolEventOptions,
  ) => void | Promise<boolean | void>;
}

export type ComponentRenderer = (props: ComponentRendererProps) => ReactNode;

export class ComponentRegistry {
  private components = new Map<string, ComponentRenderer>();

  register(kind: string, version: string, renderer: ComponentRenderer): this {
    const key = `${kind}@${version}`;
    if (this.components.has(key)) {
      throw new Error(`component_already_registered: ${key}`);
    }
    this.components.set(key, renderer);
    return this;
  }

  resolve(kind: string, version: string): ComponentRenderer | undefined {
    return this.components.get(`${kind}@${version}`);
  }

  has(kind: string, version: string): boolean {
    return this.components.has(`${kind}@${version}`);
  }

  get registeredKinds(): string[] {
    return [...this.components.keys()].sort();
  }
}
