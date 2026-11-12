import { useTranslation } from "react-i18next";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { ErpTable, type ErpColumn } from "./erp-table";
import { purchaseOrders } from "./erp-data";
type Order = (typeof purchaseOrders)[number];
export function ErpOverview({
  metrics,
  rows,
  columns,
  changeModule,
}: {
  metrics: { key: string; value: string; delta: string }[];
  rows: Order[];
  columns: ErpColumn<Order>[];
  changeModule: (value: string) => void;
}) {
  const { t } = useTranslation("showcase");
  return (
    <>
      <div className="erp-kpis">
        {metrics.map((m) => (
          <Card key={m.key}>
            <CardHeader>
              <CardDescription>{t(m.key)}</CardDescription>
              <CardTitle>{m.value}</CardTitle>
            </CardHeader>
            <CardContent>
              <span>{m.delta}</span>
              <small>{t("month")}</small>
            </CardContent>
          </Card>
        ))}
      </div>
      <div className="erp-overview-grid">
        <Card>
          <CardHeader>
            <CardTitle>{t("erpTrend")}</CardTitle>
            <CardDescription>{t("erpWeek")}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="erp-chart-legend">
              <span>{t("erpSales")}</span>
              <span>{t("erpPurchases")}</span>
            </div>
            <svg
              viewBox="0 0 640 200"
              className="erp-chart"
              role="img"
              aria-label={t("erpTrend")}
            >
              <title>{t("erpTrend")}</title>
              {[0, 50, 100, 150].map((n) => (
                <g key={n}>
                  <line
                    x1="35"
                    x2="620"
                    y1={175 - n}
                    y2={175 - n}
                    stroke="var(--border)"
                  />
                  <text
                    x="2"
                    y={180 - n}
                    fontSize="10"
                    fill="var(--muted-foreground)"
                  >
                    {n * 2}
                  </text>
                </g>
              ))}
              {[
                ["var(--app-green)", [56, 72, 65, 98, 115, 142]],
                ["var(--muted-foreground)", [32, 50, 45, 68, 76, 92]],
              ].map(([color, values]) => (
                <polyline
                  key={String(color)}
                  points={(values as number[])
                    .map((v, i) => `${40 + i * 112},${175 - v}`)
                    .join(" ")}
                  fill="none"
                  stroke={String(color)}
                  strokeWidth="3"
                />
              ))}
            </svg>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("erpTodos")}</CardTitle>
            <CardDescription>{t("pendingReview")}</CardDescription>
          </CardHeader>
          <CardContent>
            {[
              ["erpTaskReceipt", "erpProcurement", 4],
              ["erpTaskPlan", "erpProduction", 3],
              ["erpTaskFinance", "erpFinance", 2],
            ].map(([label, target, count]) => (
              <Button
                key={label}
                variant="ghost"
                className="erp-todo"
                onClick={() => changeModule(String(target))}
              >
                <span>{t(String(label))}</span>
                <Badge variant="secondary">{count}</Badge>
              </Button>
            ))}
            <div className="erp-fulfillment">
              <h3>{t("erpFulfillment")}</h3>
              {["erpPlanned", "erpCompleted", "erpArrived"].map((key, i) => (
                <div key={key}>
                  <span>{t(key)}</span>
                  <Progress aria-label={t(key)} value={[92, 76, 68][i]} />
                  <strong>
                    {[92, 76, 68][i]}
                    {t("progressUnit")}
                  </strong>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
      <div className="erp-section-heading">
        <h2>{t("erpExceptions")}</h2>
        <Badge variant="outline">{t("records", { count: 4 })}</Badge>
      </div>
      <div className="erp-exceptions">
        {[
          ["erpRestock", "erpInventory", "erpHigh"],
          ["erpLateDelivery", "erpProcurement", "erpHigh"],
          ["erpQualityIssue", "erpProcurement", "erpMedium"],
          ["erpInvoiceIssue", "erpFinance", "erpMedium"],
        ].map(([label, target, priority]) => (
          <Button
            key={label}
            variant="outline"
            onClick={() => changeModule(target)}
          >
            <span className="erp-exception-dot" />
            <span>{t(label)}</span>
            <Badge variant="secondary">{t(priority)}</Badge>
          </Button>
        ))}
      </div>
      <div className="erp-section-heading">
        <h2>{t("erpRecentDocs")}</h2>
        <Button variant="ghost" onClick={() => changeModule("erpProcurement")}>
          {t("detail")}
        </Button>
      </div>
      <ErpTable rows={rows.slice(0, 4)} columns={columns} />
    </>
  );
}
