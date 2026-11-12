import { useTranslation } from "react-i18next";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { stock, workOrders, purchaseOrders } from "./erp-data";
import { ErpTable, money } from "./erp-table";
export function ErpInventory({
  warehouse,
  search,
}: {
  warehouse: string;
  search: string;
}) {
  const { t } = useTranslation("showcase");
  const rows = stock.filter(
    (r) =>
      (warehouse === "erpAllWarehouses" || r.warehouse === warehouse) &&
      (r.name + r.id).toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <>
      <div className="erp-section-heading">
        <h2>{t("erpCatalog")}</h2>
        <Badge variant="outline">{t("records", { count: rows.length })}</Badge>
      </div>
      <ErpTable
        rows={rows}
        columns={[
          {
            id: "name",
            label: "erpMaterial",
            cell: (r) => (
              <div className="erp-object">
                <strong>{r.name}</strong>
                <small>
                  {r.id}
                  {t("erpSeparator")}
                  {r.location}
                </small>
              </div>
            ),
          },
          {
            id: "warehouse",
            label: "erpWarehouse",
            cell: (r) => t(r.warehouse),
          },
          {
            id: "available",
            label: "erpAvailable",
            cell: (r) => (
              <strong className={r.available < r.safety ? "erp-warning" : ""}>
                {r.available}
              </strong>
            ),
          },
          { id: "reserved", label: "erpReserved", cell: (r) => r.reserved },
          { id: "safety", label: "erpSafety", cell: (r) => r.safety },
          {
            id: "status",
            label: "status",
            cell: (r) => (
              <Badge variant="secondary">
                {t(r.available < r.safety ? "erpWarning" : "erpQualified")}
              </Badge>
            ),
          },
        ]}
      />
      <div className="erp-stock-summary">
        {rows
          .filter((r) => r.available < r.safety)
          .map((r) => (
            <Card key={r.id}>
              <CardHeader>
                <CardDescription>{t("erpStockAlerts")}</CardDescription>
                <CardTitle>{r.name}</CardTitle>
              </CardHeader>
              <CardContent>
                <Progress
                  aria-label={r.name}
                  value={Math.round((r.available / r.safety) * 100)}
                />
                <p>
                  {r.available}
                  {t("erpOf")}
                  {r.safety}
                </p>
              </CardContent>
            </Card>
          ))}
      </div>
    </>
  );
}
export function ErpProduction() {
  const { t } = useTranslation("showcase");
  return (
    <>
      <div className="erp-lines">
        {["erpLineA", "erpLineB", "erpLineC"].map((key, i) => (
          <Card key={key}>
            <CardHeader>
              <CardDescription>{t("erpShift")}</CardDescription>
              <CardTitle>{t(key)}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="erp-capacity">
                <span>{t("erpCapacity")}</span>
                <strong>
                  {[86, 72, 94][i]}
                  {t("progressUnit")}
                </strong>
              </div>
              <Progress value={[86, 72, 94][i]} aria-label={t(key)} />
              <div className="erp-schedule">
                {[0, 1, 2, 3, 4, 5, 6, 7].map((n) => (
                  <span key={n} className={n < 6 - i ? "erp-slot-busy" : ""} />
                ))}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
      <div className="erp-section-heading">
        <h2>{t("erpProduction")}</h2>
        <Badge variant="outline">{t("erpShift")}</Badge>
      </div>
      <ErpTable
        rows={workOrders}
        columns={[
          {
            id: "id",
            label: "erpOrder",
            cell: (r) => (
              <div className="erp-object">
                <strong>{r.id}</strong>
                <small>{r.product}</small>
              </div>
            ),
          },
          { id: "line", label: "erpLine", cell: (r) => t(r.line) },
          { id: "planned", label: "erpPlanned", cell: (r) => r.planned },
          {
            id: "completed",
            label: "erpCompleted",
            cell: (r) => (
              <div className="erp-cell-progress">
                <span>
                  {r.completed}
                  {t("erpOf")}
                  {r.planned}
                </span>
                <Progress
                  value={Math.round((r.completed / r.planned) * 100)}
                  aria-label={r.id}
                />
              </div>
            ),
          },
          {
            id: "ready",
            label: "status",
            cell: (r) => (
              <Badge variant="secondary">
                {t(r.ready ? "erpReady" : "erpShortage")}
              </Badge>
            ),
          },
          { id: "due", label: "erpDue", cell: (r) => r.due },
        ]}
      />
    </>
  );
}
export function ErpFinance() {
  const { t, i18n } = useTranslation("showcase");
  const locale = i18n.resolvedLanguage ?? "en";
  return (
    <>
      <div className="erp-aging">
        <h2>{t("erpAging")}</h2>
        {["0–30", "31–60", "61–90", "90+"].map((range, i) => (
          <div key={range}>
            <span>{range}</span>
            <Progress value={[68, 20, 9, 3][i]} aria-label={range} />
            <strong>{money([286400, 84200, 37800, 12600][i], locale)}</strong>
          </div>
        ))}
      </div>
      <div className="erp-section-heading">
        <h2>{t("erpReconciliation")}</h2>
        <Badge variant="outline">{t("erpCurrency")}</Badge>
      </div>
      <ErpTable
        rows={purchaseOrders.slice(0, 5)}
        columns={[
          {
            id: "id",
            label: "erpDocument",
            cell: (r) => r.id.replace("PO", "AP"),
          },
          { id: "supplier", label: "erpSupplier", cell: (r) => r.supplier },
          {
            id: "amount",
            label: "erpInvoice",
            cell: (r) => money(r.quantity * r.price, locale),
          },
          {
            id: "difference",
            label: "erpDifference",
            cell: (r) => (
              <strong className={r.id.endsWith("018") ? "erp-warning" : ""}>
                {money(r.id.endsWith("018") ? 1280 : 0, locale)}
              </strong>
            ),
          },
          { id: "due", label: "erpDue", cell: (r) => r.due },
          {
            id: "status",
            label: "status",
            cell: (r) => (
              <Badge variant="secondary">
                {t(r.id.endsWith("018") ? "erpPendingMatch" : "erpMatched")}
              </Badge>
            ),
          },
        ]}
      />
    </>
  );
}
