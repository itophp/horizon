import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { projects } from "./catalog";
const schema = z.object({
  name: z.string().min(3, "valid"),
  budget: z.number().min(0, "required"),
  date: z.string().min(1, "required"),
  notes: z.string(),
});
export function ProjectForm({
  edit = false,
  variant = "wizard",
  onSaved,
}: {
  edit?: boolean;
  variant?: "wizard" | "single";
  onSaved: (message: string, name?: string) => void;
}) {
  const { t } = useTranslation("showcase");
  const [step, setStep] = useState(0);
  const [fail, setFail] = useState(false);
  const [failed, setFailed] = useState(false);
  const [items, setItems] = useState(["Discovery", "Pilot rollout"]);
  const {
    register,
    handleSubmit,
    trigger,
    control,
    formState: { errors },
  } = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      name: edit ? projects[0] : "",
      budget: 48000,
      date: "2026-11-20",
      notes: "",
    },
  });
  const values = useWatch({ control });
  return (
    <form
      className="sc-form"
      onSubmit={handleSubmit((values) => {
        setFailed(fail);
        if (!fail) onSaved(t("saved"), values.name);
      })}
    >
      {variant === "wizard" && (
        <ol className="sc-steps">
          {["basics", "scope", "confirm"].map((s, i) => (
            <li key={s} aria-current={step === i ? "step" : undefined}>
              <span>{i + 1}</span>
              {t(s)}
            </li>
          ))}
        </ol>
      )}
      {variant === "wizard" && (
        <p className="sc-muted">{t("step", { count: step + 1 })}</p>
      )}
      {(step === 0 || variant === "single") && (
        <>
          <h2>{t("basics")}</h2>
          <label>
            {t("name")}
            <Input
              {...register("name")}
              aria-invalid={!!errors.name}
              aria-describedby="name-error"
            />
          </label>
          {errors.name && (
            <p id="name-error" role="alert">
              {t(String(errors.name.message))}
            </p>
          )}
          <div className="sc-grid-two">
            <label>
              {t("budget")}
              <Input
                type="number"
                {...register("budget", { valueAsNumber: true })}
              />
              {errors.budget && <span role="alert">{t("required")}</span>}
            </label>
            <label>
              {t("date")}
              <Input type="date" {...register("date")} />
              {errors.date && <span role="alert">{t("required")}</span>}
            </label>
          </div>
          <label>
            {t("notes")}
            <Textarea {...register("notes")} />
          </label>
        </>
      )}
      {(step === 1 || variant === "single") && (
        <>
          <h2>{t("scope")}</h2>
          <p>{t("detailsText")}</p>
          {items.map((item, i) => (
            <div className="sc-toolbar" key={i}>
              <Input
                aria-label={t("item")}
                value={item}
                onChange={(e) =>
                  setItems(items.map((v, j) => (i === j ? e.target.value : v)))
                }
              />
              <Button
                type="button"
                variant="outline"
                onClick={() => setItems(items.filter((_, j) => i !== j))}
              >
                {t("remove")}
              </Button>
            </div>
          ))}
          <Button
            type="button"
            variant="outline"
            onClick={() => setItems([...items, ""])}
          >
            {t("add")}
          </Button>
        </>
      )}
      {step === 2 && (
        <>
          <h2>{t("confirm")}</h2>
          <dl className="sc-definition">
            <dt>{t("name")}</dt>
            <dd>{values.name}</dd>
            <dt>{t("budget")}</dt>
            <dd>{values.budget}</dd>
            <dt>{t("date")}</dt>
            <dd>{values.date}</dd>
            <dt>{t("item")}</dt>
            <dd>{items.join(" · ")}</dd>
          </dl>
          <label className="sc-check">
            <Checkbox
              checked={fail}
              onCheckedChange={(v) => setFail(v === true)}
            />
            {t("simulate")}
          </label>
        </>
      )}
      {failed && <p role="alert">{t("failed")}</p>}
      <footer className="sc-pagination">
        <Button
          type="button"
          variant="outline"
          disabled={step === 0}
          onClick={() => setStep(step - 1)}
        >
          {t("previous")}
        </Button>
        {step < 2 && variant === "wizard" ? (
          <Button
            type="button"
            onClick={async () => {
              if (await trigger()) setStep(step + 1);
            }}
          >
            {t("next")}
          </Button>
        ) : (
          <Button type="submit">{t("save")}</Button>
        )}
      </footer>
    </form>
  );
}
