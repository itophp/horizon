import {
  contributionResources,
  type ContributionResource,
} from "./contributions";
import { extensions, type Extension } from "./v2-registry";
import { componentRegistry, type RuntimeComponent } from "./components";
import type { RuntimeApplicationV2 } from "@/api/generated/runtime";
export function registerExtensionCandidate(
  extension: Extension,
  entry = extension.id,
) {
  return contributionResources.register({
    source: extension.source,
    entry,
    version: extension.version,
    type: extension.type,
    activate: () => extensions.attach(extension),
  });
}
export function registerComponentCandidate(component: RuntimeComponent) {
  return contributionResources.register({
    source: component.source,
    entry: component.name,
    version: component.version,
    type: "component",
    activate: () => componentRegistry.attach(component),
  });
}
// Factories can own abort controllers, subscriptions and temporary resources; disposal is mandatory.
export function registerResourceCandidate(
  resource: ContributionResource & {
    activate: (application: RuntimeApplicationV2) => () => void;
  },
) {
  return contributionResources.register(resource);
}
