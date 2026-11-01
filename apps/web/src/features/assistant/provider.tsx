import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import { useQuery } from "@tanstack/react-query";
import {
  assistantAgent,
  assistantChat,
  assistantStatus,
} from "@/api/generated/baseline";
import type {
  AssistantMessage,
  AssistantTool,
  AssistantReply,
} from "@/api/generated/models";
import { ApiError } from "@/api/client/request";
import { useWebMcp } from "@/features/webmcp/provider";
import { useSession } from "@/features/administration/auth";

export type AssistantMode = "page" | "agent";

export interface AssistantTurn {
  id: string;
  role: "user" | "assistant" | "tool";
  content: string;
  toolCalls?: AssistantReply["toolCalls"];
  tools?: string[];
  toolCallId?: string;
  toolName?: string;
  status?: "pending" | "failed";
}

export interface AssistantContextValue {
  messages: AssistantTurn[];
  busy: boolean;
  error: string | null;
  model: string;
  ready: boolean;
  configured: boolean;
  toolCount: number;
  mode: AssistantMode;
  agentEnabled: boolean;
  setMode: (mode: AssistantMode) => void;
  send: (text: string) => Promise<void>;
  stop: () => void;
  clear: () => void;
}

const Context = createContext<AssistantContextValue | null>(null);

const maximumRounds = 4;
const maximumContent = 8000;

function identifier() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function truncate(value: string) {
  return value.length > maximumContent ? value.slice(0, maximumContent) : value;
}

// Tool results arrive as MCP CallToolResult already serialised by the registry.
function toolText(result: { content?: unknown; isError?: boolean }) {
  const parts = Array.isArray(result.content) ? result.content : [];
  const text = parts
    .map((part) =>
      typeof part === "object" &&
      part !== null &&
      typeof (part as { text?: unknown }).text === "string"
        ? (part as { text: string }).text
        : "",
    )
    .filter(Boolean)
    .join("\n");
  return text || JSON.stringify({ code: result.isError ? "failed" : "empty" });
}

function toMessage(turn: AssistantTurn): AssistantMessage {
  return {
    role: turn.role === "tool" ? "tool" : turn.role,
    content: truncate(turn.content),
    ...(turn.toolCallId ? { toolCallId: turn.toolCallId } : {}),
    ...(turn.toolCalls?.length ? { toolCalls: turn.toolCalls } : {}),
  };
}

