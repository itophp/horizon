import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Slider } from "@/components/ui/slider";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from "@/components/ui/accordion";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
const schema = z.object({
  name: z.string().min(3),
  retention: z.number().min(1).max(365),
});
export function SettingsPattern() {
  const { t } = useTranslation("showcase");
  const [section, setSection] = useState("general");
  const [digest, setDigest] = useState(true);
  const [alerts, setAlerts] = useState(true);
  const [threshold, setThreshold] = useState(75);
  const [mode, setMode] = useState("manual");
  const [saved, setSaved] = useState(false);
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm({
    resolver: zodResolver(schema),
    defaultValues: { name: "Atlas workspace", retention: 90 },
  });
  return (
    <div className="sc-pattern-split">
      <aside className="sc-filter-nav">
        {["general", "deliveryOptions", "advanced"].map((key) => (
          <Button
            variant={section === key ? "secondary" : "ghost"}
            key={key}
            onClick={() => {
              setSection(key);
              setSaved(false);
            }}
          >
            {t(key)}
          </Button>
        ))}
      </aside>
      <form
        className="sc-settings-form"
        onSubmit={handleSubmit(() => setSaved(true))}
      >
        <h2>{t(section)}</h2>
        <p className="sc-muted">{t("settingsDescription")}</p>
        {section === "general" && (
          <>
            <label className="sc-field">
              {t("workspaceName")}
              <Input {...register("name")} aria-invalid={!!errors.name} />
              {errors.name && <span role="alert">{t("valid")}</span>}
            </label>
            <label className="sc-field">
              {t("retention")}
              <Input
                type="number"
                {...register("retention", { valueAsNumber: true })}
              />
              {errors.retention && <span role="alert">{t("required")}</span>}
            </label>
          </>
        )}
        {section === "deliveryOptions" && (
          <>
            {[
              ["dailyDigest", digest, setDigest],
              ["failureAlerts", alerts, setAlerts],
            ].map(([key, value, setter]) => (
              <label className="sc-setting-row" key={String(key)}>
                <span>{t(String(key))}</span>
                <Switch
                  checked={Boolean(value)}
                  onCheckedChange={setter as (v: boolean) => void}
                />
              </label>
            ))}
            <label className="sc-field">
              {t("notificationThreshold")}
              <strong>{threshold}</strong>
              <Slider
                aria-label={t("notificationThreshold")}
                value={[threshold]}
                onValueChange={(v) => setThreshold(Array.isArray(v) ? v[0] : v)}
              />
            </label>
          </>
        )}
        {section === "advanced" && (
          <Accordion defaultValue={["policy"]}>
            <AccordionItem value="policy">
              <AccordionTrigger>{t("policyMode")}</AccordionTrigger>
              <AccordionContent>
                <RadioGroup
                  value={mode}
                  onValueChange={(v) => setMode(String(v))}
                >
                  {["manual", "automatic"].map((key) => (
                    <label className="sc-check" key={key}>
                      <RadioGroupItem value={key} />
                      {t(key)}
                    </label>
                  ))}
                </RadioGroup>
                <p className="sc-muted">{t("localOnly")}</p>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        )}
        {saved && (
          <Alert>
            <AlertTitle>{t("success")}</AlertTitle>
            <AlertDescription>{t("saved")}</AlertDescription>
          </Alert>
        )}
        <footer className="sc-pagination">
          <span>{t("localOnly")}</span>
          <Button type="submit">{t("save")}</Button>
        </footer>
      </form>
    </div>
  );
}
