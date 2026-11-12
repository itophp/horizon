import { projectTranslations } from "./translations";
import type { FrontendPlugin } from "@/app/plugins/registry";

export const projectFrontendPlugin: FrontendPlugin = {
  id: "com.horizon.projects",
  routes: [],
  menus: [],
  translations: projectTranslations,
  permissionFunctions: { projects: "projects:heading" },
  permissionActions: {
    "projects:write": "projects:permission.write",
    "projects:admin": "projects:permission.admin",
  },
};
