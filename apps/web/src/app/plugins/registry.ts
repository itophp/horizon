import type { ComponentType } from "react";
import type { RouteObject } from "react-router";

export interface FrontendPlugin {
  id: string;
  routes: RouteObject[];
  menus: {
    to: string;
    labelKey: string;
    icon: ComponentType;
    permissions: string[];
  }[];
  translations: Record<"en" | "zh-CN", Record<string, object>>;
  permissionFunctions?: Record<string, string>;
  permissionActions?: Record<string, string>;
}

function translationLeaves(value: object, prefix = ""): string[] {
  return Object.entries(value)
    .flatMap(([key, item]) => {
      const path = prefix ? `${prefix}.${key}` : key;
      return item !== null && typeof item === "object" && !Array.isArray(item)
        ? translationLeaves(item, path)
        : [path];
    })
    .sort();
}

export function registerFrontendPlugins(plugins: FrontendPlugin[]) {
  const ids = new Set<string>();
  const paths = new Set<string>();
  const menuPaths = new Set<string>();
  const namespaces = new Set<string>();
  const permissionFunctions: Record<string, string> = {};
  const permissionActions: Record<string, string> = {};
  for (const plugin of plugins) {
    if (ids.has(plugin.id))
      throw new Error(`Duplicate frontend plugin: ${plugin.id}`);
    ids.add(plugin.id);
    const english = Object.keys(plugin.translations.en).sort();
    const chinese = Object.keys(plugin.translations["zh-CN"]).sort();
    if (JSON.stringify(english) !== JSON.stringify(chinese))
      throw new Error(`Plugin locale namespace mismatch: ${plugin.id}`);
    for (const namespace of english) {
      if (
        JSON.stringify(translationLeaves(plugin.translations.en[namespace])) !==
        JSON.stringify(
          translationLeaves(plugin.translations["zh-CN"][namespace]),
        )
      ) {
        throw new Error(
          `Plugin locale key mismatch: ${plugin.id}/${namespace}`,
        );
      }
      if (namespaces.has(namespace))
        throw new Error(`Duplicate plugin namespace: ${namespace}`);
      namespaces.add(namespace);
    }
    for (const key of [
      ...Object.values(plugin.permissionFunctions ?? {}),
      ...Object.values(plugin.permissionActions ?? {}),
    ]) {
      const separator = key.indexOf(":");
      const namespace = key.slice(0, separator);
      const path = key.slice(separator + 1);
      const resource = plugin.translations.en[namespace];
      if (
        separator <= 0 ||
        !resource ||
        !translationLeaves(resource).includes(path)
      ) {
        throw new Error(
          `Plugin permission label missing translation: ${plugin.id}/${key}`,
        );
      }
    }
    for (const [name, key] of Object.entries(
      plugin.permissionFunctions ?? {},
    )) {
      if (Object.hasOwn(permissionFunctions, name))
        throw new Error(`Duplicate permission function: ${name}`);
      permissionFunctions[name] = key;
    }
    for (const [name, key] of Object.entries(plugin.permissionActions ?? {})) {
      if (Object.hasOwn(permissionActions, name))
        throw new Error(`Duplicate permission action: ${name}`);
      permissionActions[name] = key;
    }
    for (const menu of plugin.menus) {
      if (menuPaths.has(menu.to))
        throw new Error(`Duplicate plugin menu: ${menu.to}`);
      menuPaths.add(menu.to);
    }
    for (const route of plugin.routes) {
      if (route.path && paths.has(route.path))
        throw new Error(`Duplicate plugin route: ${route.path}`);
      if (route.path) paths.add(route.path);
    }
  }
  return {
    routes: plugins.flatMap((plugin) => plugin.routes),
    menus: plugins.flatMap((plugin) => plugin.menus),
    translations: plugins.map((plugin) => plugin.translations),
    permissionFunctions,
    permissionActions,
  };
}

// A plugin must never silently replace a platform translation namespace.
export function assertTranslationNamespaces(
  core: Record<"en" | "zh-CN", Record<string, object>>,
  translations: FrontendPlugin["translations"][],
) {
  for (const language of ["en", "zh-CN"] as const) {
    for (const contribution of translations) {
      for (const namespace of Object.keys(contribution[language])) {
        if (Object.hasOwn(core[language], namespace)) {
          throw new Error(
            `Plugin namespace conflicts with platform: ${language}/${namespace}`,
          );
        }
      }
    }
  }
}

// Menu visibility is a presentation filter; Gateway remains authoritative.
export function canShowMenu(
  required: readonly string[],
  granted: readonly string[],
) {
  return (
    required.length === 0 ||
    required.some((permission) => granted.includes(permission))
  );
}
