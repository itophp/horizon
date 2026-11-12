import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";

import { Panel, Status } from "./primitives";
import { projects } from "./catalog";
export function Catalog({
  engine,
  onOpen,
}: {
  engine: number;
  onOpen: (name: string) => void;
}) {
  const { t } = useTranslation("showcase");
  const [query, setQuery] = useState("");
  const names =
    engine === 2
      ? [
          "Text / Input",
          "Status / Badge",
          "Reference / Combobox",
          "Date / DateTime",
          "Markdown / RichText",
          "Progress / Task",
        ]
      : engine === 1
        ? [
            "Portfolio / List",
            "Project / Detail",
            "Launch / Form",
            "Operations / Custom",
            "Settings / Form",
            "Review / Master-detail",
          ]
        : engine === 12
          ? [
              "Articles UI",
              "Project workspace",
              "Drive resources",
              "Workflow tools",
              "Semantic adapter",
              "Notification shell",
            ]
          : [
              "Atlas workspace",
              "Operations suite",
              "Knowledge hub",
              "Customer portal",
              "Delivery console",
              "Service platform",
            ];
  return (
    <>
      <div className="sc-toolbar">
        <Input
          aria-label={t("search")}
          placeholder={t("search")}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <span className="sc-muted">
          {t("records", { count: names.length })}
        </span>
      </div>
      <div className="sc-catalog">
        {names
          .filter((n) => n.toLowerCase().includes(query.toLowerCase()))
          .map((name, i) => (
            <article key={name}>
              <div className="sc-catalog-icon">
                {String(i + 1).padStart(2, "0")}
              </div>
              <Status value={i === 2 ? "review" : "installed"} />
              <h2>{name}</h2>
              <p>{t(`engines.${engine}.description`)}</p>
              <dl>
                <dt>{t("version")}</dt>
                <dd>
                  {t("versionPrefix")}
                  {i}
                </dd>
                <dt>{t("compatibility")}</dt>
                <dd>{t("runtimeVersion")}</dd>
              </dl>
              <Button variant="outline" onClick={() => onOpen(name)}>
                {t("detail")}
              </Button>
            </article>
          ))}
      </div>
    </>
  );
}
export function Detail({ name, onEdit }: { name: string; onEdit: () => void }) {
  const { t } = useTranslation("showcase");
  return (
    <>
      <div className="sc-detail-hero">
        <div>
          <Status value="active" />
          <h2>{name}</h2>
          <p>{t("detailsText")}</p>
        </div>
        <Button onClick={onEdit}>{t("editAction")}</Button>
      </div>
      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">{t("overview")}</TabsTrigger>
          <TabsTrigger value="activity">{t("activity")}</TabsTrigger>
        </TabsList>
        <TabsContent value="overview">
          <div className="sc-grid-two">
            <Panel title={t("progress")}>
              <div className="sc-big-number">
                {t("progressValue")}
                <span>{t("progressUnit")}</span>
              </div>
              <progress max="100" value="72" />
              <p>
                {t("milestones")}
                {t("milestoneCount")}
              </p>
            </Panel>
            <Panel title={t("health")}>
              <dl className="sc-definition">
                <dt>{t("owner")}</dt>
                <dd>{t("sampleOwner")}</dd>
                <dt>{t("budget")}</dt>
                <dd>{t("sampleBudget")}</dd>
                <dt>{t("date")}</dt>
                <dd>{t("sampleDate")}</dd>
                <dt>{t("version")}</dt>
                <dd>{t("sampleVersion")}</dd>
              </dl>
            </Panel>
          </div>
        </TabsContent>
        <TabsContent value="activity">
          <Timeline />
        </TabsContent>
      </Tabs>
      <details>
        <summary>{t("expand")}</summary>
        <pre>
          {JSON.stringify(
            { schemaVersion: "3.0", source: "showcase:local", name },
            null,
            2,
          )}
        </pre>
      </details>
    </>
  );
}
export function Timeline() {
  const { t } = useTranslation("showcase");
  return (
    <ol className="sc-timeline">
      {["queued", "validate", "execute", "result"].map((s, i) => (
        <li key={s}>
          <span className="sc-event-dot" />
          <div>
            <strong>{t(s)}</strong>
            <p>{t("detailsText")}</p>
            <small>
              {t("eventTimePrefix")}
              {10 + i * 5}
              {t("eventTimeSuffix")}
            </small>
          </div>
          <Status value={i === 3 ? "review" : "done"} />
        </li>
      ))}
    </ol>
  );
}
export function Workbench({ engine }: { engine: number }) {
  const { t } = useTranslation("showcase");
  const [selected, setSelected] = useState(0);
  return (
    <div className="sc-workbench">
      <aside>
        {projects.slice(0, 6).map((name, i) => (
          <Button
            variant={i === selected ? "secondary" : "ghost"}
            key={name}
            onClick={() => setSelected(i)}
          >
            <span>{name}</span>
            <Status value={i % 2 ? "review" : "active"} />
          </Button>
        ))}
      </aside>
      <section>
        <div className="sc-detail-hero">
          <div>
            <small>{t(engine === 11 ? "source" : "overview")}</small>
            <h2>{projects[selected]}</h2>
          </div>
          <Status value="active" />
        </div>
        {engine === 10 ? (
          <>
            <p>{t("boundary")}</p>
            <Panel title={t("references")}>
              <pre>
                {JSON.stringify(
                  {
                    applicationId: "showcase",
                    pageType: "detail",
                    selection: [
                      { entityRef: "project", recordId: `mock-${selected}` },
                    ],
                    availableActions: ["preview"],
                    source: "showcase:local",
                  },
                  null,
                  2,
                )}
              </pre>
            </Panel>
          </>
        ) : (
          <>
            <Panel title={t("progress")}>
              <div className="sc-big-number">
                {68 + selected * 4}
                <span>{t("workProgressUnit")}</span>
              </div>
              <progress max="100" value={68 + selected * 4} />
            </Panel>
            <Timeline />
          </>
        )}
      </section>
    </div>
  );
}
export { Permission, Execution, Analytics } from "./operations";
