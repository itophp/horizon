import {
  useEffect,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import type {
  RuntimeApplicationV2,
  RuntimeExtensionV2,
} from "@/api/generated/runtime";
import { Loading } from "@/features/administration/shared";
import { i18n } from "@/i18n";
import { Diagnostic } from "./v2-registry";
export type ContributionResource = {
  source: string;
  entry: string;
  version: string;
  type: string;
  activate: (application: RuntimeApplicationV2) => () => void;
};
type Lease = { count: number; release: () => void };
// Code candidates are deployment-owned. Only leases from authorized definitions are live.
export class ContributionResources {
  private candidates = new Map<string, ContributionResource>();
  private leases = new Map<string, Lease>();
  private listeners = new Set<() => void>();
  private revisions = new Map<string, number>();
  private key(source: string, entry: string, version: string) {
    return `${source}:${entry}@${version}`;
  }
  register(resource: ContributionResource) {
    const key = this.key(resource.source, resource.entry, resource.version);
    if (this.candidates.has(key) || !resource.source || !resource.entry)
      throw new Error("plugin_incompatible");
    this.candidates.set(key, resource);
    return () => {
      if (this.leases.get(key)) throw new Error("plugin_incompatible");
      this.candidates.delete(key);
    };
  }
  has(source: string, entry: string, version: string) {
    return this.candidates.has(this.key(source, entry, version));
  }
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  snapshot = (application: RuntimeApplicationV2) =>
    this.revisions.get(application.metadataVersion) ?? 0;
  acquire(application: RuntimeApplicationV2) {
    const releases: (() => void)[] = [];
    try {
      for (const declaration of application.extensions) {
        const key = this.key(
          declaration.source,
          declaration.entry,
          declaration.version,
        );
        const candidate = this.candidates.get(key);
        if (!candidate) {
          if (["theme", "locale", "shell"].includes(declaration.type))
            throw new Error("component_not_registered");
          continue;
        }
        if (candidate.type !== declaration.type)
          throw new Error("plugin_incompatible");
        let lease = this.leases.get(key);
        if (!lease) {
          lease = { count: 0, release: candidate.activate(application) };
          this.leases.set(key, lease);
        }
        lease.count++;
        releases.push(() => {
          const current = this.leases.get(key);
          if (!current) return;
          if (--current.count === 0) {
            try {
              current.release();
            } finally {
              this.leases.delete(key);
            }
          }
        });
      }
      if (application.messages?.length) {
        const namespace = application.messages[0].key.split(":")[0];
        const key = `${application.source}:messages:${namespace}`;
        let lease = this.leases.get(key);
        if (!lease) {
          const resources = {
            en: Object.fromEntries(
              application.messages.map((message) => [
                message.key.split(":").slice(1).join(":"),
                message.en,
              ]),
            ),
            "zh-CN": Object.fromEntries(
              application.messages.map((message) => [
                message.key.split(":").slice(1).join(":"),
                message.zhCN,
              ]),
            ),
          };
          lease = {
            count: 0,
            release: localeContribution(
              {
                source: application.source,
                entry: key,
                version:
                  application.applicationVersion ?? application.metadataVersion,
              },
              namespace,
              resources,
            ).activate(application),
          };
          this.leases.set(key, lease);
        }
        lease.count++;
        releases.push(() => {
          const current = this.leases.get(key);
          if (current && --current.count === 0) {
            try {
              current.release();
            } finally {
              this.leases.delete(key);
            }
          }
        });
      }
      this.revisions.set(
        application.metadataVersion,
        (this.revisions.get(application.metadataVersion) ?? 0) + 1,
      );
      this.emit();
    } catch (error) {
      releases.reverse().forEach((release) => release());
      throw error;
    }
    return () => {
      releases.reverse().forEach((release) => release());
      const count = (this.revisions.get(application.metadataVersion) ?? 1) - 1;
      if (count) this.revisions.set(application.metadataVersion, count);
      else this.revisions.delete(application.metadataVersion);
      this.emit();
    };
  }
  private emit() {
    for (const listener of this.listeners) listener();
  }
}
export const contributionResources = new ContributionResources();
export function ContributionBoundary({
  application,
  children,
  enabled = true,
}: {
  enabled?: boolean;
  application: RuntimeApplicationV2;
  children: (application: RuntimeApplicationV2, readOnly: boolean) => ReactNode;
}) {
  const state = useMemo(
    () => new ResourceState(application.source),
    [application.source],
  );
  useSyncExternalStore(state.subscribe, state.snapshot);
  useEffect(
    () => (enabled ? state.mount(application) : undefined),
    [state, application, enabled],
  );
  const effective = state.application;
  if (!effective)
    return state.error !== undefined ? (
      <Diagnostic error={state.error} />
    ) : (
      <Loading />
    );
  return (
    <>
      <div>
        {state.error !== undefined && <Diagnostic error={state.error} />}
      </div>
      <div
        style={Object.fromEntries(
          (effective.theme ?? []).map((token) => [token.name, token.value]),
        )}
      >
        {children(
          effective,
          !enabled || effective !== application || state.error !== undefined,
        )}
      </div>
    </>
  );
}
class ResourceState {
  application: RuntimeApplicationV2 | undefined;
  error: unknown;
  private revision = 0;
  private listeners = new Set<() => void>();
  constructor(private source: string) {}
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  snapshot = () => this.revision;
  mount(application: RuntimeApplicationV2) {
    try {
      if (application.source !== this.source)
        throw new Error("plugin_incompatible");
      const release = contributionResources.acquire(application);
      this.application = application;
      this.error = undefined;
      this.emit();
      return release;
    } catch (error) {
      this.error = error;
      this.emit();
      return () => {};
    }
  }
  private emit() {
    this.revision++;
    for (const listener of this.listeners) listener();
  }
}
export function localeContribution(
  declaration: Pick<RuntimeExtensionV2, "source" | "entry" | "version">,
  namespace: string,
  resources: Record<"en" | "zh-CN", object>,
): ContributionResource {
  const keys = (value: object, prefix = ""): string[] =>
    Object.entries(value)
      .flatMap(([key, child]) =>
        child && typeof child === "object"
          ? keys(child, prefix + key + ".")
          : [prefix + key],
      )
      .sort();
  if (
    JSON.stringify(keys(resources.en)) !==
    JSON.stringify(keys(resources["zh-CN"]))
  )
    throw new Error("plugin_incompatible");
  return {
    ...declaration,
    type: "locale",
    activate: () => {
      if (
        i18n.hasResourceBundle("en", namespace) ||
        i18n.hasResourceBundle("zh-CN", namespace)
      )
        throw new Error("plugin_incompatible");
      for (const language of ["en", "zh-CN"] as const)
        i18n.addResourceBundle(language, namespace, resources[language]);
      return () => {
        for (const language of ["en", "zh-CN"])
          i18n.removeResourceBundle(language, namespace);
      };
    },
  };
}
export function themeContribution(
  declaration: Pick<RuntimeExtensionV2, "source" | "entry" | "version">,
  tokens: Record<string, string>,
): ContributionResource {
  for (const [key, value] of Object.entries(tokens))
    if (
      !/^--plugin-[a-z0-9-]+$/.test(key) ||
      !/^(?:#[\da-fA-F]{3,8}|-?\d+(?:\.\d+)?(?:px|rem|em|%)?|var\(--[a-z0-9-]+\))$/.test(
        value,
      )
    )
      throw new Error("plugin_incompatible");
  return {
    ...declaration,
    type: "theme",
    activate: () => {
      const style = document.documentElement.style;
      const previous = new Map(
        Object.keys(tokens).map((key) => [key, style.getPropertyValue(key)]),
      );
      for (const [key, value] of Object.entries(tokens))
        style.setProperty(key, value);
      return () => {
        for (const [key, value] of previous)
          if (value) style.setProperty(key, value);
          else style.removeProperty(key);
      };
    },
  };
}
