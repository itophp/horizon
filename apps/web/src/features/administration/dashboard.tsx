import { useConfiguration } from "@/features/governance/runtime";
import { useMemo, useState } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import {
  Users,
  Building2,
  ShieldCheck,
  Network,
  ArrowUpRight,
  ArrowRight,
  CalendarDays,
  Plus,
  KeyRound,
} from "lucide-react";
import { useSession } from "./auth";
import {
  useAdminData,
  PageHeading,
  StatusBadge,
  ShortDate,
  initials,
  Loading,
  ErrorBanner,
  EmptyState,
  DataTable,
  type AdminColumn,
} from "./shared";
import { Button } from "@/components/ui/button";
import type { AccountRecord } from "@/api/generated/models";
function dateKey(value: Date, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(value);
  return ["year", "month", "day"]
    .map((key) => parts.find((part) => part.type === key)?.value)
    .join("-");
}
export function DashboardPage() {
  const { t, i18n } = useTranslation();
  const config = useConfiguration();
  const timeZone = String(config.value("display.timezone", "Asia/Shanghai"));
  const session = useSession();
  const query = useAdminData();
  const data = query.data;
  const [tab, setTab] = useState<"overview" | "recent">("overview");
  const [now] = useState(() => new Date());
  const number = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      useGrouping: Boolean(config.value("display.grouping", true)),
    }).format(value);
  const chart = useMemo(() => {
    const days = Array.from(
      { length: 7 },
      (_, i) => new Date(now.getTime() - (6 - i) * 86400000),
    );
    const counts = days.map(
      (day) =>
        data.accounts.filter(
          (account) =>
            dateKey(new Date(account.createdAt), timeZone) <=
            dateKey(day, timeZone),
        ).length,
    );
    const max = Math.max(3, ...counts);
    const points = counts.map((count, index) => [
      index * 110,
      164 - (count / max) * 138,
    ]);
    return {
      days,
      counts,
      max,
      points,
      path: points.map((p, i) => `${i ? "L" : "M"}${p[0]},${p[1]}`).join(" "),
    };
  }, [data.accounts, now, timeZone]);
  const recent = useMemo(
    () =>
      [
        ...data.accounts.map((account) => ({
          id: `account-${account.id}`,
          name: account.fullName,
          type: t("admin:nav.accounts"),
          date: account.createdAt,
          href: `/accounts?q=${encodeURIComponent(account.username)}`,
        })),
        ...data.organizations.map((org) => ({
          id: `org-${org.id}`,
          name: org.name,
          type: t("admin:nav.organizations"),
          date: org.createdAt,
          href: `/organizations?q=${encodeURIComponent(org.code)}`,
        })),
        ...data.departments.map((dept) => ({
          id: `dept-${dept.id}`,
          name: dept.name,
          type: t("admin:nav.departments"),
          date: dept.createdAt,
          href: `/departments?q=${encodeURIComponent(dept.code)}`,
        })),
        ...data.roles.map((role) => ({
          id: `role-${role.id}`,
          name: role.name,
          type: t("admin:nav.roles"),
          date: role.createdAt,
          href: `/roles?q=${encodeURIComponent(role.code)}`,
        })),
      ]
        .sort((a, b) => b.date.localeCompare(a.date))
        .slice(0, 10),
    [data, t],
  );
  const stats = [
    {
      label: t("admin:overview.activeAccounts"),
      value: data.accounts.filter((a) => a.status === "active").length,
      detail: t("admin:overview.totalAccounts", {
        count: data.accounts.length,
      }),
      icon: Users,
    },
    {
      label: t("admin:overview.orgCount"),
      value: data.organizations.length,
      detail: t("admin:overview.orgDetail", {
        enterprises: data.organizations.filter((o) => o.kind === "enterprise")
          .length,
        companies: data.organizations.filter((o) => o.kind === "company")
          .length,
      }),
      icon: Building2,
    },
    {
      label: t("admin:overview.roleCount"),
      value: data.roles.length,
      detail: t("admin:overview.roleDetail"),
      icon: ShieldCheck,
    },
    {
      label: t("admin:overview.deptCount"),
      value: data.departments.length,
      detail: t("admin:overview.deptDetail"),
      icon: Network,
    },
  ];
  const columns: AdminColumn<AccountRecord>[] = [
    {
      id: "account",
      header: t("admin:fields.fullName"),
      cell: (account) => (
        <div className="account-cell">
          <span className={`person-avatar tone-${account.id % 5}`}>
            {initials(account.fullName)}
          </span>
          <span>
            <strong>{account.fullName}</strong>
            <small>{account.email || account.username}</small>
          </span>
        </div>
      ),
    },
    {
      id: "department",
      header: t("admin:fields.department"),
      cell: (account) => (
        <div className="cell-tags">
          {account.memberships.map((m) => (
            <span key={m.departmentId} className="neutral-badge">
              {m.departmentName ??
                data.departments.find((d) => d.id === m.departmentId)?.name ??
                t("admin:fields.unassigned")}
            </span>
          ))}
        </div>
      ),
    },
    {
      id: "status",
      header: t("admin:fields.status"),
      cell: (account) => <StatusBadge status={account.status} />,
    },
    {
      id: "date",
      header: t("admin:fields.createdAt"),
      cell: (account) => <ShortDate value={account.createdAt} />,
    },
  ];
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("admin:overview.title")}
        subtitle={t("admin:overview.welcome", {
          name: session.account.fullName,
        })}
      >
        <span className="date-chip">
          <CalendarDays size={14} />
          {new Intl.DateTimeFormat(i18n.resolvedLanguage, {
            year: "numeric",
            month: "short",
            day: "numeric",
            timeZone,
          }).format(now)}
        </span>
      </PageHeading>
      {query.isError && (
        <ErrorBanner
          error={query.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      )}
      <div className="overview-tabs">
        <button
          className={tab === "overview" ? "active" : ""}
          onClick={() => setTab("overview")}
        >
          {t("admin:overview.tabOverview")}
        </button>
        <button
          className={tab === "recent" ? "active" : ""}
          onClick={() => setTab("recent")}
        >
          {t("admin:overview.tabActivity")}
        </button>
      </div>
      <div className="stat-grid">
        {stats.map((stat) => (
          <article className="admin-panel stat-card" key={stat.label}>
            <div>
              <h2>{stat.label}</h2>
              <stat.icon size={20} />
            </div>
            <strong>{number(stat.value)}</strong>
            <p>
              <span className="stat-dot" />
              {stat.detail}
            </p>
          </article>
        ))}
      </div>
      {tab === "recent" ? (
        <section className="admin-panel recent-panel">
          <div className="panel-heading">
            <div>
              <h2>{t("admin:overview.tabActivity")}</h2>
              <p>{t("admin:overview.scopeNote")}</p>
            </div>
          </div>
          {recent.map((item) => (
            <Link to={item.href} className="recent-item" key={item.id}>
              <span className="entity-symbol">
                <Plus size={15} />
              </span>
              <div>
                <strong>{item.name}</strong>
                <small>{item.type}</small>
              </div>
              <ShortDate value={item.date} />
              <ArrowUpRight size={15} />
            </Link>
          ))}
          {!recent.length && <EmptyState />}
        </section>
      ) : (
        <>
          <div className="dashboard-upper">
            <section className="admin-panel chart-panel">
              <div className="panel-heading">
                <div>
                  <h2>{t("admin:overview.chartTitle")}</h2>
                  <p>{t("admin:overview.chartSubtitle")}</p>
                </div>
                <span className="neutral-badge">
                  {t("admin:overview.sevenDays")}
                </span>
              </div>
              <div className="growth-chart">
                <div className="chart-labels">
                  {[
                    chart.max,
                    Math.round((chart.max * 2) / 3),
                    Math.round(chart.max / 3),
                    0,
                  ].map((label, index) => (
                    <span key={index}>{number(label)}</span>
                  ))}
                </div>
                <div className="chart-drawing">
                  <svg
                    viewBox="0 0 660 180"
                    preserveAspectRatio="none"
                    role="img"
                    aria-label={t("admin:overview.chartTitle")}
                  >
                    <defs>
                      <linearGradient
                        id="growth-fill"
                        x1="0"
                        y1="0"
                        x2="0"
                        y2="1"
                      >
                        <stop
                          offset="0%"
                          stopColor="currentColor"
                          stopOpacity=".22"
                        />
                        <stop
                          offset="100%"
                          stopColor="currentColor"
                          stopOpacity="0"
                        />
                      </linearGradient>
                    </defs>
                    {[26, 72, 118, 164].map((y) => (
                      <line
                        key={y}
                        x1="0"
                        y1={y}
                        x2="660"
                        y2={y}
                        className="chart-gridline"
                      />
                    ))}
                    <path
                      d={`${chart.path} L660,164 L0,164 Z`}
                      fill="url(#growth-fill)"
                    />
                    <path
                      d={chart.path}
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      vectorEffect="non-scaling-stroke"
                    />
                    {chart.points.map(([x, y], index) => (
                      <circle
                        key={index}
                        cx={x}
                        cy={y}
                        r="3"
                        fill="var(--app-panel)"
                        stroke="currentColor"
                        strokeWidth="1.5"
                      />
                    ))}
                  </svg>
                  <div className="chart-dates">
                    {chart.days.map((day) => (
                      <span key={day.toISOString()}>
                        {new Intl.DateTimeFormat(i18n.resolvedLanguage, {
                          month: "short",
                          day: "numeric",
                          timeZone,
                        }).format(day)}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
              <div className="panel-small-footer">
                <i />
                {t("admin:overview.scopeNote")}
              </div>
            </section>
            <section className="admin-panel distribution-panel">
              <div className="panel-heading">
                <div>
                  <h2>{t("admin:overview.distribution")}</h2>
                  <p>
                    {t("admin:departments.total", {
                      count: data.departments.length,
                    })}
                  </p>
                </div>
                <MoreDots />
              </div>
              <div className="distribution-total">
                <strong>{number(data.accounts.length)}</strong>
                <span>{t("admin:nav.accounts")}</span>
                <div className="avatar-stack">
                  {data.accounts.slice(0, 5).map((account) => (
                    <span
                      className={`person-avatar tone-${account.id % 5}`}
                      key={account.id}
                    >
                      {initials(account.fullName)}
                    </span>
                  ))}
                </div>
              </div>
              <div className="department-bars">
                {data.departments.slice(0, 4).map((dept) => {
                  const count = data.accounts.filter((account) =>
                    account.memberships.some((m) => m.departmentId === dept.id),
                  ).length;
                  return (
                    <div key={dept.id}>
                      <div>
                        <span>{dept.name}</span>
                        <strong>{number(count)}</strong>
                      </div>
                      <span className="bar-track">
                        <i
                          style={{
                            width: `${Math.max(2, (count / Math.max(1, data.accounts.length)) * 100)}%`,
                          }}
                        />
                      </span>
                    </div>
                  );
                })}
              </div>
              <Link className="panel-text-link" to="/departments">
                {t("admin:departments.title")}
                <ArrowUpRight size={14} />
              </Link>
            </section>
          </div>
          <div className="dashboard-lower">
            <section className="admin-panel recent-accounts-panel">
              <div className="panel-heading">
                <div>
                  <h2>{t("admin:overview.recentAccounts")}</h2>
                  <p>{t("admin:overview.scopeNote")}</p>
                </div>
                <Link to="/accounts" className="subtle-link">
                  {t("admin:overview.viewAll")}
                  <ArrowUpRight size={14} />
                </Link>
              </div>
              <DataTable
                rows={[...data.accounts].reverse().slice(0, 5)}
                columns={columns}
              />
            </section>
            <section className="admin-panel quick-actions">
              <div className="panel-heading">
                <div>
                  <h2>{t("admin:overview.shortcuts")}</h2>
                  <p>{t("admin:overview.shortcutsDescription")}</p>
                </div>
                <KeyRound size={18} />
              </div>
              <Link to="/organizations">
                <span className="entity-symbol">
                  <Building2 size={18} />
                </span>
                <div>
                  <strong>{t("admin:overview.structure")}</strong>
                  <p>{t("admin:overview.structureHint")}</p>
                </div>
                <ArrowUpRight size={15} />
              </Link>
              <Link to="/permissions">
                <span className="entity-symbol">
                  <ShieldCheck size={18} />
                </span>
                <div>
                  <strong>{t("admin:overview.access")}</strong>
                  <p>{t("admin:overview.accessHint")}</p>
                </div>
                <ArrowUpRight size={15} />
              </Link>
              <div className="access-callout">
                <span>
                  <ShieldCheck size={14} />
                  {t("admin:overview.security")}
                </span>
                <h3>{t("admin:overview.securityTitle")}</h3>
                <p>{t("admin:overview.securityHint")}</p>
                <Button variant="outline" render={<Link to="/permissions" />}>
                  {t("admin:actions.manageAccess")}
                  <ArrowRight size={14} />
                </Button>
              </div>
            </section>
          </div>
        </>
      )}
    </>
  );
}
function MoreDots() {
  return (
    <span className="muted-text" aria-hidden="true">
      <svg width="18" height="18">
        <circle cx="3" cy="9" r="1.3" fill="currentColor" />
        <circle cx="9" cy="9" r="1.3" fill="currentColor" />
        <circle cx="15" cy="9" r="1.3" fill="currentColor" />
      </svg>
    </span>
  );
}
