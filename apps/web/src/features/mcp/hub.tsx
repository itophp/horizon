import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { Command, Plug, Sparkles } from "lucide-react";
import { PageHeading } from "@/features/administration/shared";
import { useWebMcp } from "@/features/webmcp/provider";
import { WebMcpSection } from "@/features/webmcp/settings";
import { WebSkillsSection } from "@/features/webmcp/skills";
import { RemoteMcpSection } from "./remote";
import "./mcp.css";
import "./hub.css";
const tabs = ["remote", "page", "skills"] as const;
type TabId = (typeof tabs)[number];
const tabParam = "tab";
const storageKey = "baseline.mcp.tab";
function isTab(value: string | null): value is TabId {
  return !!value && tabs.includes(value as TabId);
}
// The active tab is interface state, not a document: the URL only seeds it so
// redirects and deep links land on the intended panel, while in-page
// navigation triggered by MCP tools does not reset the panel in use.
function storedTab(): TabId {
  try {
    const value = sessionStorage.getItem(storageKey);
    return isTab(value) ? value : "remote";
  } catch {
    return "remote";
  }
}
export function McpHubPage() {
  const { t } = useTranslation("mcp");
  const [params, setParams] = useSearchParams();
  const { status, toolCount } = useWebMcp();
  const [tab, setTab] = useState<TabId>(() => {
    const requested = params.get(tabParam);
    return isTab(requested) ? requested : storedTab();
  });
  function select(next: TabId) {
    setTab(next);
    try {
      sessionStorage.setItem(storageKey, next);
    } catch {
      // Private modes can block storage; switching still works without it.
    }
    setParams({ [tabParam]: next }, { replace: true });
  }
  return (
    <>
      <PageHeading title={t("hub.title")} subtitle={t("hub.subtitle")} />
      <div
        className="admin-page-tabs"
        role="tablist"
        aria-label={t("hub.tabsLabel")}
      >
        <button
          type="button"
          role="tab"
          aria-selected={tab === "remote"}
          className={tab === "remote" ? "active" : ""}
          onClick={() => select("remote")}
        >
          <Command size={15} />
          {t("hub.tabs.remote")}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "page"}
          className={tab === "page" ? "active" : ""}
          aria-label={
            status === "connected"
              ? t("hub.pageTabAria", { count: toolCount })
              : undefined
          }
          onClick={() => select("page")}
        >
          <Plug size={15} />
          {t("hub.tabs.page")}
          {status === "connected" && (
            <span className="mcp-hub-count" aria-hidden="true">
              {toolCount}
            </span>
          )}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "skills"}
          className={tab === "skills" ? "active" : ""}
          onClick={() => select("skills")}
        >
          <Sparkles size={15} />
          {t("hub.tabs.skills")}
        </button>
      </div>
      <div role="tabpanel">
        {tab === "remote" ? (
          <RemoteMcpSection />
        ) : tab === "page" ? (
          <WebMcpSection />
        ) : (
          <WebSkillsSection />
        )}
      </div>
    </>
  );
}
