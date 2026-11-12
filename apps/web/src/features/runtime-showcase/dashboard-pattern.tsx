import { useState } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Progress } from "@/components/ui/progress";
import { Choice, Status } from "./primitives";
import { projects } from "./catalog";
const metrics = [
  { key: "members", value: "128", delta: "+12.4%" },
  { key: "pendingTasks", value: "24", delta: "−8.2%" },
  { key: "onTime", value: "94.6%", delta: "+3.1%" },
  { key: "monthlyBudget", value: "¥ 286,400", delta: "+6.8%" },
];
export function DashboardPattern({
  analytics = false,
}: {
  analytics?: boolean;
}) {
  const { t } = useTranslation("showcase");
  const [range, setRange] = useState("weekly");
  const [chart, setChart] = useState("trend");
  const values =
    range === "weekly"
      ? [42, 58, 48, 72, 65, 88, 74]
      : [32, 42, 48, 38, 58, 55, 64, 72, 65, 88, 74, 94];
  return (
    <>
      <div className="sc-metric-grid">
        {metrics.map((m) => (
          <Card key={m.key}>
            <CardHeader>
              <CardDescription>{t(m.key)}</CardDescription>
              <CardTitle className="sc-metric-value">{m.value}</CardTitle>
            </CardHeader>
            <CardContent>
              <span className="sc-positive">{m.delta}</span>
              <small className="sc-muted">{t("month")}</small>
            </CardContent>
          </Card>
        ))}
      </div>
      <div className="sc-report-grid">
        <Card>
          <CardHeader>
            <div className="sc-chart-heading">
              <div>
                <CardTitle>{t("trend")}</CardTitle>
                <CardDescription>{t("updated")}</CardDescription>
              </div>
              <Choice
                labelKey="date"
                value={range}
                options={["weekly", "monthly"]}
                onChange={setRange}
              />
            </div>
            <Tabs value={chart} onValueChange={(v) => setChart(String(v))}>
              <TabsList variant="line">
                <TabsTrigger value="trend">{t("trend")}</TabsTrigger>
                <TabsTrigger value="compareSummary">
                  {t("compareSummary")}
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </CardHeader>
          <CardContent>
            {chart === "trend" ? (
              <svg
                className="sc-trend-chart"
                viewBox="0 0 700 240"
                role="img"
                aria-label={t("trend")}
              >
                <title>{t("trend")}</title>
                {[0, 25, 50, 75, 100].map((n) => (
                  <g key={n}>
                    <line
                      x1="35"
                      x2="680"
                      y1={215 - n * 1.8}
                      y2={215 - n * 1.8}
                      stroke="var(--border)"
                    />
                    <text
                      x="5"
                      y={220 - n * 1.8}
                      fill="var(--muted-foreground)"
                      fontSize="11"
                    >
                      {n}
                    </text>
                  </g>
                ))}
                <polyline
                  points={values
                    .map(
                      (v, i) =>
                        `${40 + (i * 630) / (values.length - 1)},${215 - v * 1.8}`,
                    )
                    .join(" ")}
                  fill="none"
                  stroke="var(--app-green)"
                  strokeWidth="3"
                />
                {values.map((v, i) => (
                  <g key={i}>
                    <circle
                      cx={40 + (i * 630) / (values.length - 1)}
                      cy={215 - v * 1.8}
                      r="4"
                      fill="var(--app-green)"
                    />
                    <text
                      x={40 + (i * 630) / (values.length - 1)}
                      y={200 - v * 1.8}
                      textAnchor="middle"
                      fill="var(--foreground)"
                      fontSize="11"
                    >
                      {v}
                    </text>
                  </g>
                ))}
              </svg>
            ) : (
              <div className="sc-bars">
                {values.map((v, i) => (
                  <div key={i}>
                    <span>{v}</span>
                    <div style={{ height: v * 2 }} />
                    <small>
                      {t("weekPrefix")}
                      {i + 1}
                    </small>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>
              {t(analytics ? "distribution" : "pendingReview")}
            </CardTitle>
            <CardDescription>{t("month")}</CardDescription>
          </CardHeader>
          <CardContent>
            {["active", "review", "done"].map((key, i) => (
              <div className="sc-metric-row" key={key}>
                <Status value={key} />
                <strong>{[48, 24, 72][i]}</strong>
                <Progress aria-label={t(key)} value={[33, 17, 50][i]} />
              </div>
            ))}
            <Link className="sc-inline-link" to="?pattern=reviewDesk">
              {t("detail")}
            </Link>
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>
            {t(analytics ? "compareSummary" : "teamActivity")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="sc-dashboard-tasks">
            {projects.slice(0, 4).map((name, i) => (
              <Link key={name} to={`?pattern=recordDetail&record=${i}`}>
                <strong>{name}</strong>
                <span>{t("detailsText")}</span>
                <Status value={i % 2 ? "review" : "active"} />
              </Link>
            ))}
          </div>
        </CardContent>
      </Card>
    </>
  );
}
