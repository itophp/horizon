import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { Status } from "./primitives";
import { projects } from "./catalog";
import { Timeline } from "./views";
export function ReviewPattern() {
  const { t } = useTranslation("showcase");
  const [selected, setSelected] = useState(0);
  const [decisions, setDecisions] = useState<Record<number, string>>({});
  const [action, setAction] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState(false);
  const names = projects.slice(0, 5);
  return (
    <div className="sc-pattern-split sc-review-split">
      <aside className="sc-filter-nav">
        <h2>{t("pendingReview")}</h2>
        {names.map((name, i) => (
          <Button
            key={name}
            variant={selected === i ? "secondary" : "ghost"}
            onClick={() => setSelected(i)}
          >
            <span>
              {name}
              <small>{["Alex Lee", "林晓", "Mika Kim"][i % 3]}</small>
            </span>
            <Status value={decisions[i] ?? "review"} />
          </Button>
        ))}
      </aside>
      <section className="sc-review-detail">
        <div className="sc-detail-hero">
          <div>
            <Status value={decisions[selected] ?? "review"} />
            <h2>{names[selected]}</h2>
            <p>{t("reviewDescription")}</p>
          </div>
          <Avatar>
            <AvatarFallback>{["AL", "林", "MK"][selected % 3]}</AvatarFallback>
          </Avatar>
        </div>
        <dl className="sc-definition">
          <dt>{t("requester")}</dt>
          <dd>{["Alex Lee", "林晓", "Mika Kim"][selected % 3]}</dd>
          <dt>{t("date")}</dt>
          <dd>{t("sampleDate")}</dd>
          <dt>{t("requestChange")}</dt>
          <dd>{t("approvalAmount")}</dd>
          <dt>{t("impact")}</dt>
          <dd>{t("approvalImpact")}</dd>
        </dl>
        <div className="sc-panel">
          <h2>{t("milestones")}</h2>
          <Progress value={72} aria-label={t("progress")} />
          <Timeline />
        </div>
        <footer className="sc-review-actions">
          <Button
            variant="outline"
            disabled={!!decisions[selected]}
            onClick={() => {
              setAction("reject");
              setError(false);
              setNote("");
            }}
          >
            {t("reject")}
          </Button>
          <Button
            disabled={!!decisions[selected]}
            onClick={() => {
              setAction("approve");
              setError(false);
              setNote("");
            }}
          >
            {t("approve")}
          </Button>
        </footer>
      </section>
      <Dialog
        open={!!action}
        onOpenChange={(v) => {
          if (!v) setAction("");
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(action || "confirm")}</DialogTitle>
            <DialogDescription>{names[selected]}</DialogDescription>
          </DialogHeader>
          <label className="sc-field">
            {t("reason")}
            <Textarea value={note} onChange={(e) => setNote(e.target.value)} />
          </label>
          {error && <p role="alert">{t("decisionNeeded")}</p>}
          <Button
            onClick={() => {
              if (note.trim().length < 3) {
                setError(true);
                return;
              }
              setDecisions({
                ...decisions,
                [selected]: action === "approve" ? "done" : "blocked",
              });
              setAction("");
            }}
          >
            {t("confirm")}
          </Button>
        </DialogContent>
      </Dialog>
    </div>
  );
}
