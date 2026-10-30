import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import {
  Plug,
  Unplug,
  Download,
  Terminal,
  Bot,
  ShieldCheck,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PageHeading } from "@/features/administration/shared";
import { defaultWebMcpPort, type AgentClient } from "@/lib/webmcp/connection";
import { useWebMcp } from "./provider";
import "./webmcp.css";
const schema = z.object({
  port: z.number().int().min(1024).max(65535),
  token: z.string().regex(/^[a-f0-9]{64}$/),
});
export function WebMcpSettingsPage() {
  const { t } = useTranslation("webmcp");
  const { connection, status, toolCount } = useWebMcp();
  const [feedback, setFeedback] = useState<
    "connectFailed" | "setupFailed" | "setupDone" | "disconnectFailed" | null
  >(null);
  const [installing, setInstalling] = useState<AgentClient | null>(null);
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: { port: connection.port || defaultWebMcpPort, token: "" },
  });
  const busy = status === "connecting" || !!installing;
  const registered = connection.registry.list();
  const submit = form.handleSubmit(async ({ port, token }) => {
    setFeedback(null);
    try {
      await connection.connect(port, token);
      form.resetField("token");
    } catch {
      setFeedback("connectFailed");
    }
  });
  async function install(client: AgentClient) {
    setInstalling(client);
    setFeedback(null);
    try {
      await connection.install(client);
      setFeedback("setupDone");
    } catch {
      setFeedback("setupFailed");
    } finally {
      setInstalling(null);
    }
  }
  return (
    <>
      <PageHeading title={t("title")} subtitle={t("subtitle")} />
      <div className="webmcp-stack">
        <section className="admin-panel webmcp-section">
          <div className="webmcp-section-heading">
            <Plug size={18} />
            <h2>{t("connection")}</h2>
            <span className="neutral-badge" role="status">
              {t(`status.${status}`)}
            </span>
          </div>
          <p className="muted-text">{t("connectionHint")}</p>
          <p className="muted-text">{t("layersHint")}</p>
          <p className="muted-text">{t("coverageHint")}</p>
          <p className="muted-text">
            {t("layerCounts", {
              global: registered.filter(
                (tool) => tool.metadata?.scope === "global",
              ).length,
              module: registered.filter(
                (tool) => tool.metadata?.scope === "module",
              ).length,
              page: registered.filter((tool) => tool.metadata?.scope === "page")
                .length,
            })}
          </p>
          <pre className="webmcp-code">
            <code>{t("startCommand")}</code>
          </pre>
          <form onSubmit={submit} noValidate className="entity-form">
            <div className="editor-grid">
              <div className="editor-field">
                <label htmlFor="webmcp-port">{t("port")}</label>
                <Input
                  id="webmcp-port"
                  aria-invalid={!!form.formState.errors.port}
                  aria-describedby={
                    form.formState.errors.port ? "webmcp-port-error" : undefined
                  }
                  type="number"
                  min={1024}
                  max={65535}
                  disabled={status === "connected" || busy}
                  {...form.register("port", { valueAsNumber: true })}
                />
                {form.formState.errors.port && (
                  <small
                    id="webmcp-port-error"
                    role="alert"
                    className="field-error"
                  >
                    {t("invalidPort")}
                  </small>
                )}
              </div>
              <div className="editor-field">
                <label htmlFor="webmcp-key">{t("pairingKey")}</label>
                <Input
                  id="webmcp-key"
                  aria-invalid={!!form.formState.errors.token}
                  aria-describedby={
                    form.formState.errors.token ? "webmcp-key-error" : undefined
                  }
                  type="password"
                  autoComplete="off"
                  disabled={status === "connected" || busy}
                  {...form.register("token")}
                />
                {form.formState.errors.token && (
                  <small
                    id="webmcp-key-error"
                    role="alert"
                    className="field-error"
                  >
                    {t("invalidKey")}
                  </small>
                )}
              </div>
            </div>
            <div className="webmcp-actions">
              <Button type="submit" disabled={busy || status === "connected"}>
                <Plug size={14} />
                {t("connect")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={status === "disconnected"}
                onClick={async () => {
                  try {
                    await connection.disconnect();
                    setFeedback(null);
                  } catch {
                    setFeedback("disconnectFailed");
                  }
                }}
              >
                <Unplug size={14} />
                {t("disconnect")}
              </Button>
              <span className="muted-text">
                {t("toolCount", { count: toolCount })}
              </span>
            </div>
          </form>
        </section>
        {feedback && (
          <p role={feedback === "setupDone" ? "status" : "alert"}>
            {t(feedback)}
          </p>
        )}
        <section className="admin-panel webmcp-section">
          <div className="webmcp-section-heading">
            <Terminal size={18} />
            <h2>{t("clients")}</h2>
          </div>
          <p className="muted-text">{t("clientsHint")}</p>
          <div className="webmcp-client-grid">
            {(["codex", "workbuddy"] as const).map((client) => (
              <div className="webmcp-client" key={client}>
                <h3>{t(`client.${client}`)}</h3>
                <p className="muted-text">{t(`clientHint.${client}`)}</p>
                <Button
                  type="button"
                  variant="outline"
                  disabled={status !== "connected" || busy}
                  onClick={() => {
                    void install(client);
                  }}
                >
                  <Download size={14} />
                  {t(installing === client ? "installing" : "install")}
                </Button>
              </div>
            ))}
          </div>
          <p className="muted-text">{t("otherClients")}</p>
          <pre className="webmcp-code">
            <code>{t("genericCommand")}</code>
          </pre>
        </section>
        <section className="admin-panel webmcp-section">
          <div className="webmcp-section-heading">
            <ShieldCheck size={18} />
            <h2>{t("skills")}</h2>
          </div>
          <ul className="webmcp-skills">
            <li>
              <strong>{t("skillNames.navigate")}</strong>
              <span>{t("skillHints.navigate")}</span>
            </li>
            <li>
              <strong>{t("skillNames.accounts")}</strong>
              <span>{t("skillHints.accounts")}</span>
            </li>
            <li>
              <strong>{t("skillNames.diagnose")}</strong>
              <span>{t("skillHints.diagnose")}</span>
            </li>
          </ul>
          <p className="muted-text">{t("safety")}</p>
        </section>
        <section className="admin-panel webmcp-section">
          <div className="webmcp-section-heading">
            <Bot size={18} />
            <h2>{t("assistant")}</h2>
            <span className="neutral-badge">{t("reserved")}</span>
          </div>
          <p className="muted-text">{t("assistantHint")}</p>
        </section>
      </div>
    </>
  );
}
