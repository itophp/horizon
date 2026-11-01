import { ArticlesPage } from "@/features/articles/page";
import { McpHubPage } from "@/features/mcp/hub";
import { AiPage } from "@/features/ai/page";
import { KnowledgePage } from "@/features/knowledge/page";
import { CapabilityPage } from "@/features/capabilities/page";
import { SettingsPage } from "@/features/governance/settings";
import { FilesPage, FileDownloadPage } from "@/features/governance/files";
import {
  NotificationsPage,
  NotificationDetailPage,
} from "@/features/governance/notifications";
import { NotificationComposePage } from "@/features/governance/notification-compose";
import { TemporaryAccessPage } from "@/features/administration/temporary-access";
import {
  AuthBoundary,
  RequireSession,
  AdminLayout,
} from "@/features/administration/layout";
import { LoginPage } from "@/features/administration/login";
import { useSession } from "@/features/administration/auth";
import { DashboardPage } from "@/features/administration/dashboard";
import {
  AccountsPage,
  OrganizationsPage,
  PermissionsPage,
} from "@/features/administration/pages";
import { createBrowserRouter, Navigate } from "react-router";
import { useTranslation } from "react-i18next";
import { Shell } from "../layouts/shell";
import { Overview } from "@/features/example/overview";
function Architecture() {
  const { t } = useTranslation();
  return (
    <section className="page-intro">
      <h1>{t("about.title")}</h1>
      <p>{t("about.description")}</p>
      <div className="panel prose">
        <p>{t("about.contract")}</p>
        <p>{t("about.auth")}</p>
        <p>{t("about.generated")}</p>
      </div>
    </section>
  );
}
function NotFound() {
  const { t } = useTranslation();
  return <h1>{t("notFound")}</h1>;
}
// The AI workbench is the default surface after sign in. An account without the
// agent grant lands on statistics instead of a workbench it may not open.
export function defaultWorkspacePath(permissions: readonly string[]) {
  return permissions.includes("assistant:use") ? "/ai" : "/analytics";
}
function DefaultWorkspace() {
  const session = useSession();
  return <Navigate to={defaultWorkspacePath(session.permissions)} replace />;
}
export const router = createBrowserRouter([
  {
    element: <AuthBoundary />,
    children: [
      { path: "/login", element: <LoginPage /> },
      {
        element: <RequireSession />,
        children: [
          {
            element: <AdminLayout />,
            children: [
              { path: "/", element: <DefaultWorkspace /> },
              { path: "/analytics", element: <DashboardPage /> },
              { path: "/capabilities", element: <CapabilityPage /> },
              { path: "/ai", element: <AiPage /> },
              { path: "/articles", element: <ArticlesPage /> },
              { path: "/knowledge", element: <KnowledgePage /> },
              { path: "/accounts", element: <AccountsPage /> },
              { path: "/organizations", element: <OrganizationsPage /> },
              {
                path: "/departments",
                element: (
                  <Navigate to="/organizations?tab=departments" replace />
                ),
              },
              {
                path: "/roles",
                element: <Navigate to="/permissions?type=roles" replace />,
              },
              { path: "/settings", element: <SettingsPage /> },
              { path: "/mcp", element: <McpHubPage /> },
              {
                path: "/mcp-settings",
                element: <Navigate to="/mcp?tab=remote" replace />,
              },
              {
                path: "/webmcp",
                element: <Navigate to="/mcp?tab=page" replace />,
              },
              { path: "/preferences", element: <SettingsPage personal /> },
              {
                path: "/dictionaries",
                element: <Navigate to="/settings?tab=fields" replace />,
              },
              { path: "/attachments", element: <FilesPage /> },
              { path: "/files/:id", element: <FileDownloadPage /> },
              { path: "/notifications", element: <NotificationsPage /> },
              {
                path: "/notifications/compose",
                element: <NotificationComposePage />,
              },
              {
                path: "/notifications/:id",
                element: <NotificationDetailPage />,
              },
              { path: "/permissions", element: <PermissionsPage /> },
              { path: "/temporary-access", element: <TemporaryAccessPage /> },
            ],
          },
        ],
      },
    ],
  },
  {
    element: <Shell />,
    children: [
      { path: "/foundation", element: <Overview /> },
      { path: "/architecture", element: <Architecture /> },
      { path: "*", element: <NotFound /> },
    ],
  },
]);
