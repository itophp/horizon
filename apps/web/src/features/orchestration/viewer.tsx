import { useState } from "react";
import { useTranslation } from "react-i18next";
import { WorkflowGraph, type Diagram } from "./graph";
import { connectionCondition, connectionId } from "./connections";
import {
  applyAddition,
  applyRemoval,
  DraftError,
  type AddRequest,
} from "./draft";
import { NodeEditor } from "./node-editor";

export function WorkflowViewer({ diagram: original }: { diagram: Diagram }) {
  const [history, setHistory] = useState<Diagram[]>([original]);
  const diagram = history[history.length - 1];
  const [request, setRequest] = useState<AddRequest>();
  const [direction, setDirection] = useState<"TB" | "LR">("TB");
  const [removalError, setRemovalError] = useState("");
  const { t } = useTranslation("orchestration");
  const [selection, setSelection] = useState({ kind: "node", id: "start" });
  const selected = selection.kind === "node" ? selection.id : "";
  const selectedEdge = selection.kind === "edge" ? selection.id : "";
  const node = diagram.nodes.find((item) => item.id === selected);
  const edge = diagram.edges.find(
    (item) => connectionId(item) === selectedEdge,
  );
  const when = edge && connectionCondition(diagram, edge);
  const selectNode = (id: string) => setSelection({ kind: "node", id });
  const selectEdge = (id: string) => setSelection({ kind: "edge", id });
  const [tab, setTab] = useState<"graph" | "input">("graph");
  const removeNode = (id: string) => {
    try {
      const next = applyRemoval(diagram, id);
      setHistory((items) => [...items, next]);
      selectNode("start");
      setRemovalError("");
    } catch (error) {
      if (!(error instanceof DraftError)) throw error;
      setRemovalError(`editor.${error.code}`);
    }
  };
  return (
    <div className="workflow-viewer">
      <p className="workflow-draft-notice" role="status">
        {t(history.length > 1 ? "editor.draftHint" : "editor.startHint")}
      </p>
      {removalError && (
        <p role="alert" className="orchestration-problem">
          {t(removalError)}
        </p>
      )}
      {history.length > 1 && (
        <div className="orchestration-actions">
          <button
            type="button"
            className="ai-button is-ghost"
            onClick={() => {
              setHistory((items) => items.slice(0, -1));
              selectNode("start");
              setRemovalError("");
            }}
          >
            {t("editor.undo")}
          </button>
          <button
            type="button"
            className="ai-button is-ghost"
            onClick={() => {
              setHistory([original]);
              selectNode("start");
              setRemovalError("");
            }}
          >
            {t("editor.reset")}
          </button>
        </div>
      )}
      {request && (
        <NodeEditor
          diagram={diagram}
          request={request}
          onClose={() => setRequest(undefined)}
          onAdd={(step, originalCase) => {
            const next = applyAddition(diagram, request, step, originalCase);
            setHistory((items) => [...items, next]);
            selectNode(step.id);
            setRequest(undefined);
          }}
        />
      )}

      <div
        className="orchestration-actions"
        role="group"
        aria-label={t("viewer.views")}
      >
        <button
          type="button"
          className="ai-button is-ghost"
          aria-pressed={tab === "graph"}
          onClick={() => setTab("graph")}
        >
          {t("viewer.graph")}
        </button>
        <button
          type="button"
          className="ai-button is-ghost"
          aria-pressed={tab === "input"}
          onClick={() => setTab("input")}
        >
          {t("viewer.input")}
        </button>
      </div>
      {tab === "input" ? (
        <pre className="workflow-schema">
          {JSON.stringify(diagram.input ?? {}, null, 2)}
        </pre>
      ) : (
        <>
          <div className="workflow-workbench">
            <WorkflowGraph
              diagram={diagram}
              selected={selected}
              onSelect={selectNode}
              selectedEdge={selectedEdge}
              onSelectEdge={selectEdge}
              onAdd={setRequest}
              direction={direction}
              onDirectionChange={setDirection}
              onRemove={removeNode}
            />
            <aside
              className="workflow-inspector"
              aria-label={t(
                edge ? "viewer.edgeProperties" : "viewer.properties",
              )}
            >
              <strong>
                {t(edge ? "viewer.edgeProperties" : "viewer.properties")}
              </strong>
              <p className="orchestration-empty">{t("viewer.hint")}</p>
              <label className="orchestration-field">
                <span>{t("viewer.select")}</span>
                <select
                  value={selected}
                  onChange={(event) => selectNode(event.target.value)}
                >
                  <option value="" disabled>
                    {t("viewer.select")}
                  </option>
                  {diagram.nodes.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.id}
                    </option>
                  ))}
                </select>
              </label>
              <label className="orchestration-field">
                <span>{t("viewer.selectEdge")}</span>
                <select
                  value={selectedEdge}
                  onChange={(event) => selectEdge(event.target.value)}
                >
                  <option value="" disabled>
                    {t("viewer.selectEdge")}
                  </option>
                  {diagram.edges.map((item) => (
                    <option key={connectionId(item)} value={connectionId(item)}>
                      {t("viewer.connectionOption", {
                        from: item.from,
                        to: item.to,
                        kind: t(`viewer.edge.${item.kind}`),
                      })}
                    </option>
                  ))}
                </select>
              </label>
              {edge && edge.kind !== "data" && (
                <button
                  type="button"
                  className="ai-button is-ghost"
                  onClick={() =>
                    setRequest({ kind: "insert", edgeId: connectionId(edge) })
                  }
                >
                  {t("editor.insert")}
                </button>
              )}
              {node && node.kind !== "start" && node.kind !== "end" && (
                <div className="orchestration-actions">
                  <button
                    type="button"
                    className="ai-button is-ghost"
                    onClick={() => removeNode(node.id)}
                  >
                    {t("editor.remove")}
                  </button>
                  <button
                    type="button"
                    className="ai-button is-ghost"
                    onClick={() =>
                      setRequest({
                        kind: "sibling",
                        nodeId: node.id,
                        side: direction === "TB" ? "left" : "top",
                      })
                    }
                  >
                    {t(direction === "TB" ? "editor.addLeft" : "editor.addTop")}
                  </button>
                  <button
                    type="button"
                    className="ai-button is-ghost"
                    onClick={() =>
                      setRequest({
                        kind: "sibling",
                        nodeId: node.id,
                        side: direction === "TB" ? "right" : "bottom",
                      })
                    }
                  >
                    {t(
                      direction === "TB"
                        ? "editor.addRight"
                        : "editor.addBottom",
                    )}
                  </button>
                </div>
              )}
              {node?.draftStep && (
                <details open>
                  <summary>{t("editor.configuration")}</summary>
                  <pre className="workflow-schema">
                    {JSON.stringify(node.draftStep, null, 2)}
                  </pre>
                </details>
              )}
              {edge && (
                <dl>
                  <dt>{t("viewer.edgeType")}</dt>
                  <dd>{t(`viewer.edge.${edge.kind}`)}</dd>
                  <dt>{t("viewer.source")}</dt>
                  <dd>{edge.from}</dd>
                  <dt>{t("viewer.target")}</dt>
                  <dd>{edge.to}</dd>
                  <dt>{t("viewer.condition")}</dt>
                  <dd>
                    {when ? (
                      <dl>
                        <dt>{t("viewer.conditionSource")}</dt>
                        <dd>{when.from}</dd>
                        <dt>{t("viewer.conditionField")}</dt>
                        <dd>{when.field}</dd>
                        <dt>{t("viewer.operator")}</dt>
                        <dd>{t("viewer.equals")}</dd>
                        <dt>{t("viewer.conditionValue")}</dt>
                        <dd>
                          <pre>{JSON.stringify(when.equals, null, 2)}</pre>
                        </dd>
                      </dl>
                    ) : (
                      t(
                        edge.kind === "branch"
                          ? "viewer.conditionUnavailable"
                          : "viewer.noCondition",
                      )
                    )}
                  </dd>
                  {edge.kind === "data" && edge.from === "start" && (
                    <dd>{t("viewer.hiddenInputEdge")}</dd>
                  )}
                </dl>
              )}
              {node && (
                <dl>
                  <dt>{t("columns.id")}</dt>
                  <dd>{node.id}</dd>
                  <dt>{t("columns.type")}</dt>
                  <dd>
                    {node.kind === "start" || node.kind === "end"
                      ? t(`graph.${node.kind}`)
                      : t(`kind.${node.kind}`)}
                  </dd>
                  <dt>{t("viewer.compilation")}</dt>
                  <dd>
                    {t(node.compiled ? "viewer.compiled" : "viewer.declared")}
                  </dd>
                  {node.capability && (
                    <>
                      <dt>{t("columns.target")}</dt>
                      <dd>{node.capability}</dd>
                    </>
                  )}
                  {node.version && (
                    <>
                      <dt>{t("viewer.version")}</dt>
                      <dd>{node.version}</dd>
                    </>
                  )}
                  <dt>{t("columns.depends")}</dt>
                  <dd>{node.dependsOn?.join(", ") || t("value.none")}</dd>
                  {!!node.tools?.length && (
                    <>
                      <dt>{t("viewer.tools")}</dt>
                      <dd>
                        {node.tools.map((tool) => (
                          <code key={tool}>{tool}</code>
                        ))}
                      </dd>
                    </>
                  )}
                  {!!node.maxAttempts && (
                    <>
                      <dt>{t("viewer.attempts")}</dt>
                      <dd>{node.maxAttempts}</dd>
                    </>
                  )}
                  {node.compensation && (
                    <>
                      <dt>{t("viewer.compensation")}</dt>
                      <dd>{node.compensation}</dd>
                    </>
                  )}
                  {node.when && (
                    <>
                      <dt>{t("viewer.condition")}</dt>
                      <dd>
                        <code>
                          {t("viewer.fieldPath", {
                            from: node.when.from,
                            field: node.when.field,
                          })}
                        </code>
                        <pre>{JSON.stringify(node.when.equals, null, 2)}</pre>
                      </dd>
                    </>
                  )}
                  <dt>{t("viewer.connections")}</dt>
                  <dd>
                    <ul>
                      {diagram.edges
                        .filter(
                          (edge) =>
                            edge.from === node.id || edge.to === node.id,
                        )
                        .map((edge, index) => (
                          <li key={index}>
                            <button
                              type="button"
                              className="workflow-connection"
                              onClick={() => selectEdge(connectionId(edge))}
                            >
                              {t("viewer.connection", {
                                from: edge.from,
                                to: edge.to,
                              })}
                            </button>
                            <small>{t(`viewer.edge.${edge.kind}`)}</small>
                          </li>
                        ))}
                    </ul>
                  </dd>
                </dl>
              )}
            </aside>
          </div>
          <div className="workflow-legend">
            {(["control", "data", "branch"] as const).map((kind) => (
              <span key={kind}>
                <i className={`is-${kind}`} />
                {t(`viewer.edge.${kind}`)}
              </span>
            ))}
          </div>
          <p className="orchestration-empty">{t("viewer.projection")}</p>
        </>
      )}
    </div>
  );
}
