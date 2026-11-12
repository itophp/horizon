import { ApplicationDesignPage } from "@/features/application-runtime/design-page";
import { DynamicApplicationRoute } from "@/features/application-runtime/route";
import { frontendExtensions } from "@/app/plugins/builtins";
import { InstallationsPage } from "@/features/application-runtime/installations";

import { DrivePage } from "@/features/drive/page";
import { DriveDetailPage } from "@/features/drive/detail";
import { DriveMembersPage } from "@/features/drive/members";
import { DriveTrashPage } from "@/features/drive/trash";
import { AiPage } from "@/features/ai/page";
import { KnowledgePage } from "@/features/knowledge/page";
import "@/features/capabilities/capability.css";
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
import { createBrowserRouter, Navigate } from "react-router";
import { lazy, Suspense } from "react";
import { useTranslation } from "react-i18next";
import { Loading } from "@/features/administration/shared";
import { Shell } from "../layouts/shell";
import { Overview } from "@/features/example/overview";
// The graph canvas pulls in React Flow and dagre, so the orchestration page is
// split out instead of inflating the first screen bundle.
const CapabilityPlansPage = lazy(() =>
  import("@/features/execution-plans/page").then((module) => ({
    default: module.CapabilityPlansPage,
  })),
);
const OrchestrationPage = lazy(() =>
  import("@/features/orchestration/page").then((module) => ({
    default: module.OrchestrationPage,
  })),
);
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
              { path: "/applications", element: <InstallationsPage /> },
              {
                path: "/applications/design",
                element: <ApplicationDesignPage />,
              },
              { path: "*", element: <DynamicApplicationRoute /> },
              { path: "/analytics", element: <DynamicApplicationRoute /> },
              { path: "/capabilities", element: <DynamicApplicationRoute /> },
              {
                path: "/capabilities/:name",
                element: (
                  <Suspense fallback={<Loading />}>
                    <CapabilityPlansPage />
                  </Suspense>
                ),
              },
              {
                path: "/orchestration",
                element: (
                  <Suspense fallback={<Loading />}>
                    <OrchestrationPage />
                  </Suspense>
                ),
              },
              { path: "/ai", element: <AiPage /> },
              ...frontendExtensions.routes,
              { path: "/drive", element: <DrivePage /> },
              { path: "/drive/:id", element: <DriveDetailPage /> },
              { path: "/drive/:id/members", element: <DriveMembersPage /> },
              { path: "/drive/:id/trash", element: <DriveTrashPage /> },
              { path: "/knowledge", element: <KnowledgePage /> },
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
              { path: "/settings", element: <DynamicApplicationRoute /> },
              { path: "/mcp", element: <DynamicApplicationRoute /> },
              {
                path: "/mcp-settings",
                element: <Navigate to="/mcp?tab=remote" replace />,
              },
              {
                path: "/webmcp",
                element: <Navigate to="/mcp?tab=page" replace />,
              },
              { path: "/preferences", element: <DynamicApplicationRoute /> },
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
