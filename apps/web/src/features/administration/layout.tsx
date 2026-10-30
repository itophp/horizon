import { WebMcpProvider } from "@/features/webmcp/provider";
import { PublishedLogo, useBranding } from "@/features/governance/branding";
import { NotificationBell } from "@/features/governance/notifications";
import { useConfiguration } from "@/features/governance/runtime";
import { RuntimeProvider } from "@/features/governance/runtime";
import { useEffect, useRef, useState } from "react";
import {
  Navigate,
  NavLink,
  Outlet,
  Link,
  useLocation,
  useNavigate,
} from "react-router";
import { useTranslation } from "react-i18next";
import {
  LayoutDashboard,
  Users,
  Building2,
  Network,
  ShieldCheck,
  KeyRound,
  PanelLeft,
  Search,
  Sun,
  Moon,
  ChevronDown,
  LogOut,
  UserRound,
  Bell,
  BookOpen,
  ArrowUpRight,
  X,
  Command,
  Settings2,
  Layers3,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { buttonVariants } from "@/components/ui/button";
import { changeLanguage } from "@/i18n";
import { useTheme } from "@/app/providers/theme";
import { AuthProvider, useAuth, useSession } from "./auth";
import {
  BrandMark,
  EditorDialog,
  initials,
  Loading,
  useAdminData,
  StatusBadge,
  ErrorBanner,
} from "./shared";
export function AuthBoundary() {
  return (
    <AuthProvider>
      <Outlet />
    </AuthProvider>
  );
}
export function RequireSession() {
  const location = useLocation();
  const auth = useAuth();
  if (auth.loading)
    return (
      <div className="auth-loading-page">
        <Loading />
      </div>
    );
  if (!auth.profile)
    return (
      <Navigate
        to="/login"
        state={{ returnTo: location.pathname + location.search }}
        replace
      />
    );
  return (
    <RuntimeProvider>
      <WebMcpProvider
        key={`${auth.profile.tenantId}:${auth.profile.account.id}:${auth.profile.activeDepartmentId}:${auth.profile.csrfToken}`}
      >
        <Outlet />
      </WebMcpProvider>
    </RuntimeProvider>
  );
}
export function AdminLayout() {
  const { t, i18n } = useTranslation();
  const config = useConfiguration();
  const brand = useBranding();
  const session = useSession();
  const auth = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { theme, toggle } = useTheme();
  const { data } = useAdminData();
  const [collapsed, setCollapsed] = useState(false);
  const [search, setSearch] = useState("");
  const [profileOpen, setProfileOpen] = useState(false);
  const [guideOpen, setGuideOpen] = useState(false);
  const [switching, setSwitching] = useState(false);
  const [logoutError, setLogoutError] = useState<unknown>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    function shortcut(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        searchRef.current?.focus();
      }
    }
    window.addEventListener("keydown", shortcut);
    return () => window.removeEventListener("keydown", shortcut);
  }, []);
  const links = [
    {
      to: "/",
      label: t("admin:nav.overview"),
      icon: LayoutDashboard,
      permission: "",
    },
    {
      to: "/accounts",
      label: t("admin:nav.accounts"),
      icon: Users,
      permission: "accounts:read",
    },
    {
      to: "/organizations",
      label: t("admin:nav.organizations"),
      icon: Building2,
      permission: "organizations:read",
    },
    {
      to: "/departments",
      label: t("admin:nav.departments"),
      icon: Network,
      permission: "departments:read",
    },
    {
      to: "/roles",
      label: t("admin:nav.roles"),
      icon: ShieldCheck,
      permission: "roles:read",
    },
    {
      to: "/permissions",
      label: t("admin:nav.permissions"),
      icon: KeyRound,
      permission: "access:manage",
    },
    {
      to: "/notifications",
      label: t("governance:notificationCenter"),
      icon: Bell,
      permission: "",
    },
    { to: "/webmcp", label: t("webmcp:title"), icon: Command, permission: "" },
    {
      to: "/settings",
      label: t("governance:settings"),
      icon: Settings2,
      permission: "settings:read",
    },
    {
      to: "/dictionaries",
      label: t("governance:dictionaries"),
      icon: BookOpen,
      permission: "dictionaries:read",
    },
    {
      to: "/attachments",
      label: t("governance:files"),
      icon: Layers3,
      permission: "files:read",
    },
  ];
  const current =
    location.pathname === "/preferences"
      ? { label: t("governance:preferences") }
      : location.pathname === "/notifications"
        ? { label: t("governance:notificationCenter") }
        : location.pathname === "/temporary-access"
          ? { label: t("admin:temporary.title") }
          : (links.find((link) => link.to === location.pathname) ?? links[0]);
  const matched = search.trim().toLowerCase();
  const results = matched
    ? [
        ...data.accounts
          .filter((item) =>
            `${item.fullName} ${item.username}`.toLowerCase().includes(matched),
          )
          .map((item) => ({
            key: `a-${item.id}`,
            label: item.fullName,
            kind: t("admin:nav.accounts"),
            href: `/accounts?q=${encodeURIComponent(item.username)}`,
            icon: Users,
          })),
        ...data.organizations
          .filter((item) =>
            `${item.name} ${item.code}`.toLowerCase().includes(matched),
          )
          .map((item) => ({
            key: `o-${item.id}`,
            label: item.name,
            kind: t("admin:nav.organizations"),
            href: `/organizations?q=${encodeURIComponent(item.code)}`,
            icon: Building2,
          })),
        ...data.departments
          .filter((item) =>
            `${item.name} ${item.code}`.toLowerCase().includes(matched),
          )
          .map((item) => ({
            key: `d-${item.id}`,
            label: item.name,
            kind: t("admin:nav.departments"),
            href: `/departments?q=${encodeURIComponent(item.code)}`,
            icon: Network,
          })),
        ...data.roles
          .filter((item) =>
            `${item.name} ${item.code}`.toLowerCase().includes(matched),
          )
          .map((item) => ({
            key: `r-${item.id}`,
            label: item.name,
            kind: t("admin:nav.roles"),
            href: `/roles?q=${encodeURIComponent(item.code)}`,
            icon: ShieldCheck,
          })),
      ].slice(0, 7)
    : [];
  return (
    <div className={`admin-app ${collapsed ? "navigation-collapsed" : ""}`}>
      <a href="#admin-content" className="skip-link">
        {t("navigation:skip")}
      </a>
      <div className="console-frame">
        <aside className="console-sidebar">
          <Link to="/" className="console-brand">
            <PublishedLogo />
            <BrandMark />
            <div>
              <strong>
                {String(config.value("branding.name", t("admin:brand")))}
              </strong>
              <span>
                {brand.text("branding.tagline", t("admin:workspace"))}
              </span>
            </div>
            <ChevronDown size={13} />
          </Link>
          <div className="sidebar-workspace">
            <span className="workspace-indicator" />
            <select
              aria-label={t("admin:memberships.current")}
              value={session.activeDepartmentId}
              disabled={switching}
              onChange={async (event) => {
                setSwitching(true);
                setLogoutError(null);
                try {
                  await auth.switchDepartment(Number(event.target.value));
                  navigate("/");
                } catch (error) {
                  setLogoutError(error);
                } finally {
                  setSwitching(false);
                }
              }}
            >
              {session.account.memberships.map((m) => (
                <option key={m.departmentId} value={m.departmentId}>
                  {m.departmentName ??
                    data.departments.find((d) => d.id === m.departmentId)
                      ?.name ??
                    t("admin:fields.unassigned")}
                </option>
              ))}
            </select>
          </div>
          <nav className="console-nav" aria-label={t("navigation:main")}>
            <div className="nav-section-label">{t("admin:nav.workspace")}</div>
            {links.slice(0, 1).map((link) => (
              <NavLink
                key={link.to}
                to={link.to}
                end
                className={({ isActive }) => (isActive ? "active" : "")}
              >
                <link.icon size={17} />
                <span>{link.label}</span>
                <ArrowUpRight size={13} />
              </NavLink>
            ))}
            <div className="nav-section-label second-label">
              {t("admin:nav.management")}
            </div>
            {links
              .slice(1)
              .filter(
                (link) =>
                  !link.permission ||
                  session.permissions.includes(link.permission),
              )
              .map((link) => (
                <NavLink
                  to={link.to}
                  aria-label={link.label}
                  key={link.to}
                  className={({ isActive }) => (isActive ? "active" : "")}
                >
                  <link.icon size={17} />
                  <span>{link.label}</span>
                  {link.to === "/accounts" && (
                    <small>{data.accounts.length}</small>
                  )}
                </NavLink>
              ))}
          </nav>
          <div className="sidebar-bottom">
            <button
              className="sidebar-guide"
              onClick={() => setGuideOpen(true)}
            >
              <BookOpen size={16} />
              <span>{t("admin:nav.documentation")}</span>
              <ArrowUpRight size={13} />
            </button>
            <div className="workspace-security">
              <ShieldCheck size={17} />
              <div>
                <strong>{t("admin:overview.security")}</strong>
                <small>{t("admin:overview.securityHint")}</small>
              </div>
            </div>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <button className="sidebar-profile">
                    <span className="person-avatar tone-1">
                      {initials(session.account.fullName)}
                    </span>
                    <span>
                      <strong>{session.account.fullName}</strong>
                      <small>
                        {session.account.email || session.account.username}
                      </small>
                    </span>
                    <ChevronDown size={13} />
                  </button>
                }
              />
              <DropdownMenuContent align="start">
                <DropdownMenuItem onClick={() => setProfileOpen(true)}>
                  <UserRound size={15} />
                  {t("admin:profile.title")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => {
                    setLogoutError(null);
                    void auth
                      .signOut()
                      .then(() => navigate("/login", { replace: true }))
                      .catch(setLogoutError);
                  }}
                >
                  <LogOut size={15} />
                  {t("admin:actions.signOut")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </aside>
        <div className="console-main">
          <header className="console-topbar">
            <button
              className="icon-button sidebar-toggle"
              onClick={() => setCollapsed((value) => !value)}
              aria-label={t("admin:actions.collapse")}
            >
              <PanelLeft size={17} />
            </button>
            <span className="topbar-divider" />
            <div className="global-search">
              <Search size={16} />
              <input
                ref={searchRef}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") setSearch("");
                }}
                placeholder={t("admin:search.placeholder")}
                aria-label={t("admin:search.placeholder")}
                aria-expanded={!!matched}
                aria-controls="workspace-search-results"
              />
              <kbd>
                <Command size={11} />
                {t("admin:search.shortcut")}
              </kbd>
              {matched && (
                <div
                  className="global-search-results"
                  id="workspace-search-results"
                >
                  <div>
                    {t("admin:search.results")}
                    <button
                      aria-label={t("admin:actions.close")}
                      onClick={() => setSearch("")}
                    >
                      <X size={13} />
                    </button>
                  </div>
                  {results.map((result) => (
                    <Link
                      key={result.key}
                      to={result.href}
                      onClick={() => setSearch("")}
                    >
                      <result.icon size={16} />
                      <span>
                        <strong>{result.label}</strong>
                        <small>{result.kind}</small>
                      </span>
                      <ArrowUpRight size={13} />
                    </Link>
                  ))}
                  {!results.length && <p>{t("admin:search.none")}</p>}
                </div>
              )}
            </div>
            <div className="topbar-actions">
              <NavLink
                to="/temporary-access"
                className={`${buttonVariants({ variant: "outline", size: "sm" })} topbar-temporary-access`}
                aria-label={t("admin:temporary.create")}
                title={t("admin:temporary.create")}
              >
                <KeyRound aria-hidden="true" />
                <span>{t("admin:temporary.create")}</span>
              </NavLink>
              <NotificationBell />
              <span className="topbar-workspace-label">
                {t("admin:console")}
              </span>
              <select
                aria-label={t("common:language")}
                value={i18n.resolvedLanguage}
                onChange={(event) => {
                  void changeLanguage(event.target.value);
                }}
              >
                <option value="en">{t("admin:locale.en")}</option>
                <option value="zh-CN">{t("admin:locale.zh")}</option>
              </select>
              <button
                className="icon-button"
                onClick={toggle}
                aria-label={t("admin:theme.toggle")}
                title={
                  theme === "dark"
                    ? t("admin:theme.light")
                    : t("admin:theme.dark")
                }
              >
                {theme === "dark" ? <Sun size={17} /> : <Moon size={17} />}
              </button>
              <button
                className="icon-button"
                onClick={() => navigate("/preferences")}
                aria-label={t("governance:preferences")}
              >
                <Settings2 size={17} />
              </button>
            </div>
          </header>
          <main className="console-content" id="admin-content">
            <div className="console-breadcrumb">
              <span>{t("admin:console")}</span>
              <ChevronDown size={11} />
              <span>{current.label}</span>
            </div>
            {!!logoutError && <ErrorBanner error={logoutError} />}
            <Outlet />
          </main>
          <footer className="console-footer">
            <span>{brand.text("branding.footer", t("admin:footer"))}</span>
            <span>
              <i />
              {t("admin:overview.scopeNote")}
            </span>
          </footer>
        </div>
      </div>
      <EditorDialog
        open={profileOpen}
        onOpenChange={setProfileOpen}
        title={t("admin:profile.title")}
        description={t("admin:profile.session")}
      >
        <div className="profile-summary">
          <span className="person-avatar tone-1">
            {initials(session.account.fullName)}
          </span>
          <h3>{session.account.fullName}</h3>
          <p>{session.account.email || session.account.username}</p>
          <StatusBadge status={session.account.status} />
          <dl>
            <div>
              <dt>{t("admin:fields.organization")}</dt>
              <dd>
                {data.organizations.find(
                  (org) => org.id === session.account.organizationId,
                )?.name ?? t("admin:fields.unassigned")}
              </dd>
            </div>
            <div>
              <dt>{t("admin:fields.department")}</dt>
              <dd>
                {data.departments.find(
                  (dept) => dept.id === session.account.departmentId,
                )?.name ?? t("admin:fields.unassigned")}
              </dd>
            </div>
          </dl>
        </div>
      </EditorDialog>
      <EditorDialog
        open={guideOpen}
        onOpenChange={setGuideOpen}
        title={t("admin:guide.title")}
        description={t("admin:permissions.legend")}
      >
        <div className="access-guide">
          {[
            t("admin:guide.one"),
            t("admin:guide.two"),
            t("admin:guide.three"),
            t("admin:guide.four"),
          ].map((text, index) => (
            <div key={text}>
              <span>{index + 1}</span>
              <p>{text}</p>
            </div>
          ))}
        </div>
      </EditorDialog>
    </div>
  );
}
