import { WebMcpSettingsPage } from "@/features/webmcp/settings";
import { SettingsPage } from "@/features/governance/settings";
import { DictionariesPage } from "@/features/governance/dictionaries";
import { FilesPage, FileDownloadPage } from "@/features/governance/files";
import { NotificationsPage } from "@/features/governance/notifications";
import { TemporaryAccessPage } from "@/features/administration/temporary-access";
import {
  AuthBoundary,
  RequireSession,
  AdminLayout,
} from "@/features/administration/layout";
import { LoginPage } from "@/features/administration/login";
import { DashboardPage } from "@/features/administration/dashboard";
import {
  AccountsPage,
  OrganizationsPage,
  DepartmentsPage,
  RolesPage,
  PermissionsPage,
} from "@/features/administration/pages";
import { createBrowserRouter } from "react-router";
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
              { path: "/", element: <DashboardPage /> },
              { path: "/accounts", element: <AccountsPage /> },
              { path: "/organizations", element: <OrganizationsPage /> },
              { path: "/departments", element: <DepartmentsPage /> },
              { path: "/roles", element: <RolesPage /> },
              { path: "/settings", element: <SettingsPage /> },
              { path: "/webmcp", element: <WebMcpSettingsPage /> },
              { path: "/preferences", element: <SettingsPage personal /> },
              { path: "/dictionaries", element: <DictionariesPage /> },
              { path: "/attachments", element: <FilesPage /> },
              { path: "/files/:id", element: <FileDownloadPage /> },
              { path: "/notifications", element: <NotificationsPage /> },
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
