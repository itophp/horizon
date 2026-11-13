import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { runtimeRead } from "@/lib/capabilities";
import { ErrorBanner, Loading } from "@/features/administration/shared";
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
    <section className="space-y-3" aria-labelledby="runtime-contracts-heading">
      <h2 id="runtime-contracts-heading" className="text-lg font-semibold">
        {t("contracts.heading")}
      </h2>
      {list.isPending && <Loading />}
      {list.isError && <ErrorBanner error={list.error} />}
      {list.data && (
        <>
          <div className="flex flex-wrap gap-2">
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
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - 50))}
            >
              {t("contracts.previous")}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!list.data.hasMore}
              onClick={() => setOffset(list.data.nextOffset)}
            >
              {t("contracts.next")}
            </Button>
          </div>
        </>
      )}
      {detail.isPending && selected && <Loading />}
      {detail.isError && <ErrorBanner error={detail.error} />}
      {detail.data && (
        <div className="space-y-2">
          <h3 className="font-medium">
            {`${detail.data.contract.name}@${detail.data.contract.version}`}
          </h3>
          <p>{detail.data.contract.description}</p>
          <p>
            {t("contracts.events")}{" "}
            {detail.data.contract.events.join(", ") || "—"}
          </p>
          <p>
            {t("contracts.slots")}{" "}
            {detail.data.contract.slots.join(", ") || "—"}
          </p>
          <p>
            {t("contracts.binding")} {detail.data.contract.binding || "—"}
          </p>
          <p>
            {t("contracts.readOnly")} {detail.data.contract.readOnly}
          </p>
          <details>
            <summary>{t("contracts.properties")}</summary>
            <pre className="overflow-auto whitespace-pre-wrap">
              {detail.data.contract.propsSchema}
            </pre>
          </details>
          {detail.data.contract.example && (
            <details>
              <summary>{t("contracts.example")}</summary>
              <pre className="overflow-auto whitespace-pre-wrap">
                {detail.data.contract.example}
              </pre>
            </details>
          )}
        </div>
      )}
    </section>
  );
}
