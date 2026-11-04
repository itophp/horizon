import { FolderKanban } from "lucide-react";
import { projectTranslations } from "./translations";
import type { FrontendPlugin } from "@/app/plugins/registry";

export const projectFrontendPlugin: FrontendPlugin = {
  id: "com.horizon.projects",
  routes: [
    {
      path: "/projects",
      lazy: async () => ({ Component: (await import("./page")).ProjectsPage }),
    },
    {
      path: "/projects/:id",
      lazy: async () => ({
        Component: (await import("./detail")).ProjectDetailPage,
      }),
    },
  ],
  menus: [
    {
      to: "/projects",
      labelKey: "projects:heading",
      icon: FolderKanban,
      permissions: ["projects:read"],
    },
  ],
  translations: projectTranslations,
  permissionFunctions: { projects: "projects:heading" },
  permissionActions: {
    "projects:write": "projects:permission.write",
    "projects:admin": "projects:permission.admin",
  },
};
