import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { runtimeRead } from "@/lib/capabilities";
import { ErrorBanner, Loading } from "@/features/administration/shared";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";

const pageSchema = z.object({
  names: z.array(z.string()),
  hasMore: z.boolean(),
  nextOffset: z.number().int().nonnegative(),
});
const detailSchema = z.object({
  contract: z.object({
    name: z.string(),
    version: z.string(),
    description: z.string(),
    propsSchema: z.string(),
    slots: z.array(z.string()),
    events: z.array(z.string()),
    binding: z.string(),
    readOnly: z.string(),
    example: z.string(),
  }),
});

export function DesignContracts({ identity }: { identity: string }) {
  const { t } = useTranslation("runtime");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<string>();
  const list = useQuery({
    queryKey: ["runtime-design-contracts", identity, offset],
    queryFn: async ({ signal }) =>
      pageSchema.parse(
        await runtimeRead(
          "application.runtime.definition.contracts",
          { kind: "element", offset },
          signal,
        ),
      ),
    retry: false,
  });
  const detail = useQuery({
    queryKey: ["runtime-design-contract", identity, selected],
    enabled: !!selected,
    queryFn: async ({ signal }) =>
      detailSchema.parse(
        await runtimeRead(
          "application.runtime.definition.contracts",
          { kind: "element", name: selected },
          signal,
        ),
      ),
    retry: false,
  });

  return (
    <section
      className="design-contracts"
      aria-labelledby="runtime-contracts-heading"
    >
      <h2 id="runtime-contracts-heading" className="design-kicker">
        {t("contracts.heading")}
      </h2>
      <p className="design-tab-hint">{t("design.contractsHint")}</p>
      {list.isPending && <Loading />}
      {list.isError && <ErrorBanner error={list.error} />}
      {list.data && (
        <>
          <div className="design-contract-list">
            {list.data.names.map((name) => (
              <Button
                key={name}
                type="button"
                size="sm"
                variant={selected === name ? "default" : "outline"}
                aria-pressed={selected === name}
                onClick={() => setSelected(name)}
              >
                {name}
              </Button>
            ))}
          </div>
          <div className="design-contract-pager">
            <Button
              type="button"
              variant="outline"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - 50))}
            >
              <ChevronLeft size={14} />
              {t("contracts.previous")}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!list.data.hasMore}
              onClick={() => setOffset(list.data.nextOffset)}
            >
              {t("contracts.next")}
              <ChevronRight size={14} />
            </Button>
          </div>
        </>
      )}
      {detail.isPending && selected && <Loading />}
      {detail.isError && <ErrorBanner error={detail.error} />}
      {detail.data && (
        <div className="design-contract-detail">
          <h3>
            {`${detail.data.contract.name}@${detail.data.contract.version}`}
          </h3>
          <p>{detail.data.contract.description}</p>
          <dl className="design-facts">
            <div>
              <dt>{t("contracts.events")}</dt>
              <dd>{detail.data.contract.events.join(", ") || "—"}</dd>
            </div>
            <div>
              <dt>{t("contracts.slots")}</dt>
              <dd>{detail.data.contract.slots.join(", ") || "—"}</dd>
            </div>
            <div>
              <dt>{t("contracts.binding")}</dt>
              <dd>{detail.data.contract.binding || "—"}</dd>
            </div>
            <div>
              <dt>{t("contracts.readOnly")}</dt>
              <dd>{detail.data.contract.readOnly}</dd>
            </div>
          </dl>
          <details className="design-definition">
            <summary>{t("contracts.properties")}</summary>
            <pre>{detail.data.contract.propsSchema}</pre>
          </details>
          {detail.data.contract.example && (
            <details className="design-definition">
              <summary>{t("contracts.example")}</summary>
              <pre>{detail.data.contract.example}</pre>
            </details>
          )}
        </div>
      )}
    </section>
  );
}
