import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { purchaseOrders } from "./erp-data";
import { ErpTable, money } from "./erp-table";
export function ErpDocument({
  order,
  onClose,
  onReceive,
}: {
  order: (typeof purchaseOrders)[number] | null;
  onClose: () => void;
  onReceive: (id: string) => void;
}) {
  const { t, i18n } = useTranslation("showcase");
  const [confirm, setConfirm] = useState(false);
  const locale = i18n.resolvedLanguage ?? "en";
  return (
    <Sheet
      open={!!order}
      onOpenChange={(value) => {
        if (!value) onClose();
      }}
    >
      <SheetContent showCloseButton={false} className="erp-document">
        <SheetHeader>
          <SheetDescription>{t("erpPurchaseDetails")}</SheetDescription>
          <SheetTitle>{order?.id}</SheetTitle>
        </SheetHeader>
        {order && (
          <>
            <div className="erp-document-summary">
              <div>
                <h2>{order.supplier}</h2>
                <Badge variant="secondary">{t(order.status)}</Badge>
              </div>
              <strong>{money(order.quantity * order.price, locale)}</strong>
            </div>
            <dl className="sc-definition">
              <dt>{t("erpBuyer")}</dt>
              <dd>{order.buyer}</dd>
              <dt>{t("erpDue")}</dt>
              <dd>{order.due}</dd>
              <dt>{t("erpPaymentTerms")}</dt>
              <dd>{t("erpTerms")}</dd>
              <dt>{t("erpWarehouse")}</dt>
              <dd>{t(order.warehouse)}</dd>
            </dl>
            <div className="erp-receipt-progress">
              <span>{t("erpArrived")}</span>
              <strong>
                {order.received}
                {t("erpOf")}
                {order.quantity}
              </strong>
              <Progress
                aria-label={t("erpArrived")}
                value={Math.round((order.received / order.quantity) * 100)}
              />
            </div>
            <Tabs defaultValue="lines">
              <TabsList variant="line">
                <TabsTrigger value="lines">{t("erpLines")}</TabsTrigger>
                <TabsTrigger value="flow">{t("erpFlow")}</TabsTrigger>
                <TabsTrigger value="attachments">
                  {t("erpAttachments")}
                </TabsTrigger>
              </TabsList>
              <TabsContent value="lines">
                <ErpTable
                  rows={[order]}
                  columns={[
                    {
                      id: "material",
                      label: "erpMaterial",
                      cell: (r) => r.material,
                    },
                    {
                      id: "quantity",
                      label: "erpQuantity",
                      cell: (r) => r.quantity,
                    },
                    {
                      id: "price",
                      label: "erpUnitPrice",
                      cell: (r) => money(r.price, locale),
                    },
                    {
                      id: "amount",
                      label: "erpTotal",
                      cell: (r) => money(r.price * r.quantity, locale),
                    },
                  ]}
                />
                <div className="erp-document-total">
                  <span>{t("erpCurrency")}</span>
                  <strong>{money(order.quantity * order.price, locale)}</strong>
                </div>
              </TabsContent>
              <TabsContent value="flow">
                <ol className="sc-timeline">
                  {["erpCreated", "erpApproved", "erpConfirmed"].map(
                    (key, i) => (
                      <li key={key}>
                        <span className="sc-event-dot" />
                        <div>
                          <strong>{t(key)}</strong>
                          <p>{order.buyer}</p>
                          <small>
                            {
                              [
                                "2026-11-01 09:20",
                                "2026-11-01 14:35",
                                "2026-11-02 10:10",
                              ][i]
                            }
                          </small>
                        </div>
                      </li>
                    ),
                  )}
                </ol>
              </TabsContent>
              <TabsContent value="attachments">
                {["erpContract", "erpInspection", "erpShipment"].map((key) => (
                  <div className="erp-file" key={key}>
                    <strong>{t(key)}</strong>
                    <Badge variant="outline">{t("mock")}</Badge>
                  </div>
                ))}
              </TabsContent>
            </Tabs>
            <footer className="erp-document-actions">
              <Button variant="outline" onClick={onClose}>
                {t("close")}
              </Button>
              <Button
                disabled={order.status === "erpArrived"}
                onClick={() => setConfirm(true)}
              >
                {t("erpConfirmReceipt")}
              </Button>
            </footer>
            <Dialog open={confirm} onOpenChange={setConfirm}>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{t("erpConfirmReceipt")}</DialogTitle>
                  <DialogDescription>{t("erpReceiptHelp")}</DialogDescription>
                </DialogHeader>
                <p>{order.id}</p>
                <Button
                  onClick={() => {
                    onReceive(order.id);
                    setConfirm(false);
                  }}
                >
                  {t("confirm")}
                </Button>
              </DialogContent>
            </Dialog>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
