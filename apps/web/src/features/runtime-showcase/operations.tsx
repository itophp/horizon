import { useEffect, useState } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Choice, Panel, Status } from "./primitives";
import { Timeline } from "./views";
export function Permission() {
  const { t } = useTranslation("showcase");
  const [role, setRole] = useState("reader");
  return (
    <>
      <div className="sc-toolbar">
        <Choice
          labelKey="role"
          value={role}
          options={["reader", "editor", "admin"]}
          onChange={setRole}
        />
        <p>{t("mock")}</p>
      </div>
      <div className="sc-table-scroll">
        <table>
          <thead>
            <tr>
              {["resource", "read", "write"].map((k) => (
                <th key={k}>{t(k)}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {[
              "Application",
              "Page",
              "Provider",
              "Action",
              "Extension",
              "Runtime Context",
            ].map((n, i) => (
              <tr key={n}>
                <td>{n}</td>
                <td>
                  <Status value="allowed" />
                </td>
                <td>
                  <Status
                    value={
                      role === "admin" || (role === "editor" && i < 3)
                        ? "allowed"
                        : "blocked"
                    }
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="sc-note">{t("boundary")}</p>
    </>
  );
}
export function Execution() {
  const { t } = useTranslation("showcase");
  const [open, setOpen] = useState(false);
  const [progress, setProgress] = useState(0);
  useEffect(() => {
    if (progress < 1 || progress >= 100) return;
    const timer = setTimeout(
      () => setProgress((p) => Math.min(100, p + 20)),
      500,
    );
    return () => clearTimeout(timer);
  }, [progress]);
  return (
    <>
      <div className="sc-execution">
        <small>{t("workflowReference")}</small>
        <h2>
          {t(progress === 100 ? "complete" : progress ? "running" : "ready")}
        </h2>
        <p>{t("confirmDescription")}</p>
        <progress max="100" value={progress} />
        <strong>
          {progress}
          {t("executionUnit")}
        </strong>
        <Button
          disabled={progress > 0 && progress < 100}
          onClick={() => setOpen(true)}
        >
          {t("run")}
        </Button>
      </div>
      <Timeline />
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("confirmRun")}</DialogTitle>
            <DialogDescription>{t("confirmDescription")}</DialogDescription>
          </DialogHeader>
          <Button
            onClick={() => {
              setProgress(1);
              setOpen(false);
            }}
          >
            {t("run")}
          </Button>
        </DialogContent>
      </Dialog>
    </>
  );
}
export function Analytics() {
  const { t } = useTranslation("showcase");
  return (
    <div className="sc-grid-two">
      <Panel title={t("progress")}>
        <p className="sc-muted">
          {t("month")}
          {t("chartSeparator")}
          {t("milestones")}
        </p>
        <div className="sc-bars">
          {[42, 58, 48, 72, 65, 88, 74].map((n, i) => (
            <div key={i}>
              <span>{n}</span>
              <div style={{ height: n * 2 }} />
              <small>
                {t("weekPrefix")}
                {i + 1}
              </small>
            </div>
          ))}
        </div>
        <p>
          {t("delivered")}
          {t("deliveryRatio")}
          {t("target")}
          {t("targetValue")}
        </p>
      </Panel>
      <Panel title={t("milestones")}>
        <Timeline />
        <Link to="?engine=6">{t("detail")}</Link>
      </Panel>
    </div>
  );
}
