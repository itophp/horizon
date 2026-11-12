import { useState } from "react";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverTitle,
} from "@/components/ui/popover";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from "@/components/ui/accordion";
import { Records } from "./records";
import { Choice } from "./primitives";
export function ListPatterns({
  onNew,
  created,
  renamed,
}: {
  onNew: () => void;
  created: string[];
  renamed: Record<string, string>;
}) {
  const { t } = useTranslation("showcase");
  const [q, setQ] = useSearchParams();
  const [local, setLocal] = useState({
    layout: q.get("layout") ?? "treeList",
    group: q.get("group") ?? "allRecords",
    status: q.get("status") ?? "all",
  });
  const [previous, setPrevious] = useState(q.toString());
  if (previous !== q.toString()) {
    setPrevious(q.toString());
    setLocal({
      layout: q.get("layout") ?? "treeList",
      group: q.get("group") ?? "allRecords",
      status: q.get("status") ?? "all",
    });
  }
  const { layout, group, status } = local;
  function change(key: string, value: string) {
    const controls = { ...local, [key]: value };
    setLocal(controls);
    const next = new URLSearchParams(q);
    for (const [name, entry] of Object.entries(controls)) next.set(name, entry);
    next.set("p", "1");
    if (key === "group") next.delete("q");
    setQ(next);
  }
  return (
    <>
      <div className="sc-pattern-controls">
        <Choice
          labelKey="layoutRule"
          value={layout}
          options={["plainList", "treeList", "categoryList"]}
          onChange={(v) => change("layout", v)}
        />
        <Popover>
          <PopoverTrigger render={<Button variant="outline" />}>
            {t("filter")}
          </PopoverTrigger>
          <PopoverContent>
            <PopoverTitle>{t("filter")}</PopoverTitle>
            <Choice
              labelKey="status"
              value={status}
              options={["all", "active", "review", "done"]}
              onChange={(v) => change("status", v)}
            />
            <Button variant="outline" onClick={() => change("status", "all")}>
              {t("clear")}
            </Button>
          </PopoverContent>
        </Popover>
      </div>
      <div className={layout === "plainList" ? "" : "sc-pattern-split"}>
        {layout !== "plainList" && (
          <aside className="sc-filter-nav">
            <h2>
              {t(layout === "treeList" ? "organizationTree" : "businessType")}
            </h2>
            <Button
              variant={group === "allRecords" ? "secondary" : "ghost"}
              onClick={() => change("group", "allRecords")}
            >
              {t("allRecords")}
            </Button>
            {layout === "treeList" ? (
              <Accordion defaultValue={["hq"]}>
                <AccordionItem value="hq">
                  <AccordionTrigger>{t("headquarters")}</AccordionTrigger>
                  <AccordionContent>
                    {["platformTeam", "operationsTeam", "deliveryTeam"].map(
                      (key) => (
                        <Button
                          key={key}
                          variant={group === key ? "secondary" : "ghost"}
                          onClick={() => change("group", key)}
                        >
                          {t(key)}
                        </Button>
                      ),
                    )}
                  </AccordionContent>
                </AccordionItem>
              </Accordion>
            ) : (
              ["internal", "customer", "partner"].map((key) => (
                <Button
                  key={key}
                  variant={group === key ? "secondary" : "ghost"}
                  onClick={() => change("group", key)}
                >
                  {t(key)}
                </Button>
              ))
            )}
          </aside>
        )}
        <section className="sc-result-area">
          <Tabs
            value={status}
            onValueChange={(value) => {
              change("status", String(value));
            }}
          >
            <TabsList variant="line">
              {["all", "active", "review", "done"].map((key) => (
                <TabsTrigger value={key} key={key}>
                  {t(key)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
          <Records
            key={group + status}
            state="normal"
            statusValue={status}
            onStatusChange={(value) => change("status", value)}
            onReset={() => {
              setLocal({ layout, group: "allRecords", status: "all" });
              const next = new URLSearchParams(q);
              for (const key of ["q", "status", "group", "p"]) next.delete(key);
              setQ(next);
            }}
            created={created}
            renamed={renamed}
            group={group}
            onNew={onNew}
          />
        </section>
      </div>
    </>
  );
}
