import "@/features/mcp/hub.css";
import { useTranslation } from "react-i18next";
import { Loading } from "@/features/administration/shared";
import { DictionarySection } from "@/features/governance/dictionaries";
import { SettingsConfigPanel } from "@/features/governance/settings";
import { RemoteMcpSection } from "@/features/mcp/remote";
import { WebMcpSection } from "@/features/webmcp/settings";
import { TablePanelComponent } from "./admin-workbench";
import type { ComponentRendererProps } from "./component-registry";

function PreviewNotice() {
  const { t } = useTranslation("runtime");
  return <p className="muted-text">{t("design.previewHint")}</p>;
}

function tableProps(props: Record<string, unknown>) {
  const table = props.table as { columns?: unknown[] } | undefined;
  return { columns: table?.columns ?? [] };
}

export function SettingsPanel({ props, readOnly }: ComponentRendererProps) {
  return (
    <SettingsConfigPanel
      readOnly={readOnly}
      personal={props.personal === true}
    />
  );
}

export function WebMcpPanel({ readOnly }: ComponentRendererProps) {
  return readOnly ? <PreviewNotice /> : <WebMcpSection />;
}

export function RemoteMcpPanel(renderer: ComponentRendererProps) {
  const { t } = useTranslation("mcp");
  if (renderer.data == null) return <Loading />;
  const table = tableProps(renderer.props);
  const data = Array.isArray(renderer.data)
    ? renderer.data.map((row) => ({
        ...row,
        revocation: row.revokedAt
          ? row.revokedBy || t("systemActor")
          : t("notRevoked"),
      }))
    : [];

  if (renderer.readOnly)
    return <TablePanelComponent {...renderer} data={data} props={table} />;
  return (
    <RemoteMcpSection
      onRefresh={async () => {
        await renderer.emit("refresh");
      }}
      tokenTable={(actions) => (
        <TablePanelComponent
          {...renderer}
          props={table}
          data={data}
          renderRowActions={(row) =>
            actions(Number(row.id), String(row.name), Boolean(row.revokedAt))
          }
        />
      )}
    />
  );
}

export function DictionaryPanel(renderer: ComponentRendererProps) {
  const { i18n } = useTranslation();
  const table = tableProps(renderer.props);
  const data = Array.isArray(renderer.data)
    ? renderer.data.map((row) => ({
        ...row,
        displayName:
          i18n.resolvedLanguage?.startsWith("zh") && row.nameZh
            ? row.nameZh
            : row.nameEn,
      }))
    : [];
  if (renderer.readOnly)
    return <TablePanelComponent {...renderer} data={data} props={table} />;
  return (
    <DictionarySection
      onRefresh={async () => {
        await renderer.emit("refresh");
      }}
      renderTable={(actions, open) => (
        <TablePanelComponent
          {...renderer}
          data={data}
          props={table}
          onOpenRow={(row) => open(Number(row.id))}
          renderRowActions={(row) =>
            actions(Number(row.id), Number(row.revision))
          }
        />
      )}
    />
  );
}
