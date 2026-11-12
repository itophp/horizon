import { ErpOverview } from "./erp-overview";
import { useMemo, useState } from "react";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { Progress } from "@/components/ui/progress";
import { Choice } from "./primitives";
import { purchaseOrders, stock } from "./erp-data";
import { ErpTable, money } from "./erp-table";
import { ErpDocument } from "./erp-document";
import { ErpInventory, ErpProduction, ErpFinance } from "./erp-operations";
const modules = [
  "erpOverview",
  "erpProcurement",
  "erpInventory",
  "erpProduction",
  "erpFinance",
];
export function ErpWorkspace() {
  const { t, i18n } = useTranslation("showcase");
  const locale = i18n.resolvedLanguage ?? "en";
  const [q, setQ] = useSearchParams();
  const module = modules.includes(q.get("erpView") ?? "")
    ? q.get("erpView")!
    : "erpOverview";
  const [warehouse, setWarehouse] = useState("erpAllWarehouses");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [received, setReceived] = useState<string[]>([]);
  const [selected, setSelected] = useState("");
  const [notice, setNotice] = useState(false);
  const rows = useMemo(
    () =>
      purchaseOrders.map((r) =>
        received.includes(r.id)
          ? { ...r, status: "erpArrived", received: r.quantity }
          : r,
      ),
    [received],
  );
  const filtered = rows.filter(
    (r) =>
      (warehouse === "erpAllWarehouses" || r.warehouse === warehouse) &&
      (status === "all" || r.status === status) &&
      (r.id + r.material + r.supplier)
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const order = rows.find((r) => r.id === selected) ?? null;
  function changeModule(value: string) {
    const next = new URLSearchParams(q);
    next.set("erpView", value);
    setQ(next);
    setSearch("");
    setStatus("all");
  }
  const pendingValue = rows.reduce(
    (sum, r) => sum + (r.quantity - r.received) * r.price,
    0,
  );
  const compact = (value: number) =>
    new Intl.NumberFormat(locale, {
      style: "currency",
      currency: "CNY",
      notation: "compact",
      maximumFractionDigits: 2,
    }).format(value);
  const metrics = [
    { key: "erpRevenue", value: compact(1826400), delta: "+12.8%" },
    {
      key: "erpPurchaseAmount",
      value: compact(pendingValue),
      delta: t("records", {
        count: rows.filter((r) => r.status !== "erpArrived").length,
      }),
    },
    { key: "erpStockValue", value: compact(3648200), delta: "+6.2%" },
    { key: "erpDeliveryRate", value: "96.4%", delta: "+2.1%" },
    { key: "erpOutput", value: "12,860", delta: "+18.5%" },
    { key: "erpCash", value: compact(421000), delta: t("erpTerms") },
  ];
  const columns = [
    {
      id: "id",
      label: "erpDocument",
      cell: (r: (typeof purchaseOrders)[number]) => (
        <Button
          className="erp-document-link"
          variant="link"
          onClick={() => setSelected(r.id)}
        >
          {r.id}
        </Button>
      ),
    },
    {
      id: "supplier",
      label: "erpSupplier",
      cell: (r: (typeof purchaseOrders)[number]) => (
        <div className="erp-object">
          <strong>{r.supplier}</strong>
          <small>{r.material}</small>
        </div>
      ),
    },
    {
      id: "amount",
      label: "erpTotal",
      cell: (r: (typeof purchaseOrders)[number]) =>
        money(r.price * r.quantity, locale),
    },
    {
      id: "received",
      label: "erpArrived",
      cell: (r: (typeof purchaseOrders)[number]) => (
        <div className="erp-cell-progress">
          <span>
            {r.received}
            {t("erpOf")}
            {r.quantity}
          </span>
          <Progress
            value={Math.round((r.received / r.quantity) * 100)}
            aria-label={r.id}
          />
        </div>
      ),
    },
    {
      id: "status",
      label: "status",
      cell: (r: (typeof purchaseOrders)[number]) => (
        <Badge
          variant="secondary"
          className={r.status === "erpOverdue" ? "erp-warning" : ""}
        >
          {t(r.status)}
        </Badge>
      ),
    },
    {
      id: "due",
      label: "erpDue",
      cell: (r: (typeof purchaseOrders)[number]) => r.due,
    },
  ];
  return (
    <>
      <Tabs value={module} onValueChange={(v) => changeModule(String(v))}>
        <TabsList variant="line" className="erp-view-tabs">
          {modules.map((key) => (
            <TabsTrigger value={key} key={key}>
              {t(key)}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      <div className="erp-context">
        <Badge variant="outline">{t("erpIntro")}</Badge>
        <span>{t("updated")}</span>
      </div>
      {notice && (
        <Alert>
          <AlertTitle>{t("success")}</AlertTitle>
          <AlertDescription>{t("erpReceiptSuccess")}</AlertDescription>
          <Button variant="ghost" onClick={() => setNotice(false)}>
            {t("close")}
          </Button>
        </Alert>
      )}
      {module === "erpOverview" ? (
        <ErpOverview
          metrics={metrics}
          rows={rows}
          columns={columns}
          changeModule={changeModule}
        />
      ) : (
        <>
          {["erpProcurement", "erpInventory"].includes(module) && (
            <div className="erp-toolbar">
              <Input
                aria-label={t("search")}
                placeholder={t("search")}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
              <Choice
                labelKey="erpWarehouse"
                value={warehouse}
                options={[
                  "erpAllWarehouses",
                  "erpRawWarehouse",
                  "erpFinishedWarehouse",
                ]}
                onChange={setWarehouse}
              />
              {module === "erpProcurement" && (
                <Choice
                  labelKey="status"
                  value={status}
                  options={["all", "erpAwaiting", "erpArrived", "erpOverdue"]}
                  onChange={setStatus}
                />
              )}
              <span>
                {t("records", {
                  count:
                    module === "erpProcurement"
                      ? filtered.length
                      : stock.filter(
                          (r) =>
                            (warehouse === "erpAllWarehouses" ||
                              r.warehouse === warehouse) &&
                            (r.name + r.id)
                              .toLowerCase()
                              .includes(search.toLowerCase()),
                        ).length,
                })}
              </span>
            </div>
          )}
          {module === "erpProcurement" && (
            <>
              <ErpTable rows={filtered} columns={columns} />
              <div className="erp-supplier-performance">
                <h2>{t("erpSupplierPerformance")}</h2>
                {purchaseOrders.slice(0, 4).map((r, i) => (
                  <div key={r.id}>
                    <strong>{r.supplier}</strong>
                    <Progress
                      value={[82, 96, 91, 98][i]}
                      aria-label={r.supplier}
                    />
                    <span>
                      {[82, 96, 91, 98][i]}
                      {t("progressUnit")}
                    </span>
                  </div>
                ))}
              </div>
            </>
          )}
          {module === "erpInventory" && (
            <ErpInventory warehouse={warehouse} search={search} />
          )}{" "}
          {module === "erpProduction" && <ErpProduction />}
          {module === "erpFinance" && <ErpFinance />}
        </>
      )}
      <ErpDocument
        order={order}
        onClose={() => setSelected("")}
        onReceive={(id) => {
          setReceived([...received, id]);
          setNotice(true);
        }}
      />
    </>
  );
}
