import { z } from "zod";
import type { CallToolResult } from "@opentiny/next-sdk/core";

export interface PageTool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  metadata?: ToolMetadata;
  execute: (input: unknown) => Promise<CallToolResult>;
}
export interface ToolMetadata {
  scope: "global" | "module" | "page";
  version: string;
  effect: "read" | "navigation" | "interaction";
  permission: string | null;
}
export interface ModelContext {
  registerTool: (
    tool: PageTool,
    options: { signal: AbortSignal },
  ) => void | Promise<void>;
  getTools: () => PageTool[] | Promise<PageTool[]>;
}
export function isToolAvailable(
  tool: PageTool,
  permissions: readonly string[],
) {
  return (
    !tool.metadata?.permission || permissions.includes(tool.metadata.permission)
  );
}
export const result = (value: unknown): CallToolResult => ({
  content: [{ type: "text", text: JSON.stringify(value) }],
});
export const failure = (code: string): CallToolResult => ({
  ...result({ code }),
  isError: true,
});
export function pageTool<T>(
  name: string,
  description: string,
  schema: z.ZodType<T>,
  execute: (input: T) => unknown | Promise<unknown>,
  metadata: Partial<ToolMetadata> = {},
): PageTool {
  return {
    name,
    description,
    inputSchema: z.toJSONSchema(schema),
    metadata: {
      scope: "page",
      version: "1.0.0",
      effect: "read",
      permission: null,
      ...metadata,
    },
    execute: async (input) => {
      const parsed = schema.safeParse(input);
      if (!parsed.success) return failure("invalid_arguments");
      try {
        return result(await execute(parsed.data));
      } catch {
        return failure("operation_failed");
      }
    },
  };
}
// One registry for remote MCP and a future embedded assistant. No DOM eval or API bypass.
export class PageToolRegistry {
  private tools = new Map<string, PageTool>();
  private context?: ModelContext;
  private connected = false;
  private controllers = new Map<string, AbortController>();
  private listeners = new Set<() => void>();
  onError?: () => void;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  snapshot = () => this.tools.size;
  private notify() {
    for (const listener of this.listeners) listener();
  }
  register(tool: PageTool) {
    if (this.tools.has(tool.name)) throw new Error("duplicate_tool");
    this.tools.set(tool.name, tool);
    void this.publish(tool).catch(() => {
      this.deactivate();
      this.onError?.();
    });
    this.notify();
    return () => {
      if (this.tools.get(tool.name) !== tool) return;
      this.controllers.get(tool.name)?.abort();
      this.controllers.delete(tool.name);
      this.tools.delete(tool.name);
      this.notify();
    };
  }
  private async publish(tool: PageTool) {
    if (!this.context || !this.connected) return;
    this.controllers.get(tool.name)?.abort();
    const controller = new AbortController();
    this.controllers.set(tool.name, controller);
    try {
      await this.context.registerTool(
        {
          ...tool,
          execute: (input) =>
            controller.signal.aborted
              ? Promise.resolve(failure("tool_unavailable"))
              : this.invoke(tool.name, input),
        },
        { signal: controller.signal },
      );
    } catch (error) {
      // Aborting a page registration during navigation is an expected cancellation.
      if (!controller.signal.aborted) throw error;
    }
  }
  async activate(context: ModelContext) {
    // A second activation with the same context must not republish: the embedded
    // assistant and the remote MCP client share this registry.
    if (this.connected && this.context === context) return;
    this.context = context;
    this.connected = true;
    try {
      await Promise.all(
        [...this.tools.values()].map((tool) => this.publish(tool)),
      );
    } catch (error) {
      this.deactivate();
      throw error;
    }
  }
  deactivate() {
    this.connected = false;
    for (const controller of this.controllers.values()) controller.abort();
    this.controllers.clear();
    this.context = undefined;
  }
  isActive() {
    return this.connected && this.context !== undefined;
  }
  list() {
    return [...this.tools.values()].map(
      ({ name, description, inputSchema, outputSchema, metadata }) => ({
        name,
        description,
        inputSchema,
        outputSchema,
        metadata,
      }),
    );
  }
  async invoke(name: string, input: unknown) {
    if (!this.connected) return failure("connection_disabled");
    const tool = this.tools.get(name);
    return tool ? tool.execute(input) : failure("tool_unavailable");
  }
}
