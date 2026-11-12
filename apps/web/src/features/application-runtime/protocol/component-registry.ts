import { createContext, type ReactNode } from "react";

export const PresentationPermissions = createContext<readonly string[]>([]);

export type ProtocolEventPayload = unknown;

export interface ComponentRendererProps {
  props: Record<string, unknown>;
  data?: unknown;
  children?: ReactNode;
  dataMap?: Record<string, unknown>;
  readOnly: boolean;
  emit: (
    event: string,
    payload?: ProtocolEventPayload,
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
