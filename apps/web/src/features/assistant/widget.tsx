import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { ArrowUp, Bot, Eraser, Loader2, Square, X } from "lucide-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { useAssistant, type AssistantTurn } from "./provider";
import "./assistant.css";

function Turn({ turn }: { turn: AssistantTurn }) {
  const { t } = useTranslation("assistant");
  if (turn.role === "tool") {
    return (
      <div
        className={`assistant-tool ${turn.status === "failed" ? "is-failed" : ""}`}
      >
        <span className="assistant-tool-name">{turn.toolName}</span>
        <code>{turn.content}</code>
        <span className="sr-only">{t("toolResult")}</span>
      </div>
    );
  }
  const mine = turn.role === "user";
  return (
    <div className={`assistant-turn ${mine ? "is-user" : "is-assistant"}`}>
      <div className="assistant-bubble">
        {mine ? (
          <p>{turn.content}</p>
        ) : (
          <div className="assistant-markdown">
            <Markdown remarkPlugins={[remarkGfm]}>{turn.content}</Markdown>
          </div>
        )}
        {turn.toolCalls?.length ? (
          <ul className="assistant-calls">
            {turn.toolCalls.map((call) => (
              <li key={call.id}>{t("toolRunning", { name: call.name })}</li>
            ))}
          </ul>
        ) : null}
      </div>
    </div>
  );
}

export function AssistantLauncher() {
  const { t } = useTranslation("assistant");
  const assistant = useAssistant();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const panelId = useId();
  const titleId = useId();
  const launcher = useRef<HTMLButtonElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const log = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const focus = window.setTimeout(() => composer.current?.focus(), 40);
    const escape = (event: globalThis.KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.stopPropagation();
      setOpen(false);
      launcher.current?.focus();
    };
    window.addEventListener("keydown", escape);
    return () => {
      window.clearTimeout(focus);
      window.removeEventListener("keydown", escape);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const element = log.current;
    if (element) element.scrollTop = element.scrollHeight;
  }, [open, assistant.messages, assistant.busy]);

  // The switch lives in the admin settings; without it the assistant stays hidden.
  if (!assistant.ready) return null;

  const submit = () => {
    const text = draft.trim();
    if (!text || assistant.busy) return;
    setDraft("");
    void assistant.send(text);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      submit();
    }
  };

  return (
    <div className="assistant-root">
      {open ? (
        <section
          className="assistant-panel"
          id={panelId}
          role="dialog"
          aria-labelledby={titleId}
        >
          <header className="assistant-header">
            <span className="assistant-avatar" aria-hidden="true">
              <Bot size={16} />
            </span>
            <div className="assistant-heading">
              <h2 id={titleId}>{t("title")}</h2>
              <small>
                {assistant.model
                  ? t("subtitle", { model: assistant.model })
                  : t("subtitleUnknown")}
              </small>
            </div>
            <button
              type="button"
              className="assistant-icon-button"
              onClick={assistant.clear}
              aria-label={t("clear")}
              title={t("clear")}
              disabled={!assistant.messages.length}
            >
              <Eraser size={15} />
            </button>
            <button
              type="button"
              className="assistant-icon-button"
              onClick={() => {
                setOpen(false);
                launcher.current?.focus();
              }}
              aria-label={t("close")}
              title={t("close")}
            >
              <X size={15} />
            </button>
          </header>

          <div className="assistant-log" ref={log}>
            {!assistant.configured ? (
              <p className="assistant-notice">{t("notConfigured")}</p>
            ) : null}
            {!assistant.messages.length ? (
              <div className="assistant-empty">
                <span className="assistant-avatar large" aria-hidden="true">
                  <Bot size={20} />
                </span>
                <h3>{t("emptyTitle")}</h3>
                <p>{t("emptyHint")}</p>
                <small>{t("toolCount", { count: assistant.toolCount })}</small>
              </div>
            ) : (
              assistant.messages.map((turn) => (
                <Turn key={turn.id} turn={turn} />
              ))
            )}
            {assistant.busy ? (
              <div className="assistant-thinking">
                <Loader2 size={14} className="assistant-spin" />
                {t("thinking")}
              </div>
            ) : null}
            {assistant.error ? (
              <p className="assistant-error" role="alert">
                {t(`errors.${assistant.error}` as "errors.failed")}
              </p>
            ) : null}
          </div>

          <footer className="assistant-composer">
            <textarea
              ref={composer}
              value={draft}
              rows={2}
              placeholder={t("placeholder")}
              aria-label={t("placeholder")}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={onKeyDown}
            />
            {assistant.busy ? (
              <button
                type="button"
                className="assistant-send is-stop"
                onClick={assistant.stop}
                aria-label={t("stop")}
                title={t("stop")}
              >
                <Square size={14} />
              </button>
            ) : (
              <button
                type="button"
                className="assistant-send"
                onClick={submit}
                disabled={!draft.trim() || !assistant.configured}
                aria-label={t("send")}
                title={t("send")}
              >
                <ArrowUp size={16} />
              </button>
            )}
          </footer>
        </section>
      ) : null}

      <button
        ref={launcher}
        type="button"
        className={`assistant-launcher ${open ? "is-open" : ""}`}
        aria-label={open ? t("minimize") : t("open")}
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        title={open ? t("minimize") : t("open")}
        onClick={() => setOpen((value) => !value)}
      >
        {open ? <X size={20} /> : <Bot size={22} />}
      </button>
    </div>
  );
}
