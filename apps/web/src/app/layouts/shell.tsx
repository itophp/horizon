import { NavLink, Outlet } from "react-router";
import { useTranslation } from "react-i18next";
import {
  Blocks,
  LayoutDashboard,
  Layers,
  Globe2,
  ArrowUpRight,
} from "lucide-react";
import { changeLanguage } from "@/i18n";
export function Shell() {
  const { t, i18n } = useTranslation(["common", "navigation"]);
  return (
    <div className="app-shell">
      <a className="skip-link" href="#content">
        {t("navigation:skip")}
      </a>
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">
            <Blocks size={22} />
          </span>
          <div>
            <strong>{t("appName")}</strong>
            <small>{t("workspace")}</small>
          </div>
        </div>
        <nav aria-label={t("navigation:main")}>
          <NavLink to="/foundation" end>
            <LayoutDashboard size={18} />
            {t("navigation:overview")}
            <ArrowUpRight size={14} />
          </NavLink>
          <NavLink to="/architecture">
            <Layers size={18} />
            {t("navigation:principles")}
          </NavLink>
        </nav>
        <div className="sidebar-note">
          <span className="status-dot" />
          {t("framework")}
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <span>{t("workspace")}</span>
          <label className="language">
            <Globe2 size={16} />
            <span className="sr-only">{t("language")}</span>
            <select
              aria-label={t("language")}
              value={i18n.resolvedLanguage}
              onChange={(event) => {
                void changeLanguage(event.target.value);
              }}
            >
              <option value="en">{t("english")}</option>
              <option value="zh-CN">{t("chinese")}</option>
            </select>
          </label>
        </header>
        <main id="content">
          <Outlet />
        </main>
        <footer>{t("footer")}</footer>
      </div>
    </div>
  );
}
