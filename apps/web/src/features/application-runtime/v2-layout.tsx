import { EmbeddedPage } from "./page-view";
import { Fields } from "./display";
import type { ReactNode } from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  extensions,
  LocalBoundary,
  useRuntime,
  type RecordData,
} from "./v2-registry";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
export function PageLayout({
  record,
  children,
}: {
  record: RecordData;
  children: ReactNode;
}) {
  const ctx = useRuntime();
  const { t } = useTranslation("runtime");
  const layout = ctx.page.layout;
  const claimed = new Set(layout.sections.flatMap((s) => s.fields));

  const [active, setActive] = useState(layout.sections[0]?.id ?? "");
  if (ctx.page.mode === "custom" && !ctx.readOnly) {
    const View = extensions.resolve(ctx.page.view, ctx.application).render;
    if (!View) throw new Error("component_not_registered");
    extensions
      .resolve(ctx.page.view, ctx.application)
      .propsSchema?.parse({ record });
    return (
      <LocalBoundary>
        <View record={record} />
      </LocalBoundary>
    );
  }
  const sections = layout.sections.map((section) => {
    let content: ReactNode = null;
    if (section.slot && !ctx.readOnly) {
      const extension = extensions.resolve(section.slot, ctx.application);
      const View = extension.render;
      if (!View) throw new Error("component_not_registered");
      extension.propsSchema?.parse({ record });
      content = (
        <LocalBoundary key={section.slot}>
          <View record={record} />
        </LocalBoundary>
      );
    }
    content = (
      <>
        <Fields
          fields={ctx.page.fields.filter((f) =>
            section.fields.includes(f.name),
          )}
          record={record}
        />
        {content}
        {section.pageId && <EmbeddedPage pageId={section.pageId} />}
      </>
    );
    return { section, content };
  });
  return (
    <div
      className={`runtime-layout runtime-layout-${layout.type}`}
      style={
        {
          "--runtime-columns": Math.min(4, Math.max(1, layout.columns)),
        } as React.CSSProperties
      }
    >
      <section className="admin-panel">
        <Fields
          fields={ctx.page.fields.filter((f) => !claimed.has(f.name))}
          record={record}
        />
        {children}
      </section>
      {layout.type === "tabs" ? (
        <Tabs
          value={active}
          onValueChange={(value) => setActive(String(value))}
        >
          <TabsList>
            {sections.map(({ section }) => (
              <TabsTrigger key={section.id} value={section.id}>
                {t(section.labelKey)}
              </TabsTrigger>
            ))}
          </TabsList>
          {sections.map(({ section, content }) => (
            <TabsContent key={section.id} value={section.id}>
              {content}
            </TabsContent>
          ))}
        </Tabs>
      ) : (
        sections.map(({ section, content }) => (
          <section key={section.id} className="runtime-section">
            <h2>{t(section.labelKey)}</h2>
            {content}
          </section>
        ))
      )}
    </div>
  );
}
