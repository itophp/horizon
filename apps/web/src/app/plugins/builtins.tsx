import { projectFrontendPlugin } from "@/features/projects/plugin";
import { registerFrontendPlugins } from "./registry";

export const frontendExtensions = registerFrontendPlugins([
  projectFrontendPlugin,
]);