export function AssistantProvider({ children }: PropsWithChildren) {
  const { connection, toolCount } = useWebMcp();
  const session = useSession();
  const allowed = session.permissions.includes("assistant:use");
  const [messages, setMessages] = useState<AssistantTurn[]>([]);
  const [mode, setMode] = useState<AssistantMode>("page");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const controller = useRef<AbortController | null>(null);
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      controller.current?.abort();
    };
  }, []);

  const status = useQuery({
    queryKey: ["assistant-status"],
    queryFn: () => assistantStatus(),
    retry: false,
    staleTime: 60000,
    // Hiding the entry point is a convenience; the endpoint re-checks the grant.
    enabled: allowed,
  });
  const snapshot = status.data?.status === 200 ? status.data.data : undefined;
  const ready = allowed && Boolean(snapshot?.enabled);
  const configured = Boolean(snapshot?.configured);
  const agentEnabled = Boolean(snapshot?.agentEnabled);
  const model = snapshot?.model ?? "";

  // Page tools reach the model only when the workspace switch is on and a model is
  // configured; the same registry still powers the remote MCP connection.
  useEffect(() => {
    if (!ready || !configured) return;
    void connection.ensureLocalTools().catch(() => {
      console.error(
        "WebMCP page tools could not be published; the assistant has no tools.",
      );
    });
  }, [ready, configured, connection]);

  const tools = useCallback((): AssistantTool[] => {
    return connection.registry
      .list()
      .slice(0, 40)
      .map((tool) => ({
        name: tool.name,
        description: tool.description,
        parameters: tool.inputSchema,
      }));
  }, [connection]);

  const stop = useCallback(() => {
    controller.current?.abort();
    controller.current = null;
    setBusy(false);
  }, []);

  const clear = useCallback(() => {
    stop();
    setMessages([]);
    setError(null);
  }, [stop]);

  const send = useCallback(
    async (text: string) => {
      const question = text.trim();
      if (!question || busy) return;
      setError(null);
      setBusy(true);
      const abort = new AbortController();
      controller.current?.abort();
      controller.current = abort;
      let history: AssistantTurn[] = [
        ...messages,
        { id: identifier(), role: "user", content: question },
      ];
      setMessages(history);
      try {
        if (mode === "agent") {
          // The Eino agent runs the whole tool loop on the server; the browser only
          // sends the question and renders the grounded answer.
          const result = await assistantAgent(
            { question },
            { signal: abort.signal },
          );
          if (abort.signal.aborted) return;
          if (result.status !== 200) {
            throw new ApiError("unavailable", result.status);
          }
          if (!mounted.current) return;
          setMessages([
            ...history,
            {
              id: identifier(),
              role: "assistant",
              content: result.data.content,
              tools: result.data.tools,
            },
          ]);
          return;
        }
        for (let round = 0; round < maximumRounds; round += 1) {
          const result = await assistantChat(
            {
              messages: history
                .filter((turn) => turn.role !== "tool" || turn.toolCallId)
                .map(toMessage),
              tools: tools(),
            },
            { signal: abort.signal },
          );
          if (abort.signal.aborted) return;
          if (result.status !== 200) {
            throw new ApiError("unavailable", result.status);
          }
          const reply = result.data;
          const calls = reply.toolCalls ?? [];
          const turn: AssistantTurn = {
            id: identifier(),
            role: "assistant",
            content: reply.content,
            ...(calls.length ? { toolCalls: calls } : {}),
          };
          history = [...history, turn];
          setMessages(history);
          if (!calls.length) return;
          for (const call of calls) {
            const executed = await execute(call);
            history = [...history, executed];
            if (!mounted.current) return;
            setMessages(history);
          }
        }
      } catch (caught) {
        if (abort.signal.aborted) return;
        const code = caught instanceof ApiError ? caught.code : "internal";
        if (mounted.current) {
          setError(
            code === "forbidden" || code === "unauthenticated"
              ? "forbidden"
              : code === "unavailable"
                ? "unavailable"
                : "failed",
          );
        }
      } finally {
        if (controller.current === abort) controller.current = null;
        if (mounted.current) setBusy(false);
      }

      // Page tools run in the page and keep their own permission checks; the model
      // only ever requests a tool, it never executes one.
      async function execute(
        call: NonNullable<AssistantReply["toolCalls"]>[number],
      ): Promise<AssistantTurn> {
        let input: unknown;
        try {
          input = call.arguments ? JSON.parse(call.arguments) : {};
        } catch {
          return {
            id: identifier(),
            role: "tool",
            content: JSON.stringify({ code: "invalid_arguments" }),
            toolCallId: call.id,
            toolName: call.name,
            status: "failed",
          };
        }
        try {
          const result = await connection.registry.invoke(call.name, input);
          return {
            id: identifier(),
            role: "tool",
            content: truncate(toolText(result)),
            toolCallId: call.id,
            toolName: call.name,
          };
        } catch {
          return {
            id: identifier(),
            role: "tool",
            content: JSON.stringify({ code: "operation_failed" }),
            toolCallId: call.id,
            toolName: call.name,
            status: "failed",
          };
        }
      }
    },
    [busy, connection, messages, mode, tools],
  );

  const value = useMemo<AssistantContextValue>(
    () => ({
      messages,
      busy,
      error,
      model,
      ready,
      configured,
      toolCount,
      mode,
      agentEnabled,
      setMode,
      send,
      stop,
      clear,
    }),
    [
      messages,
      busy,
      error,
      model,
      ready,
      configured,
      toolCount,
      mode,
      agentEnabled,
      setMode,
      send,
      stop,
      clear,
    ],
  );
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useAssistant() {
  const value = useContext(Context);
  if (!value) throw new Error("AssistantProvider required");
  return value;
}
